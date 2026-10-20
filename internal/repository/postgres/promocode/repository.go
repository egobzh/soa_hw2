package promocode

import (
	"context"
	"database/sql"
	"errors"

	"marketplace/internal/domain"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, p *domain.PromoCode) error {
	query := `
		INSERT INTO promo_codes (
			id, code, discount_type, discount_value, min_order_amount,
			max_uses, current_uses, valid_from, valid_until, active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.Code, p.DiscountType, p.DiscountValue, p.MinOrderAmount,
		p.MaxUses, p.CurrentUses, p.ValidFrom, p.ValidUntil, p.Active, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *Repository) GetByCode(ctx context.Context, code string) (*domain.PromoCode, error) {
	query := `
		SELECT id, code, discount_type, discount_value, min_order_amount,
		       max_uses, current_uses, valid_from, valid_until, active, created_at, updated_at
		FROM promo_codes
		WHERE UPPER(code) = UPPER($1)
	`
	return scanPromo(r.db.QueryRowContext(ctx, query, code))
}

func (r *Repository) GetByID(ctx context.Context, id string) (*domain.PromoCode, error) {
	query := `
		SELECT id, code, discount_type, discount_value, min_order_amount,
		       max_uses, current_uses, valid_from, valid_until, active, created_at, updated_at
		FROM promo_codes
		WHERE id = $1
	`
	return scanPromo(r.db.QueryRowContext(ctx, query, id))
}

func scanPromo(row *sql.Row) (*domain.PromoCode, error) {
	var p domain.PromoCode
	err := row.Scan(
		&p.ID, &p.Code, &p.DiscountType, &p.DiscountValue, &p.MinOrderAmount,
		&p.MaxUses, &p.CurrentUses, &p.ValidFrom, &p.ValidUntil, &p.Active, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPromoCodeInvalid
		}
		return nil, err
	}
	return &p, nil
}
