package service

import (
	"context"
	"strings"
	"time"

	"marketplace/internal/domain"
	"github.com/google/uuid"
)

type ProductService struct {
	repo domain.ProductRepository
}

func NewProductService(repo domain.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) Create(ctx context.Context, req *domain.ProductCreateRequest) (*domain.Product, error) {
	name := strings.TrimSpace(req.Name)
	category := strings.TrimSpace(req.Category)

	now := time.Now().UTC()
	p := &domain.Product{
		ID:          uuid.New().String(),
		Name:        name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		Category:    category,
		Status:      req.Status,
		SellerID:    req.SellerID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}

	return p, nil
}

func (s *ProductService) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ProductService) List(ctx context.Context, page, size int, status *domain.ProductStatus, category *string) (*domain.ProductListResponse, error) {
	products, total, err := s.repo.List(ctx, page, size, status, category)
	if err != nil {
		return nil, err
	}

	return &domain.ProductListResponse{
		Items:         products,
		TotalElements: total,
		Page:          page,
		Size:          size,
	}, nil
}

func (s *ProductService) Update(ctx context.Context, id string, req *domain.ProductUpdateRequest) (*domain.Product, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	p.Name = strings.TrimSpace(req.Name)
	p.Description = req.Description
	p.Price = req.Price
	p.Stock = req.Stock
	p.Category = strings.TrimSpace(req.Category)
	p.Status = req.Status
	p.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}

	return p, nil
}

func (s *ProductService) Delete(ctx context.Context, id string) (*domain.Product, error) {
	return s.repo.SoftDelete(ctx, id)
}
