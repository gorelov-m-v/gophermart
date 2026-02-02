package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Withdrawal represents a withdrawal entity in the database.
type Withdrawal struct {
	ID          int64
	UserID      int64
	OrderNumber string
	Sum         decimal.Decimal
	ProcessedAt time.Time
}

// WithdrawalsRepository defines the interface for withdrawal data operations.
type WithdrawalsRepository interface {
	ListByUser(ctx context.Context, userID int64) ([]*Withdrawal, error)
	ExistsByUserAndOrderTx(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error)
	InsertTx(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string, sum decimal.Decimal, processedAt time.Time) error
}

type withdrawalsRepository struct {
	db *sql.DB
}

// NewWithdrawalsRepository creates a new WithdrawalsRepository instance.
func NewWithdrawalsRepository(db *sql.DB) WithdrawalsRepository {
	return &withdrawalsRepository{db: db}
}

func (r *withdrawalsRepository) ListByUser(ctx context.Context, userID int64) ([]*Withdrawal, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, user_id, order_number, sum, processed_at
		 FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []*Withdrawal
	for rows.Next() {
		withdrawal := &Withdrawal{}
		err := rows.Scan(&withdrawal.ID, &withdrawal.UserID, &withdrawal.OrderNumber, &withdrawal.Sum, &withdrawal.ProcessedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating withdrawals: %w", err)
	}

	return withdrawals, nil
}

func (r *withdrawalsRepository) ExistsByUserAndOrderTx(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM withdrawals WHERE user_id = $1 AND order_number = $2)`,
		userID, orderNumber,
	).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("failed to check withdrawal existence: %w", err)
	}

	return exists, nil
}

func (r *withdrawalsRepository) InsertTx(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string, sum decimal.Decimal, processedAt time.Time) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO withdrawals (user_id, order_number, sum, processed_at)
		 VALUES ($1, $2, $3, $4)`,
		userID, orderNumber, sum, processedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert withdrawal: %w", err)
	}

	return nil
}
