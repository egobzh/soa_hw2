package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	SQL *sql.DB
}

func NewDB(databaseURL string) (*DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	pg := &DB{SQL: db}
	if err := pg.initSchema(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return pg, nil
}

func (p *DB) initSchema(ctx context.Context) error {
	query := `
	CREATE EXTENSION IF NOT EXISTS "pgcrypto";

	CREATE OR REPLACE FUNCTION update_updated_at_column()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = NOW();
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;

	-- 1. Таблица products (Пункт 4)
	CREATE TABLE IF NOT EXISTS products (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL,
		description VARCHAR(4000),
		price DECIMAL(12, 2) NOT NULL CHECK (price >= 0.01),
		stock INT NOT NULL CHECK (stock >= 0),
		category VARCHAR(100) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'INACTIVE', 'ARCHIVED')),
		seller_id UUID,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	-- Индекс на поле status (Обязательное требование пункта 4)
	CREATE INDEX IF NOT EXISTS idx_products_status ON products(status);
	CREATE INDEX IF NOT EXISTS idx_products_category ON products(category);

	DROP TRIGGER IF EXISTS trigger_products_updated_at ON products;
	CREATE TRIGGER trigger_products_updated_at
	BEFORE UPDATE ON products
	FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

	-- 2. Таблица promo_codes (Пункт 7)
	CREATE TABLE IF NOT EXISTS promo_codes (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		code VARCHAR(20) NOT NULL UNIQUE,
		discount_type VARCHAR(20) NOT NULL CHECK (discount_type IN ('PERCENTAGE', 'FIXED_AMOUNT')),
		discount_value DECIMAL(12, 2) NOT NULL CHECK (discount_value >= 0.01),
		min_order_amount DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
		max_uses INT NOT NULL CHECK (max_uses >= 1),
		current_uses INT NOT NULL DEFAULT 0 CHECK (current_uses >= 0),
		valid_from TIMESTAMP WITH TIME ZONE NOT NULL,
		valid_until TIMESTAMP WITH TIME ZONE NOT NULL,
		active BOOLEAN NOT NULL DEFAULT TRUE,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_promo_codes_code ON promo_codes(code);

	DROP TRIGGER IF EXISTS trigger_promo_codes_updated_at ON promo_codes;
	CREATE TRIGGER trigger_promo_codes_updated_at
	BEFORE UPDATE ON promo_codes
	FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

	-- 3. Таблица orders (Пункт 7)
	CREATE TABLE IF NOT EXISTS orders (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'CREATED' CHECK (status IN ('CREATED', 'PAYMENT_PENDING', 'PAID', 'SHIPPED', 'COMPLETED', 'CANCELED')),
		promo_code_id UUID REFERENCES promo_codes(id) ON DELETE SET NULL,
		total_amount DECIMAL(12, 2) NOT NULL,
		discount_amount DECIMAL(12, 2) NOT NULL DEFAULT 0.00,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
	CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);

	DROP TRIGGER IF EXISTS trigger_orders_updated_at ON orders;
	CREATE TRIGGER trigger_orders_updated_at
	BEFORE UPDATE ON orders
	FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

	-- 4. Таблица order_items (Пункт 7)
	CREATE TABLE IF NOT EXISTS order_items (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
		product_id UUID NOT NULL REFERENCES products(id),
		quantity INT NOT NULL CHECK (quantity >= 1 AND quantity <= 999),
		price_at_order DECIMAL(12, 2) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);

	-- 5. Таблица user_operations (Пункт 7)
	CREATE TABLE IF NOT EXISTS user_operations (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL,
		operation_type VARCHAR(30) NOT NULL CHECK (operation_type IN ('CREATE_ORDER', 'UPDATE_ORDER')),
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_user_operations_user_type_created ON user_operations(user_id, operation_type, created_at DESC);

	-- Предустановленные тестовые промокоды
	INSERT INTO promo_codes (id, code, discount_type, discount_value, min_order_amount, max_uses, current_uses, valid_from, valid_until, active)
	VALUES 
		('11111111-1111-1111-1111-111111111111', 'SALE20', 'PERCENTAGE', 20.00, 1000.00, 100, 0, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', true),
		('22222222-2222-2222-2222-222222222222', 'FIXED500', 'FIXED_AMOUNT', 500.00, 2000.00, 50, 0, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', true),
		('33333333-3333-3333-3333-333333333333', 'EXPIRED10', 'PERCENTAGE', 10.00, 50.00, 10, 0, NOW() - INTERVAL '10 days', NOW() - INTERVAL '1 day', true),
		('44444444-4444-4444-4444-444444444444', 'EXHAUSTED', 'PERCENTAGE', 15.00, 50.00, 5, 5, NOW() - INTERVAL '1 day', NOW() + INTERVAL '30 days', true)
	ON CONFLICT (code) DO NOTHING;
	`
	_, err := p.SQL.ExecContext(ctx, query)
	return err
}
