package domain

import (
	"context"
	"errors"
	"time"
)

// Обязательные коды ошибок согласно пункту 5 ТЗ
const (
	ErrCodeProductNotFound        = "PRODUCT_NOT_FOUND"
	ErrCodeProductInactive        = "PRODUCT_INACTIVE"
	ErrCodeOrderNotFound          = "ORDER_NOT_FOUND"
	ErrCodeOrderLimitExceeded     = "ORDER_LIMIT_EXCEEDED"
	ErrCodeOrderHasActive         = "ORDER_HAS_ACTIVE"
	ErrCodeInvalidStateTransition = "INVALID_STATE_TRANSITION"
	ErrCodeInsufficientStock      = "INSUFFICIENT_STOCK"
	ErrCodePromoCodeInvalid       = "PROMO_CODE_INVALID"
	ErrCodePromoCodeMinAmount     = "PROMO_CODE_MIN_AMOUNT"
	ErrCodeOrderOwnershipViolation = "ORDER_OWNERSHIP_VIOLATION"
	ErrCodeValidationError        = "VALIDATION_ERROR"
)

var (
	ErrProductNotFound        = errors.New("product not found")
	ErrProductInactive        = errors.New("product is inactive or archived")
	ErrOrderNotFound          = errors.New("order not found")
	ErrOrderLimitExceeded     = errors.New("order rate limit exceeded, please wait")
	ErrOrderHasActive         = errors.New("user already has an active order (CREATED or PAYMENT_PENDING)")
	ErrInvalidStateTransition = errors.New("invalid order state transition")
	ErrInsufficientStock      = errors.New("insufficient stock for one or more items")
	ErrPromoCodeInvalid       = errors.New("promo code is invalid, expired, or usage limit reached")
	ErrPromoCodeMinAmount     = errors.New("order total is below minimum required for promo code")
	ErrOrderOwnershipViolation = errors.New("order belongs to another user")
	ErrValidationFailed       = errors.New("validation failed")
)

// ProductStatus enum
type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "ACTIVE"
	ProductStatusInactive ProductStatus = "INACTIVE"
	ProductStatusArchived ProductStatus = "ARCHIVED"
)

// OrderStatus enum
type OrderStatus string

const (
	OrderStatusCreated        OrderStatus = "CREATED"
	OrderStatusPaymentPending OrderStatus = "PAYMENT_PENDING"
	OrderStatusPaid           OrderStatus = "PAID"
	OrderStatusShipped        OrderStatus = "SHIPPED"
	OrderStatusCompleted      OrderStatus = "COMPLETED"
	OrderStatusCanceled       OrderStatus = "CANCELED"
)

// DiscountType enum
type DiscountType string

const (
	DiscountTypePercentage  DiscountType = "PERCENTAGE"
	DiscountTypeFixedAmount DiscountType = "FIXED_AMOUNT"
)

// OperationType enum
type OperationType string

const (
	OperationTypeCreateOrder OperationType = "CREATE_ORDER"
	OperationTypeUpdateOrder OperationType = "UPDATE_ORDER"
)

// Единый формат ошибок согласно пункту 5 ТЗ
type ErrorResponse struct {
	ErrorCode string `json:"error_code" example:"INSUFFICIENT_STOCK"`
	Message   string `json:"message" example:"Not enough stock for products"`
	Details   any    `json:"details,omitempty"`
}

// ----------------- Product Models & DTOs -----------------

type Product struct {
	ID          string        `json:"id" example:"a4d2e482-1d54-47ef-a0b2-c07a9b8efaa1"`
	Name        string        `json:"name" example:"Смартфон Pro Max"`
	Description *string       `json:"description,omitempty" example:"Флагманский смартфон с OLED-экраном"`
	Price       float64       `json:"price" example:"99990.00"`
	Stock       int           `json:"stock" example:"50"`
	Category    string        `json:"category" example:"Электроника"`
	Status      ProductStatus `json:"status" example:"ACTIVE"`
	SellerID    *string       `json:"seller_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	CreatedAt   time.Time     `json:"created_at" example:"2026-10-18T12:00:00Z"`
	UpdatedAt   time.Time     `json:"updated_at" example:"2026-10-18T12:00:00Z"`
}

type ProductCreateRequest struct {
	Name        string        `json:"name" binding:"required,min=1,max=255" example:"Смартфон Pro Max"`
	Description *string       `json:"description,omitempty" binding:"omitempty,max=4000" example:"Флагманский смартфон"`
	Price       float64       `json:"price" binding:"required,gt=0" example:"99990.00"`
	Stock       int           `json:"stock" binding:"gte=0" example:"50"`
	Category    string        `json:"category" binding:"required,min=1,max=100" example:"Электроника"`
	Status      ProductStatus `json:"status" binding:"required,oneof=ACTIVE INACTIVE ARCHIVED" example:"ACTIVE"`
	SellerID    *string       `json:"seller_id,omitempty" binding:"omitempty,uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type ProductUpdateRequest struct {
	Name        string        `json:"name" binding:"required,min=1,max=255" example:"Смартфон Pro Max (Обновленный)"`
	Description *string       `json:"description,omitempty" binding:"omitempty,max=4000" example:"Новая ревизия"`
	Price       float64       `json:"price" binding:"required,gt=0" example:"94990.00"`
	Stock       int           `json:"stock" binding:"gte=0" example:"45"`
	Category    string        `json:"category" binding:"required,min=1,max=100" example:"Электроника"`
	Status      ProductStatus `json:"status" binding:"required,oneof=ACTIVE INACTIVE ARCHIVED" example:"ACTIVE"`
}

type ProductListResponse struct {
	Items         []*Product `json:"items"`
	TotalElements int        `json:"totalElements" example:"42"`
	Page          int        `json:"page" example:"0"`
	Size          int        `json:"size" example:"20"`
}

// ----------------- Order Models & DTOs -----------------

type OrderItem struct {
	ID           string    `json:"id" example:"6ba7b810-9dad-11d1-80b4-00c04fd430c8"`
	OrderID      string    `json:"order_id" example:"a4d2e482-1d54-47ef-a0b2-c07a9b8efaa1"`
	ProductID    string    `json:"product_id" example:"11111111-1111-1111-1111-111111111111"`
	Quantity     int       `json:"quantity" example:"2"`
	PriceAtOrder float64   `json:"price_at_order" example:"99990.00"`
	CreatedAt    time.Time `json:"created_at"`
}

type Order struct {
	ID             string       `json:"id" example:"a4d2e482-1d54-47ef-a0b2-c07a9b8efaa1"`
	UserID         string       `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status         OrderStatus  `json:"status" example:"CREATED"`
	PromoCodeID    *string      `json:"promo_code_id,omitempty" example:"22222222-2222-2222-2222-222222222222"`
	PromoCode      *string      `json:"promo_code,omitempty" example:"SALE20"`
	TotalAmount    float64      `json:"total_amount" example:"159984.00"`
	DiscountAmount float64      `json:"discount_amount" example:"39996.00"`
	Items          []*OrderItem `json:"items"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

type OrderItemRequest struct {
	ProductID string `json:"product_id" binding:"required,uuid" example:"11111111-1111-1111-1111-111111111111"`
	Quantity  int    `json:"quantity" binding:"required,min=1,max=999" example:"2"`
}

type OrderCreateRequest struct {
	Items     []OrderItemRequest `json:"items" binding:"required,min=1,max=50,dive"`
	PromoCode *string            `json:"promo_code,omitempty" binding:"omitempty,min=4,max=20" example:"SALE20"`
}

type OrderUpdateRequest struct {
	Items []OrderItemRequest `json:"items" binding:"required,min=1,max=50,dive"`
}

type DeficientStockItem struct {
	ProductID      string `json:"product_id"`
	RequestedCount int    `json:"requested_count"`
	AvailableStock int    `json:"available_stock"`
}

// ----------------- Promo Code Models & DTOs -----------------

type PromoCode struct {
	ID             string       `json:"id" example:"22222222-2222-2222-2222-222222222222"`
	Code           string       `json:"code" example:"SALE20"`
	DiscountType   DiscountType `json:"discount_type" example:"PERCENTAGE"`
	DiscountValue  float64      `json:"discount_value" example:"20.00"`
	MinOrderAmount float64      `json:"min_order_amount" example:"1000.00"`
	MaxUses        int          `json:"max_uses" example:"100"`
	CurrentUses    int          `json:"current_uses" example:"5"`
	ValidFrom      time.Time    `json:"valid_from"`
	ValidUntil     time.Time    `json:"valid_until"`
	Active         bool         `json:"active" example:"true"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

type PromoCodeCreateRequest struct {
	Code           string       `json:"code" binding:"required,min=4,max=20" example:"SALE20"`
	DiscountType   DiscountType `json:"discount_type" binding:"required,oneof=PERCENTAGE FIXED_AMOUNT" example:"PERCENTAGE"`
	DiscountValue  float64      `json:"discount_value" binding:"required,gt=0" example:"20.00"`
	MinOrderAmount float64      `json:"min_order_amount" binding:"gte=0" example:"1000.00"`
	MaxUses        int          `json:"max_uses" binding:"required,gt=0" example:"100"`
	ValidFrom      time.Time    `json:"valid_from" binding:"required" example:"2026-01-01T00:00:00Z"`
	ValidUntil     time.Time    `json:"valid_until" binding:"required" example:"2026-12-31T23:59:59Z"`
	Active         bool         `json:"active" example:"true"`
}

// ----------------- User Operation Model -----------------

type UserOperation struct {
	ID            string        `json:"id"`
	UserID        string        `json:"user_id"`
	OperationType OperationType `json:"operation_type"`
	CreatedAt     time.Time     `json:"created_at"`
}

// ----------------- Repository Interfaces -----------------

type ProductRepository interface {
	Create(ctx context.Context, p *Product) error
	GetByID(ctx context.Context, id string) (*Product, error)
	List(ctx context.Context, page, size int, status *ProductStatus, category *string) ([]*Product, int, error)
	Update(ctx context.Context, p *Product) error
	SoftDelete(ctx context.Context, id string) (*Product, error)
}

type OrderRepository interface {
	CreateWithTx(ctx context.Context, order *Order, items []*OrderItem, promo *PromoCode) error
	GetByID(ctx context.Context, id string) (*Order, error)
	HasActiveOrder(ctx context.Context, userID string) (bool, error)
	UpdateWithTx(ctx context.Context, order *Order, newItems []*OrderItem, oldItems []*OrderItem, promo *PromoCode, oldPromo *PromoCode) error
	CancelWithTx(ctx context.Context, order *Order, items []*OrderItem, promo *PromoCode) error
}

type PromoCodeRepository interface {
	Create(ctx context.Context, promo *PromoCode) error
	GetByCode(ctx context.Context, code string) (*PromoCode, error)
	GetByID(ctx context.Context, id string) (*PromoCode, error)
}

type UserOperationRepository interface {
	Record(ctx context.Context, userID string, opType OperationType) error
	GetLastOperation(ctx context.Context, userID string, opType OperationType) (*UserOperation, error)
}

// ----------------- Service Interfaces -----------------

type ProductService interface {
	Create(ctx context.Context, req *ProductCreateRequest) (*Product, error)
	GetByID(ctx context.Context, id string) (*Product, error)
	List(ctx context.Context, page, size int, status *ProductStatus, category *string) (*ProductListResponse, error)
	Update(ctx context.Context, id string, req *ProductUpdateRequest) (*Product, error)
	Delete(ctx context.Context, id string) (*Product, error)
}

type OrderService interface {
	CreateOrder(ctx context.Context, userID string, req *OrderCreateRequest) (*Order, error)
	UpdateOrder(ctx context.Context, userID, orderID string, req *OrderUpdateRequest) (*Order, error)
	CancelOrder(ctx context.Context, userID, orderID string) (*Order, error)
	GetByID(ctx context.Context, orderID string) (*Order, error)
}

type PromoCodeService interface {
	Create(ctx context.Context, req *PromoCodeCreateRequest) (*PromoCode, error)
	GetByCode(ctx context.Context, code string) (*PromoCode, error)
}
