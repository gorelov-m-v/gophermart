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

func TestUsersRepository_Create_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	login := "testuser"
	passwordHash := "$2a$10$hashedpassword"
	expectedID := int64(123)

	rows := sqlmock.NewRows([]string{"id"}).AddRow(expectedID)
	mock.ExpectQuery("INSERT INTO users").
		WithArgs(login, passwordHash).
		WillReturnRows(rows)

	userID, err := repo.Create(ctx, login, passwordHash)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if userID != expectedID {
		t.Errorf("expected userID %d, got %d", expectedID, userID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_Create_Conflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("duplicate", "hash").
		WillReturnError(&database.MockUniqueViolationError{})

	_, err = repo.Create(ctx, "duplicate", "hash")
	if !errors.Is(err, database.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_Create_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	expectedErr := errors.New("db error")

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("user", "hash").
		WillReturnError(expectedErr)

	_, err = repo.Create(ctx, "user", "hash")
	if err == nil {
		t.Error("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_GetByLogin_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	login := "testuser"
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "login", "password_hash", "current_balance", "withdrawn_total", "created_at"}).
		AddRow(int64(1), login, "hash", decimal.NewFromFloat(100.50), decimal.NewFromFloat(50.00), now)

	mock.ExpectQuery("SELECT id, login, password_hash, current_balance, withdrawn_total, created_at FROM users WHERE login").
		WithArgs(login).
		WillReturnRows(rows)

	user, err := repo.GetByLogin(ctx, login)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if user.ID != 1 {
		t.Errorf("expected ID 1, got %d", user.ID)
	}
	if user.Login != login {
		t.Errorf("expected login %s, got %s", login, user.Login)
	}
	if user.PasswordHash != "hash" {
		t.Errorf("expected hash 'hash', got %s", user.PasswordHash)
	}
	expectedBalance := decimal.NewFromFloat(100.50)
	if !user.CurrentBalance.Equal(expectedBalance) {
		t.Errorf("expected current_balance 100.50, got %s", user.CurrentBalance)
	}
	expectedWithdrawn := decimal.NewFromFloat(50.00)
	if !user.WithdrawnTotal.Equal(expectedWithdrawn) {
		t.Errorf("expected withdrawn_total 50.00, got %s", user.WithdrawnTotal)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_GetByLogin_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id, login, password_hash, current_balance, withdrawn_total, created_at FROM users WHERE login").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetByLogin(ctx, "nonexistent")
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_GetByID_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	userID := int64(42)
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "login", "password_hash", "current_balance", "withdrawn_total", "created_at"}).
		AddRow(userID, "testuser", "hash", decimal.NewFromFloat(200.00), decimal.NewFromFloat(100.00), now)

	mock.ExpectQuery("SELECT id, login, password_hash, current_balance, withdrawn_total, created_at FROM users WHERE id").
		WithArgs(userID).
		WillReturnRows(rows)

	user, err := repo.GetByID(ctx, userID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if user.ID != userID {
		t.Errorf("expected ID %d, got %d", userID, user.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_GetByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id, login, password_hash, current_balance, withdrawn_total, created_at FROM users WHERE id").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetByID(ctx, 999)
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_GetBalances_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	userID := int64(1)
	expectedCurrent := decimal.NewFromFloat(500.50)
	expectedWithdrawn := decimal.NewFromFloat(42.00)

	rows := sqlmock.NewRows([]string{"current_balance", "withdrawn_total"}).
		AddRow(expectedCurrent, expectedWithdrawn)

	mock.ExpectQuery("SELECT current_balance, withdrawn_total FROM users WHERE id").
		WithArgs(userID).
		WillReturnRows(rows)

	current, withdrawn, err := repo.GetBalances(ctx, userID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !current.Equal(expectedCurrent) {
		t.Errorf("expected current %s, got %s", expectedCurrent, current)
	}
	if !withdrawn.Equal(expectedWithdrawn) {
		t.Errorf("expected withdrawn %s, got %s", expectedWithdrawn, withdrawn)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_GetBalances_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT current_balance, withdrawn_total FROM users WHERE id").
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	_, _, err = repo.GetBalances(ctx, 999)
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_GetUserForUpdateTx_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	userID := int64(1)
	now := time.Now()

	mock.ExpectBegin()
	tx, _ := db.Begin()

	rows := sqlmock.NewRows([]string{"id", "login", "password_hash", "current_balance", "withdrawn_total", "created_at"}).
		AddRow(userID, "testuser", "hash", decimal.NewFromFloat(100.00), decimal.NewFromFloat(50.00), now)

	mock.ExpectQuery("SELECT id, login, password_hash, current_balance, withdrawn_total, created_at FROM users WHERE id .* FOR UPDATE").
		WithArgs(userID).
		WillReturnRows(rows)

	user, err := repo.GetUserForUpdateTx(ctx, tx, userID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if user.ID != userID {
		t.Errorf("expected ID %d, got %d", userID, user.ID)
	}
	expectedBalance := decimal.NewFromFloat(100.00)
	if !user.CurrentBalance.Equal(expectedBalance) {
		t.Errorf("expected balance 100.00, got %s", user.CurrentBalance)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_GetUserForUpdateTx_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	userID := int64(999)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, login, password_hash, current_balance, withdrawn_total, created_at FROM users WHERE id = \\$1 FOR UPDATE").
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	user, err := repo.GetUserForUpdateTx(ctx, tx, userID)
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	if user != nil {
		t.Errorf("expected nil user, got %v", user)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_ApplyWithdrawTx_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	userID := int64(1)
	amount := decimal.NewFromFloat(50.00)

	mock.ExpectBegin()
	tx, _ := db.Begin()

	mock.ExpectExec("UPDATE users SET current_balance = current_balance - \\$1, withdrawn_total = withdrawn_total \\+ \\$1 WHERE id").
		WithArgs(amount, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.ApplyWithdrawTx(ctx, tx, userID, amount)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_ApplyWithdrawTx_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	amount := decimal.NewFromFloat(50.00)

	mock.ExpectBegin()
	tx, _ := db.Begin()

	mock.ExpectExec("UPDATE users SET current_balance = current_balance - \\$1, withdrawn_total = withdrawn_total \\+ \\$1 WHERE id").
		WithArgs(amount, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.ApplyWithdrawTx(ctx, tx, 999, amount)
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_CreditBalanceTx_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	userID := int64(1)
	amount := decimal.NewFromFloat(100.50)

	mock.ExpectBegin()
	tx, _ := db.Begin()

	mock.ExpectExec("UPDATE users SET current_balance = current_balance \\+ \\$1 WHERE id").
		WithArgs(amount, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.CreditBalanceTx(ctx, tx, userID, amount)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestUsersRepository_CreditBalanceTx_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewUsersRepository(db)
	ctx := context.Background()

	amount := decimal.NewFromFloat(100.50)

	mock.ExpectBegin()
	tx, _ := db.Begin()

	mock.ExpectExec("UPDATE users SET current_balance = current_balance \\+ \\$1 WHERE id").
		WithArgs(amount, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.CreditBalanceTx(ctx, tx, 999, amount)
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
