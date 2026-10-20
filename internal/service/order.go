package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"marketplace/internal/domain"
	"github.com/google/uuid"
)

type InsufficientStockError struct {
	Items []domain.DeficientStockItem `json:"items"`
}

func (e *InsufficientStockError) Error() string {
	return "insufficient stock for one or more items"
}

type OrderService struct {
	orderRepo     domain.OrderRepository
	productRepo   domain.ProductRepository
	promoRepo     domain.PromoCodeRepository
	opRepo        domain.UserOperationRepository
	rateLimitTime time.Duration
}

func NewOrderService(
	orderRepo domain.OrderRepository,
	productRepo domain.ProductRepository,
	promoRepo domain.PromoCodeRepository,
	opRepo domain.UserOperationRepository,
	rateLimitTime time.Duration,
) *OrderService {
	return &OrderService{
		orderRepo:     orderRepo,
		productRepo:   productRepo,
		promoRepo:     promoRepo,
		opRepo:        opRepo,
		rateLimitTime: rateLimitTime,
	}
}

func (s *OrderService) GetByID(ctx context.Context, orderID string) (*domain.Order, error) {
	return s.orderRepo.GetByID(ctx, orderID)
}

func (s *OrderService) CreateOrder(ctx context.Context, userID string, req *domain.OrderCreateRequest) (*domain.Order, error) {
	now := time.Now().UTC()

	// 1. Ограничение частоты создания (CREATE_ORDER)
	lastOp, err := s.opRepo.GetLastOperation(ctx, userID, domain.OperationTypeCreateOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to check user operations: %w", err)
	}
	if lastOp != nil && now.Sub(lastOp.CreatedAt) < s.rateLimitTime {
		return nil, domain.ErrOrderLimitExceeded
	}

	// 2. Проверка активных заказов (CREATED или PAYMENT_PENDING)
	hasActive, err := s.orderRepo.HasActiveOrder(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active orders: %w", err)
	}
	if hasActive {
		return nil, domain.ErrOrderHasActive
	}

	// 3 & 4. Проверка каталога и остатков
	orderID := uuid.New().String()
	var orderItems []*domain.OrderItem
	var deficientItems []domain.DeficientStockItem
	var rawTotal float64

	// Агрегируем количества, если в запросе передан один и тот же товар несколько раз
	itemCounts := make(map[string]int)
	for _, it := range req.Items {
		itemCounts[it.ProductID] += it.Quantity
	}

	for prodID, qty := range itemCounts {
		prod, err := s.productRepo.GetByID(ctx, prodID)
		if err != nil {
			return nil, domain.ErrProductNotFound
		}
		if prod.Status != domain.ProductStatusActive {
			return nil, domain.ErrProductInactive
		}
		if prod.Stock < qty {
			deficientItems = append(deficientItems, domain.DeficientStockItem{
				ProductID:      prodID,
				RequestedCount: qty,
				AvailableStock: prod.Stock,
			})
		} else {
			// Снапшот цены
			orderItems = append(orderItems, &domain.OrderItem{
				ID:           uuid.New().String(),
				OrderID:      orderID,
				ProductID:    prodID,
				Quantity:     qty,
				PriceAtOrder: prod.Price,
				CreatedAt:    now,
			})
			rawTotal += prod.Price * float64(qty)
		}
	}

	if len(deficientItems) > 0 {
		return nil, &InsufficientStockError{Items: deficientItems}
	}

	// 7. Расчёт стоимости и применение промокода
	totalAmount := rawTotal
	discountAmount := 0.00
	var appliedPromo *domain.PromoCode

	if req.PromoCode != nil && strings.TrimSpace(*req.PromoCode) != "" {
		code := strings.ToUpper(strings.TrimSpace(*req.PromoCode))
		promo, err := s.promoRepo.GetByCode(ctx, code)
		if err != nil {
			return nil, domain.ErrPromoCodeInvalid
		}

		// Проверка активности, лимита использования и срока действия
		if !promo.Active || promo.CurrentUses >= promo.MaxUses || now.Before(promo.ValidFrom) || now.After(promo.ValidUntil) {
			return nil, domain.ErrPromoCodeInvalid
		}

		// Проверка минимальной суммы заказа
		if totalAmount < promo.MinOrderAmount {
			return nil, domain.ErrPromoCodeMinAmount
		}

		// Расчет скидки
		if promo.DiscountType == domain.DiscountTypePercentage {
			// Не более 70% от total_amount, иначе цена 100%
			discount := totalAmount * promo.DiscountValue / 100.0
			if promo.DiscountValue > 70.0 {
				discountAmount = 0.00
			} else {
				discountAmount = math.Round(discount*100) / 100
			}
		} else if promo.DiscountType == domain.DiscountTypeFixedAmount {
			discountAmount = math.Min(promo.DiscountValue, totalAmount)
		}

		totalAmount = math.Max(0, totalAmount-discountAmount)
		appliedPromo = promo
	}

	var promoID *string
	var promoCodeStr *string
	if appliedPromo != nil {
		promoID = &appliedPromo.ID
		promoCodeStr = &appliedPromo.Code
	}

	order := &domain.Order{
		ID:             orderID,
		UserID:         userID,
		Status:         domain.OrderStatusCreated,
		PromoCodeID:    promoID,
		PromoCode:      promoCodeStr,
		TotalAmount:    totalAmount,
		DiscountAmount: discountAmount,
		Items:          orderItems,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// 5. Резервирование остатков и сохранение в одной транзакции
	if err := s.orderRepo.CreateWithTx(ctx, order, orderItems, appliedPromo); err != nil {
		return nil, fmt.Errorf("failed to save order transactionally: %w", err)
	}

	// 8. Фиксация операции в user_operations
	_ = s.opRepo.Record(ctx, userID, domain.OperationTypeCreateOrder)

	return order, nil
}

func (s *OrderService) UpdateOrder(ctx context.Context, userID, orderID string, req *domain.OrderUpdateRequest) (*domain.Order, error) {
	now := time.Now().UTC()

	// 1. Проверка существования заказа и владельца
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, domain.ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, domain.ErrOrderOwnershipViolation
	}

	// 2. Проверка состояния: только CREATED
	if order.Status != domain.OrderStatusCreated {
		return nil, domain.ErrInvalidStateTransition
	}

	// 3. Ограничение частоты обновления
	lastOp, err := s.opRepo.GetLastOperation(ctx, userID, domain.OperationTypeUpdateOrder)
	if err != nil {
		return nil, fmt.Errorf("failed to check update operation: %w", err)
	}
	if lastOp != nil && now.Sub(lastOp.CreatedAt) < s.rateLimitTime {
		return nil, domain.ErrOrderLimitExceeded
	}

	// 4 & 5. Проверка и резервирование новых позиций с учетом возврата старых остатков
	oldItems := order.Items
	oldItemQuantities := make(map[string]int)
	for _, it := range oldItems {
		oldItemQuantities[it.ProductID] += it.Quantity
	}

	newItemCounts := make(map[string]int)
	for _, it := range req.Items {
		newItemCounts[it.ProductID] += it.Quantity
	}

	var newOrderItems []*domain.OrderItem
	var deficientItems []domain.DeficientStockItem
	var rawTotal float64

	for prodID, newQty := range newItemCounts {
		prod, err := s.productRepo.GetByID(ctx, prodID)
		if err != nil {
			return nil, domain.ErrProductNotFound
		}
		if prod.Status != domain.ProductStatusActive {
			return nil, domain.ErrProductInactive
		}

		// Доступный остаток с учетом возврата позиций этого же заказа
		effectiveStock := prod.Stock + oldItemQuantities[prodID]
		if effectiveStock < newQty {
			deficientItems = append(deficientItems, domain.DeficientStockItem{
				ProductID:      prodID,
				RequestedCount: newQty,
				AvailableStock: effectiveStock,
			})
		} else {
			newOrderItems = append(newOrderItems, &domain.OrderItem{
				ID:           uuid.New().String(),
				OrderID:      orderID,
				ProductID:    prodID,
				Quantity:     newQty,
				PriceAtOrder: prod.Price,
				CreatedAt:    now,
			})
			rawTotal += prod.Price * float64(newQty)
		}
	}

	if len(deficientItems) > 0 {
		return nil, &InsufficientStockError{Items: deficientItems}
	}

	// 6. Пересчет стоимости и проверка промокода
	totalAmount := rawTotal
	discountAmount := 0.00
	var appliedPromo *domain.PromoCode
	var oldPromo *domain.PromoCode

	if order.PromoCodeID != nil {
		promo, err := s.promoRepo.GetByID(ctx, *order.PromoCodeID)
		if err == nil && promo != nil {
			oldPromo = promo
			// Проверяем условия заново
			if promo.Active && now.After(promo.ValidFrom) && now.Before(promo.ValidUntil) && totalAmount >= promo.MinOrderAmount {
				if promo.DiscountType == domain.DiscountTypePercentage {
					if promo.DiscountValue <= 70.0 {
						discountAmount = math.Round((totalAmount*promo.DiscountValue/100.0)*100) / 100
					}
				} else if promo.DiscountType == domain.DiscountTypeFixedAmount {
					discountAmount = math.Min(promo.DiscountValue, totalAmount)
				}
				totalAmount = math.Max(0, totalAmount-discountAmount)
				appliedPromo = promo
			}
		}
	}

	order.TotalAmount = totalAmount
	order.DiscountAmount = discountAmount
	if appliedPromo != nil {
		order.PromoCodeID = &appliedPromo.ID
		order.PromoCode = &appliedPromo.Code
	} else {
		order.PromoCodeID = nil
		order.PromoCode = nil
	}
	order.Items = newOrderItems
	order.UpdatedAt = now

	// Выполняем возврат старых остатков, запись новых и обновление заказа в транзакции
	if err := s.orderRepo.UpdateWithTx(ctx, order, newOrderItems, oldItems, appliedPromo, oldPromo); err != nil {
		return nil, fmt.Errorf("failed to update order transactionally: %w", err)
	}

	// 7. Фиксация операции
	_ = s.opRepo.Record(ctx, userID, domain.OperationTypeUpdateOrder)

	return order, nil
}

func (s *OrderService) CancelOrder(ctx context.Context, userID, orderID string) (*domain.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, domain.ErrOrderNotFound
	}

	// 1. Проверка владельца
	if order.UserID != userID {
		return nil, domain.ErrOrderOwnershipViolation
	}

	// 2. Проверка состояния: только CREATED или PAYMENT_PENDING
	if order.Status != domain.OrderStatusCreated && order.Status != domain.OrderStatusPaymentPending {
		return nil, domain.ErrInvalidStateTransition
	}

	var promo *domain.PromoCode
	if order.PromoCodeID != nil {
		p, err := s.promoRepo.GetByID(ctx, *order.PromoCodeID)
		if err == nil {
			promo = p
		}
	}

	// 3, 4, 5. Возврат остатков, отмена промокода и перевод в CANCELED в транзакции
	if err := s.orderRepo.CancelWithTx(ctx, order, order.Items, promo); err != nil {
		return nil, fmt.Errorf("failed to cancel order transactionally: %w", err)
	}

	order.Status = domain.OrderStatusCanceled
	order.UpdatedAt = time.Now().UTC()
	return order, nil
}
