package order

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

func (r *Repository) HasActiveOrder(ctx context.Context, userID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM orders
			WHERE user_id = $1 AND status IN ('CREATED', 'PAYMENT_PENDING')
		)
	`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&exists)
	return exists, err
}

func (r *Repository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	orderQuery := `
		SELECT o.id, o.user_id, o.status, o.promo_code_id, p.code,
		       o.total_amount, o.discount_amount, o.created_at, o.updated_at
		FROM orders o
		LEFT JOIN promo_codes p ON o.promo_code_id = p.id
		WHERE o.id = $1
	`
	var o domain.Order
	var statusStr string
	err := r.db.QueryRowContext(ctx, orderQuery, id).Scan(
		&o.ID, &o.UserID, &statusStr, &o.PromoCodeID, &o.PromoCode,
		&o.TotalAmount, &o.DiscountAmount, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}
	o.Status = domain.OrderStatus(statusStr)

	itemsQuery := `
		SELECT id, order_id, product_id, quantity, price_at_order, created_at
		FROM order_items
		WHERE order_id = $1
	`
	rows, err := r.db.QueryContext(ctx, itemsQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.OrderItem
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(
			&item.ID, &item.OrderID, &item.ProductID, &item.Quantity, &item.PriceAtOrder, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	if items == nil {
		items = []*domain.OrderItem{}
	}
	o.Items = items

	return &o, nil
}

func (r *Repository) CreateWithTx(ctx context.Context, order *domain.Order, items []*domain.OrderItem, promo *domain.PromoCode) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Вставка заказа
	orderQuery := `
		INSERT INTO orders (id, user_id, status, promo_code_id, total_amount, discount_amount, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	if _, err := tx.ExecContext(ctx, orderQuery,
		order.ID, order.UserID, string(order.Status), order.PromoCodeID,
		order.TotalAmount, order.DiscountAmount, order.CreatedAt, order.UpdatedAt,
	); err != nil {
		return err
	}

	// 2. Вставка позиций и списание остатков
	itemQuery := `
		INSERT INTO order_items (id, order_id, product_id, quantity, price_at_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	stockQuery := `UPDATE products SET stock = stock - $1 WHERE id = $2`

	for _, item := range items {
		if _, err := tx.ExecContext(ctx, itemQuery,
			item.ID, item.OrderID, item.ProductID, item.Quantity, item.PriceAtOrder, item.CreatedAt,
		); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, stockQuery, item.Quantity, item.ProductID); err != nil {
			return err
		}
	}

	// 3. Инкремент использования промокода
	if promo != nil {
		promoQuery := `UPDATE promo_codes SET current_uses = current_uses + 1 WHERE id = $1`
		if _, err := tx.ExecContext(ctx, promoQuery, promo.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) UpdateWithTx(
	ctx context.Context,
	order *domain.Order,
	newItems []*domain.OrderItem,
	oldItems []*domain.OrderItem,
	promo *domain.PromoCode,
	oldPromo *domain.PromoCode,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Возврат старых остатков
	returnStockQuery := `UPDATE products SET stock = stock + $1 WHERE id = $2`
	for _, item := range oldItems {
		if _, err := tx.ExecContext(ctx, returnStockQuery, item.Quantity, item.ProductID); err != nil {
			return err
		}
	}

	// 2. Удаление старых позиций
	deleteItemsQuery := `DELETE FROM order_items WHERE order_id = $1`
	if _, err := tx.ExecContext(ctx, deleteItemsQuery, order.ID); err != nil {
		return err
	}

	// 3. Вставка новых позиций и резервирование остатков
	insertItemQuery := `
		INSERT INTO order_items (id, order_id, product_id, quantity, price_at_order, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	reserveStockQuery := `UPDATE products SET stock = stock - $1 WHERE id = $2`
	for _, item := range newItems {
		if _, err := tx.ExecContext(ctx, insertItemQuery,
			item.ID, item.OrderID, item.ProductID, item.Quantity, item.PriceAtOrder, item.CreatedAt,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, reserveStockQuery, item.Quantity, item.ProductID); err != nil {
			return err
		}
	}

	// 4. Корректировка промокода при изменении суммы
	if oldPromo != nil && promo == nil {
		// Промокод стал неприменим - декрементируем
		decPromoQuery := `UPDATE promo_codes SET current_uses = GREATEST(current_uses - 1, 0) WHERE id = $1`
		if _, err := tx.ExecContext(ctx, decPromoQuery, oldPromo.ID); err != nil {
			return err
		}
	}

	// 5. Обновление заказа
	updateOrderQuery := `
		UPDATE orders
		SET total_amount = $1, discount_amount = $2, promo_code_id = $3, updated_at = NOW()
		WHERE id = $4
	`
	if _, err := tx.ExecContext(ctx, updateOrderQuery,
		order.TotalAmount, order.DiscountAmount, order.PromoCodeID, order.ID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *Repository) CancelWithTx(ctx context.Context, order *domain.Order, items []*domain.OrderItem, promo *domain.PromoCode) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Установка статуса CANCELED
	cancelQuery := `UPDATE orders SET status = 'CANCELED', updated_at = NOW() WHERE id = $1`
	if _, err := tx.ExecContext(ctx, cancelQuery, order.ID); err != nil {
		return err
	}

	// 2. Возврат остатков на склад
	returnStockQuery := `UPDATE products SET stock = stock + $1 WHERE id = $2`
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, returnStockQuery, item.Quantity, item.ProductID); err != nil {
			return err
		}
	}

	// 3. Возврат использования промокода
	if promo != nil {
		decPromoQuery := `UPDATE promo_codes SET current_uses = GREATEST(current_uses - 1, 0) WHERE id = $1`
		if _, err := tx.ExecContext(ctx, decPromoQuery, promo.ID); err != nil {
			return err
		}
	}

	return tx.Commit()
}
