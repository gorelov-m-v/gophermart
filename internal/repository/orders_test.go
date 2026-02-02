package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
	"gophermart/internal/database"
)

func TestOrdersRepository_Create_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	userID := int64(1)
	number := "12345678903"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "number", "status", "accrual",
		"uploaded_at", "processed_at", "credited", "next_poll_at", "poll_attempts", "last_error",
	}).AddRow(
		int64(10), userID, number, OrderStatusNew, nil,
		now, nil, false, nil, 0, nil,
	)

	mock.ExpectQuery("INSERT INTO orders").
		WithArgs(userID, number, OrderStatusNew).
		WillReturnRows(rows)

	order, err := repo.Create(ctx, userID, number)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if order.ID != 10 {
		t.Errorf("expected ID 10, got %d", order.ID)
	}
	if order.Number != number {
		t.Errorf("expected number %s, got %s", number, order.Number)
	}
	if order.Status != OrderStatusNew {
		t.Errorf("expected status %s, got %s", OrderStatusNew, order.Status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_Create_UniqueViolation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("INSERT INTO orders").
		WithArgs(int64(1), "duplicate", OrderStatusNew).
		WillReturnError(&database.MockUniqueViolationError{})

	_, err = repo.Create(ctx, 1, "duplicate")
	if !errors.Is(err, database.ErrAlreadyExists) {
		t.Errorf("expected ErrAlreadyExists, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_GetByNumber_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	number := "12345678903"
	accrual := decimal.NewFromFloat(500.00)
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "number", "status", "accrual",
		"uploaded_at", "processed_at", "credited", "next_poll_at", "poll_attempts", "last_error",
	}).AddRow(
		int64(1), int64(10), number, OrderStatusProcessed, accrual,
		now, &now, true, nil, 5, nil,
	)

	mock.ExpectQuery("SELECT .* FROM orders WHERE number").
		WithArgs(number).
		WillReturnRows(rows)

	order, err := repo.GetByNumber(ctx, number)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if order.Number != number {
		t.Errorf("expected number %s, got %s", number, order.Number)
	}
	if order.Status != OrderStatusProcessed {
		t.Errorf("expected status %s, got %s", OrderStatusProcessed, order.Status)
	}
	if !order.Accrual.Valid || !order.Accrual.Decimal.Equal(accrual) {
		t.Errorf("expected accrual %s, got %v", accrual, order.Accrual)
	}
	if !order.Credited {
		t.Error("expected credited to be true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_GetByNumber_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT .* FROM orders WHERE number").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetByNumber(ctx, "nonexistent")
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_ListByUser_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	userID := int64(1)
	now := time.Now()
	accrual := decimal.NewFromFloat(100.00)

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "number", "status", "accrual",
		"uploaded_at", "processed_at", "credited", "next_poll_at", "poll_attempts", "last_error",
	}).
		AddRow(int64(3), userID, "333", OrderStatusProcessed, accrual, now, &now, true, nil, 1, nil).
		AddRow(int64(2), userID, "222", OrderStatusProcessing, nil, now.Add(-time.Hour), nil, false, &now, 2, nil).
		AddRow(int64(1), userID, "111", OrderStatusNew, nil, now.Add(-2*time.Hour), nil, false, nil, 0, nil)

	mock.ExpectQuery("SELECT .* FROM orders WHERE user_id .* ORDER BY uploaded_at DESC").
		WithArgs(userID).
		WillReturnRows(rows)

	orders, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(orders) != 3 {
		t.Errorf("expected 3 orders, got %d", len(orders))
	}

	if orders[0].Number != "333" {
		t.Errorf("expected first order number 333, got %s", orders[0].Number)
	}
	if orders[2].Number != "111" {
		t.Errorf("expected last order number 111, got %s", orders[2].Number)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_ListByUser_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "number", "status", "accrual",
		"uploaded_at", "processed_at", "credited", "next_poll_at", "poll_attempts", "last_error",
	})

	mock.ExpectQuery("SELECT .* FROM orders WHERE user_id .* ORDER BY uploaded_at DESC").
		WithArgs(int64(999)).
		WillReturnRows(rows)

	orders, err := repo.ListByUser(ctx, 999)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(orders) != 0 {
		t.Errorf("expected empty list, got %d orders", len(orders))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_UpdateAfterPoll_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	orderID := int64(1)
	status := OrderStatusProcessing
	accrual := decimal.NullDecimal{Decimal: decimal.NewFromFloat(100.50), Valid: true}
	now := time.Now()
	nextPollAt := now.Add(5 * time.Second)
	attempts := 3
	lastError := "some error"

	mock.ExpectExec("UPDATE orders SET status = \\$1, accrual = \\$2, processed_at = \\$3, next_poll_at = \\$4, poll_attempts = \\$5, last_error = \\$6 WHERE id").
		WithArgs(status, accrual, &now, &nextPollAt, attempts, &lastError, orderID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.UpdateAfterPoll(ctx, orderID, status, accrual, &now, &nextPollAt, attempts, &lastError)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_UpdateAfterPoll_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	orderID := int64(999)
	status := OrderStatusProcessed
	accrual := decimal.NullDecimal{Decimal: decimal.NewFromFloat(100.00), Valid: true}
	processedAt := time.Now()

	mock.ExpectExec("UPDATE orders SET status = \\$1, accrual = \\$2, processed_at = \\$3, next_poll_at = \\$4, poll_attempts = \\$5, last_error = \\$6 WHERE id = \\$7").
		WithArgs(status, accrual, &processedAt, nil, 1, nil, orderID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.UpdateAfterPoll(ctx, orderID, status, accrual, &processedAt, nil, 1, nil)
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_FinalizeOrderTx_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	orderID := int64(1)
	accrual := decimal.NewFromFloat(500.00)
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT credited FROM orders WHERE id .* FOR UPDATE").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"credited"}).AddRow(false))

	mock.ExpectExec("UPDATE orders SET status = \\$1, accrual = \\$2, processed_at = \\$3, credited = true, next_poll_at = NULL, last_error = NULL WHERE id").
		WithArgs(OrderStatusProcessed, accrual, sqlmock.AnyArg(), orderID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	credited, err := repo.FinalizeOrderTx(ctx, tx, orderID, accrual, now)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !credited {
		t.Error("expected credited to be true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_FinalizeOrderTx_AlreadyCredited(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	orderID := int64(1)
	accrual := decimal.NewFromFloat(500.00)
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT credited FROM orders WHERE id .* FOR UPDATE").
		WithArgs(orderID).
		WillReturnRows(sqlmock.NewRows([]string{"credited"}).AddRow(true))

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	credited, err := repo.FinalizeOrderTx(ctx, tx, orderID, accrual, now)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if credited {
		t.Error("expected credited to be false (already credited)")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_PickDueOrdersForUpdate_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	limit := 10
	now := time.Now()

	mock.ExpectBegin()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "number", "status", "accrual",
		"uploaded_at", "processed_at", "credited", "next_poll_at", "poll_attempts", "last_error",
	}).
		AddRow(int64(1), int64(10), "111", OrderStatusNew, nil, now, nil, false, nil, 0, nil).
		AddRow(int64(2), int64(11), "222", OrderStatusProcessing, nil, now, nil, false, &now, 2, nil)

	mock.ExpectQuery("SELECT .* FROM orders WHERE status IN .* FOR UPDATE SKIP LOCKED").
		WithArgs(OrderStatusNew, OrderStatusProcessing, sqlmock.AnyArg(), limit).
		WillReturnRows(rows)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	orders, err := repo.PickDueOrdersForUpdate(ctx, tx, limit, now)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(orders) != 2 {
		t.Errorf("expected 2 orders, got %d", len(orders))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_PickDueOrdersForUpdate_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	now := time.Now()

	mock.ExpectBegin()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "number", "status", "accrual",
		"uploaded_at", "processed_at", "credited", "next_poll_at", "poll_attempts", "last_error",
	})

	mock.ExpectQuery("SELECT .* FROM orders WHERE status IN .* FOR UPDATE SKIP LOCKED").
		WithArgs(OrderStatusNew, OrderStatusProcessing, sqlmock.AnyArg(), 10).
		WillReturnRows(rows)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	orders, err := repo.PickDueOrdersForUpdate(ctx, tx, 10, now)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(orders) != 0 {
		t.Errorf("expected empty result, got %d orders", len(orders))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_UpdatePickedOrders_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	orderIDs := []int64{1, 2, 3}
	nextPollAt := time.Now().Add(1 * time.Second)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE orders SET next_poll_at = \\$1, status = CASE WHEN status = 'NEW' THEN 'PROCESSING' ELSE status END WHERE id = ANY").
		WithArgs(nextPollAt, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 3))

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	err = repo.UpdatePickedOrders(ctx, tx, orderIDs, nextPollAt)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrdersRepository_UpdatePickedOrders_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrdersRepository(db)
	ctx := context.Background()

	orderIDs := []int64{}
	nextPollAt := time.Now().Add(1 * time.Second)

	err = repo.UpdatePickedOrders(ctx, nil, orderIDs, nextPollAt)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
