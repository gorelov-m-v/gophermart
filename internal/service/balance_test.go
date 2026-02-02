package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gophermart/internal/database"
	"gophermart/internal/log"
	"gophermart/internal/repository"
)

type MockWithdrawalsRepository struct {
	ListByUserFunc             func(ctx context.Context, userID int64) ([]*repository.Withdrawal, error)
	ExistsByUserAndOrderTxFunc func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error)
	InsertTxFunc               func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string, sum decimal.Decimal, processedAt time.Time) error
}

func (m *MockWithdrawalsRepository) ListByUser(ctx context.Context, userID int64) ([]*repository.Withdrawal, error) {
	if m.ListByUserFunc != nil {
		return m.ListByUserFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *MockWithdrawalsRepository) ExistsByUserAndOrderTx(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error) {
	if m.ExistsByUserAndOrderTxFunc != nil {
		return m.ExistsByUserAndOrderTxFunc(ctx, tx, userID, orderNumber)
	}
	return false, errors.New("not implemented")
}

func (m *MockWithdrawalsRepository) InsertTx(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string, sum decimal.Decimal, processedAt time.Time) error {
	if m.InsertTxFunc != nil {
		return m.InsertTxFunc(ctx, tx, userID, orderNumber, sum, processedAt)
	}
	return errors.New("not implemented")
}

type MockTransactionManager struct {
	WithTransactionFunc func(ctx context.Context, fn func(tx *sql.Tx) error) error
}

func (m *MockTransactionManager) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	if m.WithTransactionFunc != nil {
		return m.WithTransactionFunc(ctx, fn)
	}
	return fn(nil)
}

type MockLogger struct{}

func (m *MockLogger) Debug(msg string, args ...interface{}) {}
func (m *MockLogger) Info(msg string, args ...interface{})  {}
func (m *MockLogger) Warn(msg string, args ...interface{})  {}
func (m *MockLogger) Error(msg string, args ...interface{}) {}
func (m *MockLogger) Fatal(msg string, args ...interface{}) {}
func (m *MockLogger) With(args ...interface{}) log.Logger   { return m }
func (m *MockLogger) Sync() error                           { return nil }

type MockUsersRepositoryForBalance struct {
	MockUsersRepository
	GetBalancesFunc        func(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error)
	GetUserForUpdateTxFunc func(ctx context.Context, tx *sql.Tx, userID int64) (*repository.User, error)
	ApplyWithdrawTxFunc    func(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error
}

func (m *MockUsersRepositoryForBalance) GetBalances(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
	if m.GetBalancesFunc != nil {
		return m.GetBalancesFunc(ctx, userID)
	}
	return decimal.Zero, decimal.Zero, errors.New("not implemented")
}

func (m *MockUsersRepositoryForBalance) GetUserForUpdateTx(ctx context.Context, tx *sql.Tx, userID int64) (*repository.User, error) {
	if m.GetUserForUpdateTxFunc != nil {
		return m.GetUserForUpdateTxFunc(ctx, tx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *MockUsersRepositoryForBalance) ApplyWithdrawTx(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
	if m.ApplyWithdrawTxFunc != nil {
		return m.ApplyWithdrawTxFunc(ctx, tx, userID, amount)
	}
	return errors.New("not implemented")
}

func TestBalanceService_GetBalance_Success(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{
		GetBalancesFunc: func(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
			if userID != 1 {
				t.Errorf("expected userID 1, got %d", userID)
			}
			return decimal.NewFromFloat(1000.50), decimal.NewFromFloat(250.25), nil
		},
	}

	mockWithdrawals := &MockWithdrawalsRepository{}
	mockTxManager := &MockTransactionManager{}
	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	current, withdrawn, err := service.GetBalance(ctx, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expectedCurrent := decimal.NewFromFloat(1000.50)
	if !current.Equal(expectedCurrent) {
		t.Errorf("expected current %s, got %s", expectedCurrent, current)
	}

	expectedWithdrawn := decimal.NewFromFloat(250.25)
	if !withdrawn.Equal(expectedWithdrawn) {
		t.Errorf("expected withdrawn %s, got %s", expectedWithdrawn, withdrawn)
	}
}

func TestBalanceService_GetBalance_Error(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{
		GetBalancesFunc: func(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
			return decimal.Zero, decimal.Zero, database.ErrNotFound
		},
	}

	mockWithdrawals := &MockWithdrawalsRepository{}
	mockTxManager := &MockTransactionManager{}
	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	_, _, err := service.GetBalance(ctx, 999)
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBalanceService_Withdraw_Success(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{
		GetUserForUpdateTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64) (*repository.User, error) {
			return &repository.User{
				ID:             userID,
				CurrentBalance: decimal.NewFromFloat(500.00),
				WithdrawnTotal: decimal.NewFromFloat(100.00),
			}, nil
		},
		ApplyWithdrawTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
			return nil
		},
	}

	mockWithdrawals := &MockWithdrawalsRepository{
		ExistsByUserAndOrderTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error) {
			return false, nil
		},
		InsertTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string, sum decimal.Decimal, processedAt time.Time) error {
			return nil
		},
	}

	mockTxManager := &MockTransactionManager{
		WithTransactionFunc: func(ctx context.Context, fn func(tx *sql.Tx) error) error {
			return fn(nil)
		},
	}

	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	result, err := service.Withdraw(ctx, 1, "4561261212345467", decimal.NewFromFloat(100.00))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != WithdrawOk {
		t.Errorf("expected WithdrawOk, got %v", result)
	}
}

func TestBalanceService_Withdraw_InsufficientFunds(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{
		GetUserForUpdateTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64) (*repository.User, error) {
			return &repository.User{
				ID:             userID,
				CurrentBalance: decimal.NewFromFloat(50.00),
				WithdrawnTotal: decimal.NewFromFloat(100.00),
			}, nil
		},
	}

	mockWithdrawals := &MockWithdrawalsRepository{
		ExistsByUserAndOrderTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error) {
			return false, nil
		},
	}

	mockTxManager := &MockTransactionManager{
		WithTransactionFunc: func(ctx context.Context, fn func(tx *sql.Tx) error) error {
			return fn(nil)
		},
	}

	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	result, err := service.Withdraw(ctx, 1, "4561261212345467", decimal.NewFromFloat(100.00))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != WithdrawInsufficientFunds {
		t.Errorf("expected WithdrawInsufficientFunds, got %v", result)
	}
}

func TestBalanceService_Withdraw_AlreadyProcessed(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{}

	mockWithdrawals := &MockWithdrawalsRepository{
		ExistsByUserAndOrderTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error) {
			return true, nil
		},
	}

	mockTxManager := &MockTransactionManager{
		WithTransactionFunc: func(ctx context.Context, fn func(tx *sql.Tx) error) error {
			return fn(nil)
		},
	}

	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	result, err := service.Withdraw(ctx, 1, "4561261212345467", decimal.NewFromFloat(100.00))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != WithdrawAlreadyProcessed {
		t.Errorf("expected WithdrawAlreadyProcessed, got %v", result)
	}
}

func TestBalanceService_Withdraw_UserNotFound(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{
		GetUserForUpdateTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64) (*repository.User, error) {
			return nil, database.ErrNotFound
		},
	}

	mockWithdrawals := &MockWithdrawalsRepository{
		ExistsByUserAndOrderTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error) {
			return false, nil
		},
	}

	mockTxManager := &MockTransactionManager{
		WithTransactionFunc: func(ctx context.Context, fn func(tx *sql.Tx) error) error {
			return fn(nil)
		},
	}

	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	_, err := service.Withdraw(ctx, 999, "4561261212345467", decimal.NewFromFloat(100.00))
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestBalanceService_Withdraw_ExistenceCheckError(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{}

	mockWithdrawals := &MockWithdrawalsRepository{
		ExistsByUserAndOrderTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error) {
			return false, errors.New("database error")
		},
	}

	mockTxManager := &MockTransactionManager{
		WithTransactionFunc: func(ctx context.Context, fn func(tx *sql.Tx) error) error {
			return fn(nil)
		},
	}

	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	_, err := service.Withdraw(ctx, 1, "4561261212345467", decimal.NewFromFloat(100.00))
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestBalanceService_Withdraw_ApplyWithdrawError(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{
		GetUserForUpdateTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64) (*repository.User, error) {
			return &repository.User{
				ID:             userID,
				CurrentBalance: decimal.NewFromFloat(500.00),
				WithdrawnTotal: decimal.NewFromFloat(100.00),
			}, nil
		},
		ApplyWithdrawTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
			return errors.New("update error")
		},
	}

	mockWithdrawals := &MockWithdrawalsRepository{
		ExistsByUserAndOrderTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error) {
			return false, nil
		},
	}

	mockTxManager := &MockTransactionManager{
		WithTransactionFunc: func(ctx context.Context, fn func(tx *sql.Tx) error) error {
			return fn(nil)
		},
	}

	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	_, err := service.Withdraw(ctx, 1, "4561261212345467", decimal.NewFromFloat(100.00))
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestBalanceService_Withdraw_InsertError(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{
		GetUserForUpdateTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64) (*repository.User, error) {
			return &repository.User{
				ID:             userID,
				CurrentBalance: decimal.NewFromFloat(500.00),
				WithdrawnTotal: decimal.NewFromFloat(100.00),
			}, nil
		},
		ApplyWithdrawTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
			return nil
		},
	}

	mockWithdrawals := &MockWithdrawalsRepository{
		ExistsByUserAndOrderTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error) {
			return false, nil
		},
		InsertTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string, sum decimal.Decimal, processedAt time.Time) error {
			return errors.New("insert error")
		},
	}

	mockTxManager := &MockTransactionManager{
		WithTransactionFunc: func(ctx context.Context, fn func(tx *sql.Tx) error) error {
			return fn(nil)
		},
	}

	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	_, err := service.Withdraw(ctx, 1, "4561261212345467", decimal.NewFromFloat(100.00))
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestBalanceService_ListWithdrawals_Success(t *testing.T) {
	now := time.Now()
	expectedWithdrawals := []*repository.Withdrawal{
		{
			ID:          1,
			UserID:      1,
			OrderNumber: "4561261212345467",
			Sum:         decimal.NewFromFloat(100.00),
			ProcessedAt: now,
		},
		{
			ID:          2,
			UserID:      1,
			OrderNumber: "4561261212345468",
			Sum:         decimal.NewFromFloat(200.00),
			ProcessedAt: now.Add(-time.Hour),
		},
	}

	mockUsers := &MockUsersRepositoryForBalance{}

	mockWithdrawals := &MockWithdrawalsRepository{
		ListByUserFunc: func(ctx context.Context, userID int64) ([]*repository.Withdrawal, error) {
			if userID != 1 {
				t.Errorf("expected userID 1, got %d", userID)
			}
			return expectedWithdrawals, nil
		},
	}

	mockTxManager := &MockTransactionManager{}
	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	withdrawals, err := service.ListWithdrawals(ctx, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(withdrawals) != 2 {
		t.Errorf("expected 2 withdrawals, got %d", len(withdrawals))
	}

	if withdrawals[0].OrderNumber != "4561261212345467" {
		t.Errorf("expected order number 4561261212345467, got %s", withdrawals[0].OrderNumber)
	}
}

func TestBalanceService_ListWithdrawals_Empty(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{}

	mockWithdrawals := &MockWithdrawalsRepository{
		ListByUserFunc: func(ctx context.Context, userID int64) ([]*repository.Withdrawal, error) {
			return []*repository.Withdrawal{}, nil
		},
	}

	mockTxManager := &MockTransactionManager{}
	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	withdrawals, err := service.ListWithdrawals(ctx, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(withdrawals) != 0 {
		t.Errorf("expected 0 withdrawals, got %d", len(withdrawals))
	}
}

func TestBalanceService_ListWithdrawals_Error(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{}

	mockWithdrawals := &MockWithdrawalsRepository{
		ListByUserFunc: func(ctx context.Context, userID int64) ([]*repository.Withdrawal, error) {
			return nil, errors.New("database error")
		},
	}

	mockTxManager := &MockTransactionManager{}
	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	_, err := service.ListWithdrawals(ctx, 1)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestBalanceService_WithdrawLogging(t *testing.T) {
	mockUsers := &MockUsersRepositoryForBalance{
		GetUserForUpdateTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64) (*repository.User, error) {
			return &repository.User{
				ID:             userID,
				CurrentBalance: decimal.NewFromFloat(500.00),
				WithdrawnTotal: decimal.NewFromFloat(100.00),
			}, nil
		},
		ApplyWithdrawTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
			return nil
		},
	}

	mockWithdrawals := &MockWithdrawalsRepository{
		ExistsByUserAndOrderTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string) (bool, error) {
			return false, nil
		},
		InsertTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, orderNumber string, sum decimal.Decimal, processedAt time.Time) error {
			return nil
		},
	}

	mockTxManager := &MockTransactionManager{
		WithTransactionFunc: func(ctx context.Context, fn func(tx *sql.Tx) error) error {
			return fn(nil)
		},
	}

	mockLogger := &MockLogger{}

	service := NewBalanceService(mockUsers, mockWithdrawals, mockTxManager, mockLogger)
	ctx := context.Background()

	result, err := service.Withdraw(ctx, 1, "4561261212345467", decimal.NewFromFloat(100.00))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != WithdrawOk {
		t.Errorf("expected WithdrawOk, got %v", result)
	}
}
