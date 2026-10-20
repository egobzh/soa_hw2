package http

import (
	"errors"
	"net/http"
	"strconv"

	"marketplace/internal/domain"
	"marketplace/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	productService  domain.ProductService
	orderService    domain.OrderService
	promoCodeService domain.PromoCodeService
}

func NewHandler(
	productService domain.ProductService,
	orderService domain.OrderService,
	promoCodeService domain.PromoCodeService,
) *Handler {
	return &Handler{
		productService:  productService,
		orderService:    orderService,
		promoCodeService: promoCodeService,
	}
}

// ----------------- Products -----------------

// CreateProduct godoc
// @Summary Создание товара
// @Description Добавляет новый товар в каталог. По умолчанию создается со статусом ACTIVE.
// @Tags Products
// @Accept json
// @Produce json
// @Param X-User-Id header string false "ID продавца (UUID)"
// @Param request body domain.ProductCreateRequest true "Данные товара"
// @Success 201 {object} domain.Product "Товар успешно создан"
// @Failure 400 {object} domain.ErrorResponse "Ошибка валидации входных данных"
// @Router /products [post]
func (h *Handler) CreateProduct(c *gin.Context) {
	var req domain.ProductCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Validation error",
			Details:   err.Error(),
		})
		return
	}

	if sellerID := c.GetHeader("X-User-Id"); sellerID != "" {
		if _, err := uuid.Parse(sellerID); err == nil {
			req.SellerID = &sellerID
		}
	}

	p, err := h.productService.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, p)
}

// GetProductByID godoc
// @Summary Получение товара по ID
// @Description Возвращает подробную информацию о товаре по его идентификатору.
// @Tags Products
// @Accept json
// @Produce json
// @Param id path string true "ID товара (UUID)"
// @Success 200 {object} domain.Product "Данные товара"
// @Failure 404 {object} domain.ErrorResponse "Товар не найден"
// @Router /products/{id} [get]
func (h *Handler) GetProductByID(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Invalid UUID format for id",
		})
		return
	}

	p, err := h.productService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, domain.ErrorResponse{
				ErrorCode: domain.ErrCodeProductNotFound,
				Message:   "Product not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, p)
}

// GetProducts godoc
// @Summary Список товаров с пагинацией и фильтрацией
// @Description Возвращает список товаров с фильтрацией по статусу и категории, и метаданными пагинации.
// @Tags Products
// @Accept json
// @Produce json
// @Param page query int false "Номер страницы (начиная с 0, default 0)"
// @Param size query int false "Размер страницы (default 20)"
// @Param status query string false "Фильтр по статусу (ACTIVE, INACTIVE, ARCHIVED)" Enums(ACTIVE, INACTIVE, ARCHIVED)
// @Param category query string false "Фильтр по категории (точное совпадение)"
// @Success 200 {object} domain.ProductListResponse "Список товаров"
// @Failure 400 {object} domain.ErrorResponse "Ошибка валидации параметров"
// @Router /products [get]
func (h *Handler) GetProducts(c *gin.Context) {
	page := 0
	size := 20

	if pageStr := c.Query("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 0 {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{
				ErrorCode: domain.ErrCodeValidationError,
				Message:   "Invalid page parameter, must be non-negative integer",
			})
			return
		}
		page = p
	}

	if sizeStr := c.Query("size"); sizeStr != "" {
		s, err := strconv.Atoi(sizeStr)
		if err != nil || s <= 0 {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{
				ErrorCode: domain.ErrCodeValidationError,
				Message:   "Invalid size parameter, must be positive integer",
			})
			return
		}
		size = s
	}

	var status *domain.ProductStatus
	if st := c.Query("status"); st != "" {
		prodStatus := domain.ProductStatus(st)
		if prodStatus != domain.ProductStatusActive && prodStatus != domain.ProductStatusInactive && prodStatus != domain.ProductStatusArchived {
			c.JSON(http.StatusBadRequest, domain.ErrorResponse{
				ErrorCode: domain.ErrCodeValidationError,
				Message:   "Invalid status parameter, must be ACTIVE, INACTIVE, or ARCHIVED",
			})
			return
		}
		status = &prodStatus
	}

	var category *string
	if cat := c.Query("category"); cat != "" {
		category = &cat
	}

	res, err := h.productService.List(c.Request.Context(), page, size, status, category)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, res)
}

// UpdateProduct godoc
// @Summary Обновление товара
// @Description Обновляет данные существующего товара по его ID.
// @Tags Products
// @Accept json
// @Produce json
// @Param id path string true "ID товара (UUID)"
// @Param request body domain.ProductUpdateRequest true "Данные для обновления"
// @Success 200 {object} domain.Product "Обновленный товар"
// @Failure 400 {object} domain.ErrorResponse "Ошибка валидации"
// @Failure 404 {object} domain.ErrorResponse "Товар не найден"
// @Router /products/{id} [put]
func (h *Handler) UpdateProduct(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Invalid UUID format for id",
		})
		return
	}

	var req domain.ProductUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Validation error",
			Details:   err.Error(),
		})
		return
	}

	p, err := h.productService.Update(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, domain.ErrorResponse{
				ErrorCode: domain.ErrCodeProductNotFound,
				Message:   "Product not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, p)
}

// DeleteProduct godoc
// @Summary Мягкое удаление товара
// @Description Переводит товар в статус ARCHIVED, не удаляя запись физически из БД.
// @Tags Products
// @Accept json
// @Produce json
// @Param id path string true "ID товара (UUID)"
// @Success 200 {object} domain.Product "Архивированный товар"
// @Failure 404 {object} domain.ErrorResponse "Товар не найден"
// @Router /products/{id} [delete]
func (h *Handler) DeleteProduct(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Invalid UUID format for id",
		})
		return
	}

	p, err := h.productService.Delete(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, domain.ErrorResponse{
				ErrorCode: domain.ErrCodeProductNotFound,
				Message:   "Product not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, p)
}

// ----------------- Orders -----------------

// CreateOrder godoc
// @Summary Создание заказа
// @Description Создает заказ с проверкой лимита частоты, активных заказов, наличия остатков, снапшотом цен и промокодом.
// @Tags Orders
// @Accept json
// @Produce json
// @Param X-User-Id header string true "ID пользователя (UUID)"
// @Param request body domain.OrderCreateRequest true "Данные заказа"
// @Success 201 {object} domain.Order "Заказ успешно создан"
// @Failure 400 {object} domain.ErrorResponse "Ошибка валидации входных данных"
// @Failure 404 {object} domain.ErrorResponse "Товар не найден"
// @Failure 409 {object} domain.ErrorResponse "Конфликт логики (активный заказ, неактивный товар или нехватка остатков)"
// @Failure 422 {object} domain.ErrorResponse "Невалидный промокод или сумма ниже минимальной"
// @Failure 429 {object} domain.ErrorResponse "Превышен лимит частоты создания заказа"
// @Router /orders [post]
func (h *Handler) CreateOrder(c *gin.Context) {
	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Missing required header: X-User-Id",
		})
		return
	}
	if _, err := uuid.Parse(userID); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Invalid UUID format for X-User-Id",
		})
		return
	}

	var req domain.OrderCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Validation error",
			Details:   err.Error(),
		})
		return
	}

	order, err := h.orderService.CreateOrder(c.Request.Context(), userID, &req)
	if err != nil {
		h.handleOrderError(c, err)
		return
	}

	c.JSON(http.StatusCreated, order)
}

// UpdateOrder godoc
// @Summary Обновление позиций заказа
// @Description Обновляет состав позиций заказа. Разрешено только в статусе CREATED и только владельцу.
// @Tags Orders
// @Accept json
// @Produce json
// @Param id path string true "ID заказа (UUID)"
// @Param X-User-Id header string true "ID пользователя (UUID)"
// @Param request body domain.OrderUpdateRequest true "Новые позиции заказа"
// @Success 200 {object} domain.Order "Обновленный заказ"
// @Failure 400 {object} domain.ErrorResponse "Ошибка валидации"
// @Failure 403 {object} domain.ErrorResponse "Заказ принадлежит другому пользователю"
// @Failure 404 {object} domain.ErrorResponse "Заказ или товар не найден"
// @Failure 409 {object} domain.ErrorResponse "Недопустимый статус или нехватка остатков"
// @Failure 429 {object} domain.ErrorResponse "Превышен лимит частоты обновления"
// @Router /orders/{id} [put]
func (h *Handler) UpdateOrder(c *gin.Context) {
	orderID := c.Param("id")
	if _, err := uuid.Parse(orderID); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Invalid UUID format for id",
		})
		return
	}

	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Missing required header: X-User-Id",
		})
		return
	}

	var req domain.OrderUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Validation error",
			Details:   err.Error(),
		})
		return
	}

	order, err := h.orderService.UpdateOrder(c.Request.Context(), userID, orderID, &req)
	if err != nil {
		h.handleOrderError(c, err)
		return
	}

	c.JSON(http.StatusOK, order)
}

// CancelOrder godoc
// @Summary Отмена заказа
// @Description Отменяет заказ с возвратом остатков на склад и декрементом использований промокода.
// @Tags Orders
// @Accept json
// @Produce json
// @Param id path string true "ID заказа (UUID)"
// @Param X-User-Id header string true "ID пользователя (UUID)"
// @Success 200 {object} domain.Order "Отмененный заказ"
// @Failure 400 {object} domain.ErrorResponse "Ошибка валидации"
// @Failure 403 {object} domain.ErrorResponse "Заказ принадлежит другому пользователю"
// @Failure 404 {object} domain.ErrorResponse "Заказ не найден"
// @Failure 409 {object} domain.ErrorResponse "Недопустимый переход состояния"
// @Router /orders/{id}/cancel [post]
func (h *Handler) CancelOrder(c *gin.Context) {
	orderID := c.Param("id")
	if _, err := uuid.Parse(orderID); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Invalid UUID format for id",
		})
		return
	}

	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Missing required header: X-User-Id",
		})
		return
	}

	order, err := h.orderService.CancelOrder(c.Request.Context(), userID, orderID)
	if err != nil {
		h.handleOrderError(c, err)
		return
	}

	c.JSON(http.StatusOK, order)
}

// ----------------- Promo Codes -----------------

// CreatePromoCode godoc
// @Summary Создание промокода
// @Description Создает новый промокод с условиями использования.
// @Tags PromoCodes
// @Accept json
// @Produce json
// @Param request body domain.PromoCodeCreateRequest true "Параметры промокода"
// @Success 201 {object} domain.PromoCode "Промокод успешно создан"
// @Failure 400 {object} domain.ErrorResponse "Ошибка валидации"
// @Router /promo-codes [post]
func (h *Handler) CreatePromoCode(c *gin.Context) {
	var req domain.PromoCodeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   "Validation error",
			Details:   err.Error(),
		})
		return
	}

	promo, err := h.promoCodeService.Create(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, promo)
}

// ----------------- Helper error dispatcher -----------------

func (h *Handler) handleOrderError(c *gin.Context, err error) {
	var insufficientStock *service.InsufficientStockError
	if errors.As(err, &insufficientStock) {
		c.JSON(http.StatusConflict, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeInsufficientStock,
			Message:   "Insufficient stock for products",
			Details:   insufficientStock.Items,
		})
		return
	}

	switch {
	case errors.Is(err, domain.ErrProductNotFound):
		c.JSON(http.StatusNotFound, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeProductNotFound,
			Message:   "Product not found",
		})
	case errors.Is(err, domain.ErrProductInactive):
		c.JSON(http.StatusConflict, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeProductInactive,
			Message:   "Product is inactive or archived",
		})
	case errors.Is(err, domain.ErrOrderNotFound):
		c.JSON(http.StatusNotFound, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeOrderNotFound,
			Message:   "Order not found",
		})
	case errors.Is(err, domain.ErrOrderLimitExceeded):
		c.JSON(http.StatusTooManyRequests, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeOrderLimitExceeded,
			Message:   "Order limit exceeded. Please wait before creating or updating another order.",
		})
	case errors.Is(err, domain.ErrOrderHasActive):
		c.JSON(http.StatusConflict, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeOrderHasActive,
			Message:   "User already has an active order in progress",
		})
	case errors.Is(err, domain.ErrInvalidStateTransition):
		c.JSON(http.StatusConflict, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeInvalidStateTransition,
			Message:   "Invalid state transition for order",
		})
	case errors.Is(err, domain.ErrPromoCodeInvalid):
		c.JSON(http.StatusUnprocessableEntity, domain.ErrorResponse{
			ErrorCode: domain.ErrCodePromoCodeInvalid,
			Message:   "Promo code is invalid, expired or usage limit reached",
		})
	case errors.Is(err, domain.ErrPromoCodeMinAmount):
		c.JSON(http.StatusUnprocessableEntity, domain.ErrorResponse{
			ErrorCode: domain.ErrCodePromoCodeMinAmount,
			Message:   "Order total amount is less than promo code minimum amount",
		})
	case errors.Is(err, domain.ErrOrderOwnershipViolation):
		c.JSON(http.StatusForbidden, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeOrderOwnershipViolation,
			Message:   "Order belongs to another user",
		})
	default:
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			ErrorCode: domain.ErrCodeValidationError,
			Message:   err.Error(),
		})
	}
}
