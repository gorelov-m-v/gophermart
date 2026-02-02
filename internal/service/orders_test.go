package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gophermart/internal/database"
	"gophermart/internal/repository"
)

type MockOrdersRepository struct {
	CreateFunc      func(ctx context.Context, userID int64, number string) (*repository.Order, error)
	GetByNumberFunc func(ctx context.Context, number string) (*repository.Order, error)
	ListByUserFunc  func(ctx context.Context, userID int64) ([]*repository.Order, error)
}

func (m *MockOrdersRepository) Create(ctx context.Context, userID int64, number string) (*repository.Order, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, userID, number)
	}
	return nil, errors.New("not implemented")
}

func (m *MockOrdersRepository) GetByNumber(ctx context.Context, number string) (*repository.Order, error) {
	if m.GetByNumberFunc != nil {
		return m.GetByNumberFunc(ctx, number)
	}
	return nil, errors.New("not implemented")
}

func (m *MockOrdersRepository) ListByUser(ctx context.Context, userID int64) ([]*repository.Order, error) {
	if m.ListByUserFunc != nil {
		return m.ListByUserFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *MockOrdersRepository) UpdateStatus(ctx context.Context, number, status string) error {
	return errors.New("not implemented")
}

func (m *MockOrdersRepository) PickDueOrdersForUpdate(ctx context.Context, tx *sql.Tx, limit int, now time.Time) ([]*repository.Order, error) {
	return nil, errors.New("not implemented")
}

func (m *MockOrdersRepository) UpdatePickedOrders(ctx context.Context, tx *sql.Tx, orderIDs []int64, nextPollAt time.Time) error {
	return errors.New("not implemented")
}

func (m *MockOrdersRepository) UpdateAfterPoll(ctx context.Context, orderID int64, status string, accrual decimal.NullDecimal, processedAt *time.Time, nextPollAt *time.Time, pollAttempts int, lastError *string) error {
	return errors.New("not implemented")
}

func (m *MockOrdersRepository) FinalizeOrderTx(ctx context.Context, tx *sql.Tx, orderID int64, accrual decimal.Decimal, processedAt time.Time) (bool, error) {
	return false, errors.New("not implemented")
}

func TestOrdersService_SubmitOrder_Created(t *testing.T) {
	mockOrders := &MockOrdersRepository{
		CreateFunc: func(ctx context.Context, userID int64, number string) (*repository.Order, error) {
			if userID != 1 {
				t.Errorf("expected userID 1, got %d", userID)
			}
			if number != "4561261212345467" {
				t.Errorf("expected number 4561261212345467, got %s", number)
			}
			return &repository.Order{
				ID:         10,
				UserID:     userID,
				Number:     number,
				Status:     repository.OrderStatusNew,
				UploadedAt: time.Now(),
			}, nil
		},
	}

	service := NewOrdersService(mockOrders)
	ctx := context.Background()

	result, err := service.SubmitOrder(ctx, 1, "4561261212345467")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != SubmitResultCreated {
		t.Errorf("expected SubmitResultCreated, got %v", result)
	}
}

func TestOrdersService_SubmitOrder_AlreadyOwned(t *testing.T) {
	mockOrders := &MockOrdersRepository{
		CreateFunc: func(ctx context.Context, userID int64, number string) (*repository.Order, error) {
			return nil, database.ErrAlreadyExists
		},
		GetByNumberFunc: func(ctx context.Context, number string) (*repository.Order, error) {
			return &repository.Order{
				ID:         10,
				UserID:     1,
				Number:     number,
				Status:     repository.OrderStatusNew,
				UploadedAt: time.Now(),
			}, nil
		},
	}

	service := NewOrdersService(mockOrders)
	ctx := context.Background()

	result, err := service.SubmitOrder(ctx, 1, "4561261212345467")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != SubmitResultAlreadyOwned {
		t.Errorf("expected SubmitResultAlreadyOwned, got %v", result)
	}
}

func TestOrdersService_SubmitOrder_OwnedByAnother(t *testing.T) {
	mockOrders := &MockOrdersRepository{
		CreateFunc: func(ctx context.Context, userID int64, number string) (*repository.Order, error) {
			return nil, database.ErrAlreadyExists
		},
		GetByNumberFunc: func(ctx context.Context, number string) (*repository.Order, error) {
			return &repository.Order{
				ID:         10,
				UserID:     2,
				Number:     number,
				Status:     repository.OrderStatusNew,
				UploadedAt: time.Now(),
			}, nil
		},
	}

	service := NewOrdersService(mockOrders)
	ctx := context.Background()

	result, err := service.SubmitOrder(ctx, 1, "4561261212345467")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != SubmitResultOwnedByAnother {
		t.Errorf("expected SubmitResultOwnedByAnother, got %v", result)
	}
}

func TestOrdersService_SubmitOrder_CreateError(t *testing.T) {
	mockOrders := &MockOrdersRepository{
		CreateFunc: func(ctx context.Context, userID int64, number string) (*repository.Order, error) {
			return nil, errors.New("database error")
		},
	}

	service := NewOrdersService(mockOrders)
	ctx := context.Background()

	_, err := service.SubmitOrder(ctx, 1, "4561261212345467")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestOrdersService_SubmitOrder_GetByNumberError(t *testing.T) {
	mockOrders := &MockOrdersRepository{
		CreateFunc: func(ctx context.Context, userID int64, number string) (*repository.Order, error) {
			return nil, database.ErrAlreadyExists
		},
		GetByNumberFunc: func(ctx context.Context, number string) (*repository.Order, error) {
			return nil, errors.New("database error")
		},
	}

	service := NewOrdersService(mockOrders)
	ctx := context.Background()

	_, err := service.SubmitOrder(ctx, 1, "4561261212345467")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestOrdersService_ListOrders_Success(t *testing.T) {
	now := time.Now()
	accrual := decimal.NewFromFloat(500.00)

	expectedOrders := []*repository.Order{
		{
			ID:         1,
			UserID:     1,
			Number:     "4561261212345467",
			Status:     repository.OrderStatusProcessed,
			Accrual:    decimal.NullDecimal{Decimal: accrual, Valid: true},
			UploadedAt: now,
		},
		{
			ID:         2,
			UserID:     1,
			Number:     "4561261212345468",
			Status:     repository.OrderStatusProcessing,
			Accrual:    decimal.NullDecimal{Valid: false},
			UploadedAt: now.Add(-time.Hour),
		},
	}

	mockOrders := &MockOrdersRepository{
		ListByUserFunc: func(ctx context.Context, userID int64) ([]*repository.Order, error) {
			if userID != 1 {
				t.Errorf("expected userID 1, got %d", userID)
			}
			return expectedOrders, nil
		},
	}

	service := NewOrdersService(mockOrders)
	ctx := context.Background()

	orders, err := service.ListOrders(ctx, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(orders) != 2 {
		t.Errorf("expected 2 orders, got %d", len(orders))
	}

	if orders[0].Number != "4561261212345467" {
		t.Errorf("expected order number 4561261212345467, got %s", orders[0].Number)
	}

	if orders[0].Status != repository.OrderStatusProcessed {
		t.Errorf("expected status PROCESSED, got %s", orders[0].Status)
	}

	if !orders[0].Accrual.Valid || !orders[0].Accrual.Decimal.Equal(accrual) {
		t.Errorf("expected accrual 500.00, got %v", orders[0].Accrual)
	}
}

func TestOrdersService_ListOrders_Empty(t *testing.T) {
	mockOrders := &MockOrdersRepository{
		ListByUserFunc: func(ctx context.Context, userID int64) ([]*repository.Order, error) {
			return []*repository.Order{}, nil
		},
	}

	service := NewOrdersService(mockOrders)
	ctx := context.Background()

	orders, err := service.ListOrders(ctx, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(orders) != 0 {
		t.Errorf("expected 0 orders, got %d", len(orders))
	}
}

func TestOrdersService_ListOrders_Error(t *testing.T) {
	mockOrders := &MockOrdersRepository{
		ListByUserFunc: func(ctx context.Context, userID int64) ([]*repository.Order, error) {
			return nil, errors.New("database error")
		},
	}

	service := NewOrdersService(mockOrders)
	ctx := context.Background()

	_, err := service.ListOrders(ctx, 1)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestOrdersService_ListOrders_MultipleStatuses(t *testing.T) {
	now := time.Now()
	accrual1 := decimal.NewFromFloat(100.00)
	accrual2 := decimal.NewFromFloat(200.00)

	expectedOrders := []*repository.Order{
		{
			ID:         1,
			UserID:     1,
			Number:     "1111111111111111",
			Status:     repository.OrderStatusNew,
			Accrual:    decimal.NullDecimal{Valid: false},
			UploadedAt: now,
		},
		{
			ID:         2,
			UserID:     1,
			Number:     "2222222222222222",
			Status:     repository.OrderStatusProcessing,
			Accrual:    decimal.NullDecimal{Valid: false},
			UploadedAt: now.Add(-time.Hour),
		},
		{
			ID:         3,
			UserID:     1,
			Number:     "3333333333333333",
			Status:     repository.OrderStatusProcessed,
			Accrual:    decimal.NullDecimal{Decimal: accrual1, Valid: true},
			UploadedAt: now.Add(-2 * time.Hour),
		},
		{
			ID:         4,
			UserID:     1,
			Number:     "4444444444444444",
			Status:     repository.OrderStatusInvalid,
			Accrual:    decimal.NullDecimal{Decimal: accrual2, Valid: true},
			UploadedAt: now.Add(-3 * time.Hour),
		},
	}

	mockOrders := &MockOrdersRepository{
		ListByUserFunc: func(ctx context.Context, userID int64) ([]*repository.Order, error) {
			return expectedOrders, nil
		},
	}

	service := NewOrdersService(mockOrders)
	ctx := context.Background()

	orders, err := service.ListOrders(ctx, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(orders) != 4 {
		t.Errorf("expected 4 orders, got %d", len(orders))
	}

	statuses := make(map[string]bool)
	for _, order := range orders {
		statuses[order.Status] = true
	}

	expectedStatuses := []string{
		repository.OrderStatusNew,
		repository.OrderStatusProcessing,
		repository.OrderStatusProcessed,
		repository.OrderStatusInvalid,
	}

	for _, status := range expectedStatuses {
		if !statuses[status] {
			t.Errorf("expected to find status %s in results", status)
		}
	}
}

func TestOrdersService_SubmitOrder_ConflictResolution(t *testing.T) {
	tests := []struct {
		name           string
		userID         int64
		orderOwnerID   int64
		expectedResult SubmitResult
	}{
		{
			name:           "same user",
			userID:         1,
			orderOwnerID:   1,
			expectedResult: SubmitResultAlreadyOwned,
		},
		{
			name:           "different user",
			userID:         1,
			orderOwnerID:   2,
			expectedResult: SubmitResultOwnedByAnother,
		},
		{
			name:           "another different user",
			userID:         5,
			orderOwnerID:   3,
			expectedResult: SubmitResultOwnedByAnother,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrders := &MockOrdersRepository{
				CreateFunc: func(ctx context.Context, userID int64, number string) (*repository.Order, error) {
					return nil, database.ErrAlreadyExists
				},
				GetByNumberFunc: func(ctx context.Context, number string) (*repository.Order, error) {
					return &repository.Order{
						ID:         10,
						UserID:     tt.orderOwnerID,
						Number:     number,
						Status:     repository.OrderStatusNew,
						UploadedAt: time.Now(),
					}, nil
				},
			}

			service := NewOrdersService(mockOrders)
			ctx := context.Background()

			result, err := service.SubmitOrder(ctx, tt.userID, "4561261212345467")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if result != tt.expectedResult {
				t.Errorf("expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}
