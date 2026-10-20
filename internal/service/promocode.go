package service

import (
	"context"
	"strings"
	"time"

	"marketplace/internal/domain"
	"github.com/google/uuid"
)

type PromoCodeService struct {
	repo domain.PromoCodeRepository
}

func NewPromoCodeService(repo domain.PromoCodeRepository) *PromoCodeService {
	return &PromoCodeService{repo: repo}
}

func (s *PromoCodeService) Create(ctx context.Context, req *domain.PromoCodeCreateRequest) (*domain.PromoCode, error) {
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	now := time.Now().UTC()

	promo := &domain.PromoCode{
		ID:             uuid.New().String(),
		Code:           code,
		DiscountType:   req.DiscountType,
		DiscountValue:  req.DiscountValue,
		MinOrderAmount: req.MinOrderAmount,
		MaxUses:        req.MaxUses,
		CurrentUses:    0,
		ValidFrom:      req.ValidFrom,
		ValidUntil:     req.ValidUntil,
		Active:         req.Active,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.Create(ctx, promo); err != nil {
		return nil, err
	}

	return promo, nil
}

func (s *PromoCodeService) GetByCode(ctx context.Context, code string) (*domain.PromoCode, error) {
	return s.repo.GetByCode(ctx, code)
}
