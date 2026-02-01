package worker

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gophermart/internal/accrual"
	"gophermart/internal/log"
	"gophermart/internal/repository"
)

type MockOrdersRepository struct {
	PickDueOrdersForUpdateFunc func(ctx context.Context, tx *sql.Tx, limit int, now time.Time) ([]*repository.Order, error)
	UpdatePickedOrdersFunc     func(ctx context.Context, tx *sql.Tx, orderIDs []int64, nextPollAt time.Time) error
	UpdateAfterPollFunc        func(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error
	FinalizeOrderTxFunc        func(ctx context.Context, tx *sql.Tx, orderID int64, accrual decimal.Decimal, processedAt time.Time) (bool, error)
}

func (m *MockOrdersRepository) Create(ctx context.Context, userID int64, number string) (*repository.Order, error) {
	return nil, errors.New("not implemented")
}

func (m *MockOrdersRepository) GetByNumber(ctx context.Context, number string) (*repository.Order, error) {
	return nil, errors.New("not implemented")
}

func (m *MockOrdersRepository) ListByUser(ctx context.Context, userID int64) ([]*repository.Order, error) {
	return nil, errors.New("not implemented")
}

func (m *MockOrdersRepository) PickDueOrdersForUpdate(ctx context.Context, tx *sql.Tx, limit int, now time.Time) ([]*repository.Order, error) {
	if m.PickDueOrdersForUpdateFunc != nil {
		return m.PickDueOrdersForUpdateFunc(ctx, tx, limit, now)
	}
	return nil, errors.New("not implemented")
}

func (m *MockOrdersRepository) UpdatePickedOrders(ctx context.Context, tx *sql.Tx, orderIDs []int64, nextPollAt time.Time) error {
	if m.UpdatePickedOrdersFunc != nil {
		return m.UpdatePickedOrdersFunc(ctx, tx, orderIDs, nextPollAt)
	}
	return errors.New("not implemented")
}

func (m *MockOrdersRepository) UpdateAfterPoll(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error {
	if m.UpdateAfterPollFunc != nil {
		return m.UpdateAfterPollFunc(ctx, orderID, status, accrual, processedAt, nextPollAt, pollAttempts, lastError)
	}
	return errors.New("not implemented")
}

func (m *MockOrdersRepository) FinalizeOrderTx(ctx context.Context, tx *sql.Tx, orderID int64, accrual decimal.Decimal, processedAt time.Time) (bool, error) {
	if m.FinalizeOrderTxFunc != nil {
		return m.FinalizeOrderTxFunc(ctx, tx, orderID, accrual, processedAt)
	}
	return false, errors.New("not implemented")
}

type MockAccrualClient struct {
	GetOrderInfoFunc func(ctx context.Context, orderNumber string) (*accrual.AccrualResult, error)
}

func (m *MockAccrualClient) GetOrderInfo(ctx context.Context, orderNumber string) (*accrual.AccrualResult, error) {
	if m.GetOrderInfoFunc != nil {
		return m.GetOrderInfoFunc(ctx, orderNumber)
	}
	return nil, errors.New("not implemented")
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

type MockUsersRepository struct {
	CreditBalanceTxFunc func(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error
}

func (m *MockUsersRepository) Create(ctx context.Context, login, passwordHash string) (int64, error) {
	return 0, errors.New("not implemented")
}

func (m *MockUsersRepository) GetByLogin(ctx context.Context, login string) (*repository.User, error) {
	return nil, errors.New("not implemented")
}

func (m *MockUsersRepository) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	return nil, errors.New("not implemented")
}

func (m *MockUsersRepository) GetBalances(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
	return decimal.Zero, decimal.Zero, errors.New("not implemented")
}

func (m *MockUsersRepository) GetUserForUpdateTx(ctx context.Context, tx *sql.Tx, userID int64) (*repository.User, error) {
	return nil, errors.New("not implemented")
}

func (m *MockUsersRepository) ApplyWithdrawTx(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
	return errors.New("not implemented")
}

func (m *MockUsersRepository) CreditBalanceTx(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
	if m.CreditBalanceTxFunc != nil {
		return m.CreditBalanceTxFunc(ctx, tx, userID, amount)
	}
	return errors.New("not implemented")
}

type MockLogger struct{}

func (m *MockLogger) Debug(msg string, args ...interface{}) {}
func (m *MockLogger) Info(msg string, args ...interface{})  {}
func (m *MockLogger) Warn(msg string, args ...interface{})  {}
func (m *MockLogger) Error(msg string, args ...interface{}) {}
func (m *MockLogger) Fatal(msg string, args ...interface{}) {}
func (m *MockLogger) With(args ...interface{}) log.Logger   { return m }
func (m *MockLogger) Sync() error                           { return nil }

func TestNewOrderProcessor(t *testing.T) {
	mockOrders := &MockOrdersRepository{}
	mockUsers := &MockUsersRepository{}
	mockAccrual := &MockAccrualClient{}
	mockTxManager := &MockTransactionManager{}
	mockLogger := &MockLogger{}

	config := Config{
		PollInterval:       1 * time.Second,
		BatchSize:          10,
		Concurrency:        5,
		NormalPollInterval: 5 * time.Second,
		MaxBackoff:         10 * time.Minute,
	}

	processor := NewOrderProcessor(mockOrders, mockUsers, mockAccrual, mockTxManager, config, mockLogger)

	if processor == nil {
		t.Error("expected non-nil processor")
	}

	if processor.config.PollInterval != config.PollInterval {
		t.Errorf("expected poll interval %v, got %v", config.PollInterval, processor.config.PollInterval)
	}

	if processor.config.BatchSize != config.BatchSize {
		t.Errorf("expected batch size %d, got %d", config.BatchSize, processor.config.BatchSize)
	}
}

func TestCalculateBackoff(t *testing.T) {
	config := Config{
		MaxBackoff: 10 * time.Minute,
	}

	processor := NewOrderProcessor(nil, nil, nil, nil, config, &MockLogger{})

	tests := []struct {
		attempts int
		expected time.Duration
	}{
		{1, 2 * time.Second},   // 2^1 = 2
		{2, 4 * time.Second},   // 2^2 = 4
		{3, 8 * time.Second},   // 2^3 = 8
		{4, 16 * time.Second},  // 2^4 = 16
		{5, 32 * time.Second},  // 2^5 = 32
		{10, 10 * time.Minute}, // 2^10 = 1024s > max
		{20, 10 * time.Minute}, // 2^20 >> max
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.attempts)), func(t *testing.T) {
			result := processor.calculateBackoff(tt.attempts)
			if result != tt.expected {
				t.Errorf("attempts=%d: expected %v, got %v", tt.attempts, tt.expected, result)
			}
		})
	}
}

func TestHandleResult_NotFound(t *testing.T) {
	updateCalled := false

	mockOrders := &MockOrdersRepository{
		UpdateAfterPollFunc: func(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error {
			updateCalled = true
			if orderID != 1 {
				t.Errorf("expected orderID 1, got %d", orderID)
			}
			if pollAttempts != 1 {
				t.Errorf("expected pollAttempts 1, got %d", pollAttempts)
			}
			if nextPollAt == nil {
				t.Error("expected nextPollAt to be set")
			}
			return nil
		},
	}

	config := Config{
		NormalPollInterval: 5 * time.Second,
	}

	processor := NewOrderProcessor(mockOrders, nil, nil, nil, config, &MockLogger{})

	order := &repository.Order{
		ID:           1,
		Number:       "123",
		PollAttempts: 0,
	}

	result := &accrual.AccrualResult{
		Found: false,
	}

	err := processor.handleResult(context.Background(), order, result, 100*time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !updateCalled {
		t.Error("expected UpdateAfterPoll to be called")
	}
}

func TestHandleResult_Processing(t *testing.T) {
	updateCalled := false

	mockOrders := &MockOrdersRepository{
		UpdateAfterPollFunc: func(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error {
			updateCalled = true
			if status != repository.OrderStatusProcessing {
				t.Errorf("expected status PROCESSING, got %s", status)
			}
			if nextPollAt == nil {
				t.Error("expected nextPollAt to be set")
			}
			return nil
		},
	}

	config := Config{
		NormalPollInterval: 5 * time.Second,
	}

	processor := NewOrderProcessor(mockOrders, nil, nil, nil, config, &MockLogger{})

	order := &repository.Order{
		ID:           1,
		Number:       "123",
		PollAttempts: 0,
	}

	result := &accrual.AccrualResult{
		Found:  true,
		Status: accrual.StatusProcessing,
	}

	err := processor.handleResult(context.Background(), order, result, 100*time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !updateCalled {
		t.Error("expected UpdateAfterPoll to be called")
	}
}

func TestHandleResult_Invalid(t *testing.T) {
	updateCalled := false

	mockOrders := &MockOrdersRepository{
		UpdateAfterPollFunc: func(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error {
			updateCalled = true
			if status != repository.OrderStatusInvalid {
				t.Errorf("expected status INVALID, got %s", status)
			}
			if processedAt == nil {
				t.Error("expected processedAt to be set")
			}
			if nextPollAt != nil {
				t.Error("expected nextPollAt to be nil for terminal status")
			}
			return nil
		},
	}

	processor := NewOrderProcessor(mockOrders, nil, nil, nil, Config{}, &MockLogger{})

	order := &repository.Order{
		ID:           1,
		Number:       "123",
		PollAttempts: 0,
	}

	result := &accrual.AccrualResult{
		Found:  true,
		Status: accrual.StatusInvalid,
	}

	err := processor.handleResult(context.Background(), order, result, 100*time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !updateCalled {
		t.Error("expected UpdateAfterPoll to be called")
	}
}

func TestHandleResult_ProcessedWithAccrual(t *testing.T) {
	accrualValue := decimal.NewFromFloat(500.00)
	finalizeCalled := false
	creditCalled := false

	mockOrders := &MockOrdersRepository{
		FinalizeOrderTxFunc: func(ctx context.Context, tx *sql.Tx, orderID int64, accrual decimal.Decimal, processedAt time.Time) (bool, error) {
			finalizeCalled = true
			if orderID != 1 {
				t.Errorf("expected orderID 1, got %d", orderID)
			}
			if !accrual.Equal(accrualValue) {
				t.Errorf("expected accrual %s, got %s", accrualValue, accrual)
			}
			return true, nil
		},
	}

	mockUsers := &MockUsersRepository{
		CreditBalanceTxFunc: func(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
			creditCalled = true
			if userID != 10 {
				t.Errorf("expected userID 10, got %d", userID)
			}
			if !amount.Equal(accrualValue) {
				t.Errorf("expected amount %s, got %s", accrualValue, amount)
			}
			return nil
		},
	}

	mockTxManager := &MockTransactionManager{
		WithTransactionFunc: func(ctx context.Context, fn func(tx *sql.Tx) error) error {
			return fn(nil)
		},
	}

	processor := NewOrderProcessor(mockOrders, mockUsers, nil, mockTxManager, Config{}, &MockLogger{})

	order := &repository.Order{
		ID:           1,
		UserID:       10,
		Number:       "123",
		PollAttempts: 0,
	}

	result := &accrual.AccrualResult{
		Found:   true,
		Status:  accrual.StatusProcessed,
		Accrual: decimal.NullDecimal{Decimal: accrualValue, Valid: true},
	}

	err := processor.handleResult(context.Background(), order, result, 100*time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !finalizeCalled {
		t.Error("expected FinalizeOrderTx to be called")
	}
	if !creditCalled {
		t.Error("expected CreditBalanceTx to be called")
	}
}

func TestHandleResult_ProcessedWithoutAccrual(t *testing.T) {
	updateCalled := false

	mockOrders := &MockOrdersRepository{
		UpdateAfterPollFunc: func(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error {
			updateCalled = true
			if status != repository.OrderStatusProcessed {
				t.Errorf("expected status PROCESSED, got %s", status)
			}
			if processedAt == nil {
				t.Error("expected processedAt to be set")
			}
			return nil
		},
	}

	processor := NewOrderProcessor(mockOrders, nil, nil, nil, Config{}, &MockLogger{})

	order := &repository.Order{
		ID:           1,
		Number:       "123",
		PollAttempts: 0,
	}

	result := &accrual.AccrualResult{
		Found:   true,
		Status:  accrual.StatusProcessed,
		Accrual: decimal.NullDecimal{Decimal: decimal.Zero, Valid: true},
	}

	err := processor.handleResult(context.Background(), order, result, 100*time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !updateCalled {
		t.Error("expected UpdateAfterPoll to be called")
	}
}

func TestHandleError_TooManyRequests(t *testing.T) {
	updateCalled := false
	retryAfter := 60 * time.Second

	mockOrders := &MockOrdersRepository{
		UpdateAfterPollFunc: func(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error {
			updateCalled = true
			if status != repository.OrderStatusProcessing {
				t.Errorf("expected status PROCESSING, got %s", status)
			}
			if lastError == nil || *lastError != "rate limited" {
				t.Errorf("expected lastError 'rate limited', got %v", lastError)
			}
			if nextPollAt == nil {
				t.Error("expected nextPollAt to be set")
			}
			return nil
		},
	}

	processor := NewOrderProcessor(mockOrders, nil, nil, nil, Config{}, &MockLogger{})

	order := &repository.Order{
		ID:           1,
		Number:       "123",
		PollAttempts: 0,
	}

	err := &accrual.ErrTooManyRequests{
		RetryAfter: retryAfter,
	}

	handleErr := processor.handleError(context.Background(), order, err, 100*time.Millisecond)
	if handleErr != nil {
		t.Errorf("unexpected error: %v", handleErr)
	}

	if !updateCalled {
		t.Error("expected UpdateAfterPoll to be called")
	}
}

func TestHandleError_BadResponse(t *testing.T) {
	updateCalled := false

	mockOrders := &MockOrdersRepository{
		UpdateAfterPollFunc: func(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error {
			updateCalled = true
			if lastError == nil {
				t.Error("expected lastError to be set")
			}
			if nextPollAt == nil {
				t.Error("expected nextPollAt to be set")
			}
			return nil
		},
	}

	processor := NewOrderProcessor(mockOrders, nil, nil, nil, Config{}, &MockLogger{})

	order := &repository.Order{
		ID:           1,
		Number:       "123",
		PollAttempts: 0,
	}

	err := &accrual.ErrBadResponse{
		Reason: "invalid JSON",
	}

	handleErr := processor.handleError(context.Background(), order, err, 100*time.Millisecond)
	if handleErr != nil {
		t.Errorf("unexpected error: %v", handleErr)
	}

	if !updateCalled {
		t.Error("expected UpdateAfterPoll to be called")
	}
}

func TestHandleError_ExternalUnavailable(t *testing.T) {
	updateCalled := false

	mockOrders := &MockOrdersRepository{
		UpdateAfterPollFunc: func(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error {
			updateCalled = true
			if lastError == nil {
				t.Error("expected lastError to be set")
			}
			if nextPollAt == nil {
				t.Error("expected nextPollAt to be set with backoff")
			}
			return nil
		},
	}

	config := Config{
		MaxBackoff: 10 * time.Minute,
	}

	processor := NewOrderProcessor(mockOrders, nil, nil, nil, config, &MockLogger{})

	order := &repository.Order{
		ID:           1,
		Number:       "123",
		PollAttempts: 2,
	}

	err := errors.New("network error")

	handleErr := processor.handleError(context.Background(), order, err, 100*time.Millisecond)
	if handleErr != nil {
		t.Errorf("unexpected error: %v", handleErr)
	}

	if !updateCalled {
		t.Error("expected UpdateAfterPoll to be called")
	}
}
