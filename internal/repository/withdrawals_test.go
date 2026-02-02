package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
)

func TestWithdrawalsRepository_InsertTx_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewWithdrawalsRepository(db)
	ctx := context.Background()

	userID := int64(1)
	orderNumber := "2377225624"
	sum := decimal.NewFromFloat(500.00)
	processedAt := time.Now()

	mock.ExpectBegin()
	tx, _ := db.Begin()

	mock.ExpectExec("INSERT INTO withdrawals").
		WithArgs(userID, orderNumber, sum, processedAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.InsertTx(ctx, tx, userID, orderNumber, sum, processedAt)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestWithdrawalsRepository_InsertTx_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewWithdrawalsRepository(db)
	ctx := context.Background()

	expectedErr := errors.New("insert error")

	mock.ExpectBegin()
	tx, _ := db.Begin()

	mock.ExpectExec("INSERT INTO withdrawals").
		WithArgs(int64(1), "order", decimal.NewFromFloat(100.00), sqlmock.AnyArg()).
		WillReturnError(expectedErr)

	err = repo.InsertTx(ctx, tx, 1, "order", decimal.NewFromFloat(100.00), time.Now())
	if err == nil {
		t.Error("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestWithdrawalsRepository_ExistsByUserAndOrderTx_Exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewWithdrawalsRepository(db)
	ctx := context.Background()

	userID := int64(1)
	orderNumber := "2377225624"

	mock.ExpectBegin()
	tx, _ := db.Begin()

	rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM withdrawals WHERE user_id = \\$1 AND order_number = \\$2\\)").
		WithArgs(userID, orderNumber).
		WillReturnRows(rows)

	exists, err := repo.ExistsByUserAndOrderTx(ctx, tx, userID, orderNumber)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !exists {
		t.Error("expected exists to be true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestWithdrawalsRepository_ExistsByUserAndOrderTx_NotExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewWithdrawalsRepository(db)
	ctx := context.Background()

	mock.ExpectBegin()
	tx, _ := db.Begin()

	rows := sqlmock.NewRows([]string{"exists"}).AddRow(false)
	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM withdrawals WHERE user_id = \\$1 AND order_number = \\$2\\)").
		WithArgs(int64(1), "nonexistent").
		WillReturnRows(rows)

	exists, err := repo.ExistsByUserAndOrderTx(ctx, tx, 1, "nonexistent")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if exists {
		t.Error("expected exists to be false")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestWithdrawalsRepository_ExistsByUserAndOrderTx_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewWithdrawalsRepository(db)
	ctx := context.Background()

	expectedErr := errors.New("query error")

	mock.ExpectBegin()
	tx, _ := db.Begin()

	mock.ExpectQuery("SELECT EXISTS\\(SELECT 1 FROM withdrawals WHERE user_id = \\$1 AND order_number = \\$2\\)").
		WithArgs(int64(1), "order").
		WillReturnError(expectedErr)

	_, err = repo.ExistsByUserAndOrderTx(ctx, tx, 1, "order")
	if err == nil {
		t.Error("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestWithdrawalsRepository_ListByUser_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewWithdrawalsRepository(db)
	ctx := context.Background()

	userID := int64(1)
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "user_id", "order_number", "sum", "processed_at"}).
		AddRow(int64(3), userID, "333", decimal.NewFromFloat(300.00), now).
		AddRow(int64(2), userID, "222", decimal.NewFromFloat(200.00), now.Add(-time.Hour)).
		AddRow(int64(1), userID, "111", decimal.NewFromFloat(100.00), now.Add(-2*time.Hour))

	mock.ExpectQuery("SELECT id, user_id, order_number, sum, processed_at FROM withdrawals WHERE user_id = \\$1 ORDER BY processed_at DESC").
		WithArgs(userID).
		WillReturnRows(rows)

	withdrawals, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(withdrawals) != 3 {
		t.Errorf("expected 3 withdrawals, got %d", len(withdrawals))
	}

	if withdrawals[0].OrderNumber != "333" {
		t.Errorf("expected first order 333, got %s", withdrawals[0].OrderNumber)
	}
	expectedSum := decimal.NewFromFloat(300.00)
	if !withdrawals[0].Sum.Equal(expectedSum) {
		t.Errorf("expected first sum 300.00, got %s", withdrawals[0].Sum)
	}
	if withdrawals[2].OrderNumber != "111" {
		t.Errorf("expected last order 111, got %s", withdrawals[2].OrderNumber)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestWithdrawalsRepository_ListByUser_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewWithdrawalsRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "user_id", "order_number", "sum", "processed_at"})

	mock.ExpectQuery("SELECT id, user_id, order_number, sum, processed_at FROM withdrawals WHERE user_id = \\$1 ORDER BY processed_at DESC").
		WithArgs(int64(999)).
		WillReturnRows(rows)

	withdrawals, err := repo.ListByUser(ctx, 999)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(withdrawals) != 0 {
		t.Errorf("expected empty list, got %d withdrawals", len(withdrawals))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestWithdrawalsRepository_ListByUser_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewWithdrawalsRepository(db)
	ctx := context.Background()

	expectedErr := errors.New("query error")

	mock.ExpectQuery("SELECT id, user_id, order_number, sum, processed_at FROM withdrawals WHERE user_id = \\$1 ORDER BY processed_at DESC").
		WithArgs(int64(1)).
		WillReturnError(expectedErr)

	_, err = repo.ListByUser(ctx, 1)
	if err == nil {
		t.Error("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestWithdrawalsRepository_ListByUser_ScanError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewWithdrawalsRepository(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "user_id", "order_number", "sum", "processed_at"}).
		AddRow(int64(1), int64(1), "111", decimal.NewFromFloat(100.00), time.Now()).
		AddRow("invalid", int64(1), "222", decimal.NewFromFloat(200.00), time.Now())

	mock.ExpectQuery("SELECT id, user_id, order_number, sum, processed_at FROM withdrawals WHERE user_id = \\$1 ORDER BY processed_at DESC").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	_, err = repo.ListByUser(ctx, 1)
	if err == nil {
		t.Error("expected scan error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestWithdrawalsRepository_ListByUser_RowsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewWithdrawalsRepository(db)
	ctx := context.Background()

	expectedErr := errors.New("rows iteration error")

	rows := sqlmock.NewRows([]string{"id", "user_id", "order_number", "sum", "processed_at"}).
		AddRow(int64(1), int64(1), "111", decimal.NewFromFloat(100.00), time.Now()).
		RowError(0, expectedErr)

	mock.ExpectQuery("SELECT id, user_id, order_number, sum, processed_at FROM withdrawals WHERE user_id = \\$1 ORDER BY processed_at DESC").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	_, err = repo.ListByUser(ctx, 1)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) && err.Error() != "error iterating withdrawals: rows iteration error" {
		t.Errorf("expected wrapped error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
