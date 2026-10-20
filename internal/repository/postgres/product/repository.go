package product

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"marketplace/internal/domain"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, p *domain.Product) error {
	query := `
		INSERT INTO products (id, name, description, price, stock, category, status, seller_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		p.ID, p.Name, p.Description, p.Price, p.Stock, p.Category, p.Status, p.SellerID, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	query := `
		SELECT id, name, description, price, stock, category, status, seller_id, created_at, updated_at
		FROM products
		WHERE id = $1
	`
	return scanProduct(r.db.QueryRowContext(ctx, query, id))
}

func (r *Repository) List(ctx context.Context, page, size int, status *domain.ProductStatus, category *string) ([]*domain.Product, int, error) {
	if size <= 0 {
		size = 20
	}
	if page < 0 {
		page = 0
	}
	offset := page * size

	var whereClauses []string
	var args []any
	argIndex := 1

	if status != nil && *status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, string(*status))
		argIndex++
	}

	if category != nil && *category != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("category = $%d", argIndex))
		args = append(args, *category)
		argIndex++
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM products %s", whereSQL)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := fmt.Sprintf(`
		SELECT id, name, description, price, stock, category, status, seller_id, created_at, updated_at
		FROM products
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIndex, argIndex+1)

	args = append(args, size, offset)

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.Category, &p.Status, &p.SellerID, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		products = append(products, &p)
	}

	if products == nil {
		products = []*domain.Product{}
	}

	return products, total, nil
}

func (r *Repository) Update(ctx context.Context, p *domain.Product) error {
	query := `
		UPDATE products
		SET name = $1, description = $2, price = $3, stock = $4, category = $5, status = $6, updated_at = NOW()
		WHERE id = $7
	`
	res, err := r.db.ExecContext(ctx, query,
		p.Name, p.Description, p.Price, p.Stock, p.Category, p.Status, p.ID,
	)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrProductNotFound
	}
	return nil
}

func (r *Repository) SoftDelete(ctx context.Context, id string) (*domain.Product, error) {
	query := `
		UPDATE products
		SET status = 'ARCHIVED', updated_at = NOW()
		WHERE id = $1
		RETURNING id, name, description, price, stock, category, status, seller_id, created_at, updated_at
	`
	return scanProduct(r.db.QueryRowContext(ctx, query, id))
}

func scanProduct(row *sql.Row) (*domain.Product, error) {
	var p domain.Product
	err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.Category, &p.Status, &p.SellerID, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrProductNotFound
		}
		return nil, err
	}
	return &p, nil
}
