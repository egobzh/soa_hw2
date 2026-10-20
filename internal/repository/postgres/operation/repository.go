package operation

import (
	"context"
	"database/sql"
	"errors"

	"marketplace/internal/domain"
	"github.com/google/uuid"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Record(ctx context.Context, userID string, opType domain.OperationType) error {
	query := `
		INSERT INTO user_operations (id, user_id, operation_type, created_at)
		VALUES ($1, $2, $3, NOW())
	`
	_, err := r.db.ExecContext(ctx, query, uuid.New().String(), userID, string(opType))
	return err
}

func (r *Repository) GetLastOperation(ctx context.Context, userID string, opType domain.OperationType) (*domain.UserOperation, error) {
	query := `
		SELECT id, user_id, operation_type, created_at
		FROM user_operations
		WHERE user_id = $1 AND operation_type = $2
		ORDER BY created_at DESC
		LIMIT 1
	`
	var op domain.UserOperation
	var opTypeStr string
	err := r.db.QueryRowContext(ctx, query, userID, string(opType)).Scan(
		&op.ID, &op.UserID, &opTypeStr, &op.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // нет предыдущих операций
		}
		return nil, err
	}
	op.OperationType = domain.OperationType(opTypeStr)
	return &op, nil
}
