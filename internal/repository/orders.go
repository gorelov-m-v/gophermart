package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"gophermart/internal/database"
)

// Order status constants.
const (
	OrderStatusNew        = "NEW"
	OrderStatusProcessing = "PROCESSING"
	OrderStatusInvalid    = "INVALID"
	OrderStatusProcessed  = "PROCESSED"
)

// Order represents an order entity in the database.
type Order struct {
	ID           int64
	UserID       int64
	Number       string
	Status       string
	Accrual      decimal.NullDecimal
	UploadedAt   time.Time
	ProcessedAt  *time.Time
	Credited     bool
	NextPollAt   *time.Time
	PollAttempts int
	LastError    *string
}

// OrdersRepository defines the interface for order data operations.
type OrdersRepository interface {
	Create(ctx context.Context, userID int64, number string) (*Order, error)
	GetByNumber(ctx context.Context, number string) (*Order, error)
	ListByUser(ctx context.Context, userID int64) ([]*Order, error)
	UpdateAfterPoll(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error
	PickDueOrdersForUpdate(ctx context.Context, tx *sql.Tx, limit int, now time.Time) ([]*Order, error)
	UpdatePickedOrders(ctx context.Context, tx *sql.Tx, orderIDs []int64, nextPollAt time.Time) error
	FinalizeOrderTx(ctx context.Context, tx *sql.Tx, orderID int64, accrual decimal.Decimal, processedAt time.Time) (bool, error)
}

type ordersRepository struct {
	db *sql.DB
}

// NewOrdersRepository creates a new OrdersRepository instance.
func NewOrdersRepository(db *sql.DB) OrdersRepository {
	return &ordersRepository{db: db}
}

func (r *ordersRepository) Create(ctx context.Context, userID int64, number string) (*Order, error) {
	order := &Order{}
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO orders (user_id, number, status)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, number, status, accrual, uploaded_at, processed_at, credited, next_poll_at, poll_attempts, last_error`,
		userID, number, OrderStatusNew,
	).Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &order.Accrual,
		&order.UploadedAt, &order.ProcessedAt, &order.Credited, &order.NextPollAt, &order.PollAttempts, &order.LastError)

	if err != nil {
		if database.IsUniqueViolation(err) {
			return nil, database.ErrAlreadyExists
		}
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	return order, nil
}

func (r *ordersRepository) GetByNumber(ctx context.Context, number string) (*Order, error) {
	order := &Order{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, number, status, accrual, uploaded_at, processed_at, credited, next_poll_at, poll_attempts, last_error
		 FROM orders WHERE number = $1`,
		number,
	).Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &order.Accrual,
		&order.UploadedAt, &order.ProcessedAt, &order.Credited, &order.NextPollAt, &order.PollAttempts, &order.LastError)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, database.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get order by number: %w", err)
	}

	return order, nil
}

func (r *ordersRepository) ListByUser(ctx context.Context, userID int64) ([]*Order, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, user_id, number, status, accrual, uploaded_at, processed_at, credited, next_poll_at, poll_attempts, last_error
		 FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order := &Order{}
		err := rows.Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &order.Accrual,
			&order.UploadedAt, &order.ProcessedAt, &order.Credited, &order.NextPollAt, &order.PollAttempts, &order.LastError)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating orders: %w", err)
	}

	return orders, nil
}

func (r *ordersRepository) PickDueOrdersForUpdate(ctx context.Context, tx *sql.Tx, limit int, now time.Time) ([]*Order, error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, user_id, number, status, accrual, uploaded_at, processed_at, credited, next_poll_at, poll_attempts, last_error
		 FROM orders
		 WHERE status IN ($1, $2) AND (next_poll_at IS NULL OR next_poll_at <= $3)
		 ORDER BY uploaded_at ASC
		 LIMIT $4
		 FOR UPDATE SKIP LOCKED`,
		OrderStatusNew, OrderStatusProcessing, now, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pick orders for update: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order := &Order{}
		err := rows.Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &order.Accrual,
			&order.UploadedAt, &order.ProcessedAt, &order.Credited, &order.NextPollAt, &order.PollAttempts, &order.LastError)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating orders: %w", err)
	}

	return orders, nil
}

func (r *ordersRepository) UpdatePickedOrders(ctx context.Context, tx *sql.Tx, orderIDs []int64, nextPollAt time.Time) error {
	if len(orderIDs) == 0 {
		return nil
	}

	query := `UPDATE orders SET next_poll_at = $1, status = CASE WHEN status = 'NEW' THEN 'PROCESSING' ELSE status END WHERE id = ANY($2)`
	_, err := tx.ExecContext(ctx, query, nextPollAt, pq.Array(orderIDs))
	if err != nil {
		return fmt.Errorf("failed to update picked orders: %w", err)
	}

	return nil
}

func (r *ordersRepository) UpdateAfterPoll(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE orders
		 SET status = $1, accrual = $2, processed_at = $3, next_poll_at = $4, poll_attempts = $5, last_error = $6
		 WHERE id = $7`,
		status, accrual, processedAt, nextPollAt, pollAttempts, lastError, orderID,
	)
	if err != nil {
		return fmt.Errorf("failed to update order after poll: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return database.ErrNotFound
	}

	return nil
}

func (r *ordersRepository) FinalizeOrderTx(ctx context.Context, tx *sql.Tx, orderID int64, accrual decimal.Decimal, processedAt time.Time) (bool, error) {
	var currentCredited bool
	err := tx.QueryRowContext(
		ctx,
		`SELECT credited FROM orders WHERE id = $1 FOR UPDATE`,
		orderID,
	).Scan(&currentCredited)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, database.ErrNotFound
		}
		return false, fmt.Errorf("failed to check credited status: %w", err)
	}

	if currentCredited {
		return false, nil
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE orders
		 SET status = $1, accrual = $2, processed_at = $3, credited = true, next_poll_at = NULL, last_error = NULL
		 WHERE id = $4`,
		OrderStatusProcessed, accrual, processedAt, orderID,
	)
	if err != nil {
		return false, fmt.Errorf("failed to update order: %w", err)
	}

	return true, nil
}
