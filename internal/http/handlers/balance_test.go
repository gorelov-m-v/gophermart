package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gophermart/internal/database"
	"gophermart/internal/http/dto"
	"gophermart/internal/http/middleware"
	"gophermart/internal/repository"
	"gophermart/internal/service"
)

type MockBalanceService struct {
	GetBalanceFunc      func(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error)
	WithdrawFunc        func(ctx context.Context, userID int64, orderNumber string, sum decimal.Decimal) (service.WithdrawResult, error)
	ListWithdrawalsFunc func(ctx context.Context, userID int64) ([]*repository.Withdrawal, error)
}

func (m *MockBalanceService) GetBalance(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
	if m.GetBalanceFunc != nil {
		return m.GetBalanceFunc(ctx, userID)
	}
	return decimal.Zero, decimal.Zero, errors.New("not implemented")
}

func (m *MockBalanceService) Withdraw(ctx context.Context, userID int64, orderNumber string, sum decimal.Decimal) (service.WithdrawResult, error) {
	if m.WithdrawFunc != nil {
		return m.WithdrawFunc(ctx, userID, orderNumber, sum)
	}
	return 0, errors.New("not implemented")
}

func (m *MockBalanceService) ListWithdrawals(ctx context.Context, userID int64) ([]*repository.Withdrawal, error) {
	if m.ListWithdrawalsFunc != nil {
		return m.ListWithdrawalsFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func TestBalanceHandler_Get(t *testing.T) {
	tests := []struct {
		name           string
		userID         int64
		mockGetBalance func(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error)
		expectedStatus int
		checkJSON      bool
	}{
		{
			name:   "successful get balance",
			userID: 1,
			mockGetBalance: func(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
				return decimal.NewFromFloat(1000.50), decimal.NewFromFloat(250.25), nil
			},
			expectedStatus: http.StatusOK,
			checkJSON:      true,
		},
		{
			name:   "user not found",
			userID: 999,
			mockGetBalance: func(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
				return decimal.Zero, decimal.Zero, database.ErrNotFound
			},
			expectedStatus: http.StatusNotFound,
			checkJSON:      false,
		},
		{
			name:   "internal error",
			userID: 1,
			mockGetBalance: func(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
				return decimal.Zero, decimal.Zero, errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
			checkJSON:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockBalanceService{
				GetBalanceFunc: tt.mockGetBalance,
			}
			handler := NewBalanceHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)

			ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.Get(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkJSON {
				var response dto.BalanceResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("failed to unmarshal response: %v", err)
				}
				expectedCurrent := decimal.NewFromFloat(1000.50)
				expectedWithdrawn := decimal.NewFromFloat(250.25)
				if !response.Current.Equal(expectedCurrent) || !response.Withdrawn.Equal(expectedWithdrawn) {
					t.Errorf("unexpected balance values: current=%s, withdrawn=%s", response.Current, response.Withdrawn)
				}
			}
		})
	}
}

func TestBalanceHandler_Get_Unauthorized(t *testing.T) {
	mockService := &MockBalanceService{}
	handler := NewBalanceHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)

	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestBalanceHandler_Withdraw(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		contentType    string
		userID         int64
		mockWithdraw   func(ctx context.Context, userID int64, orderNumber string, sum decimal.Decimal) (service.WithdrawResult, error)
		expectedStatus int
	}{
		{
			name: "successful withdraw",
			requestBody: dto.WithdrawRequest{
				Order: "4561261212345467",
				Sum:   decimal.NewFromFloat(100.50),
			},
			contentType: "application/json",
			userID:      1,
			mockWithdraw: func(ctx context.Context, userID int64, orderNumber string, sum decimal.Decimal) (service.WithdrawResult, error) {
				expectedSum := decimal.NewFromFloat(100.50)
				if !sum.Equal(expectedSum) {
					return 0, errors.New("expected sum 100.50")
				}
				return service.WithdrawOk, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "insufficient funds",
			requestBody: dto.WithdrawRequest{
				Order: "4561261212345467",
				Sum:   decimal.NewFromFloat(1000.00),
			},
			contentType: "application/json",
			userID:      1,
			mockWithdraw: func(ctx context.Context, userID int64, orderNumber string, sum decimal.Decimal) (service.WithdrawResult, error) {
				return service.WithdrawInsufficientFunds, nil
			},
			expectedStatus: http.StatusPaymentRequired,
		},
		{
			name: "already processed (idempotent)",
			requestBody: dto.WithdrawRequest{
				Order: "4561261212345467",
				Sum:   decimal.NewFromFloat(100.00),
			},
			contentType: "application/json",
			userID:      1,
			mockWithdraw: func(ctx context.Context, userID int64, orderNumber string, sum decimal.Decimal) (service.WithdrawResult, error) {
				return service.WithdrawAlreadyProcessed, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "invalid content type",
			requestBody: dto.WithdrawRequest{
				Order: "4561261212345467",
				Sum:   decimal.NewFromFloat(100.00),
			},
			contentType:    "text/plain",
			userID:         1,
			mockWithdraw:   nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid json",
			requestBody:    "invalid json",
			contentType:    "application/json",
			userID:         1,
			mockWithdraw:   nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid order number",
			requestBody: dto.WithdrawRequest{
				Order: "invalid",
				Sum:   decimal.NewFromFloat(100.00),
			},
			contentType:    "application/json",
			userID:         1,
			mockWithdraw:   nil,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "negative sum",
			requestBody: dto.WithdrawRequest{
				Order: "4561261212345467",
				Sum:   decimal.NewFromFloat(-100.00),
			},
			contentType:    "application/json",
			userID:         1,
			mockWithdraw:   nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "zero sum",
			requestBody: dto.WithdrawRequest{
				Order: "4561261212345467",
				Sum:   decimal.Zero,
			},
			contentType:    "application/json",
			userID:         1,
			mockWithdraw:   nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "user not found",
			requestBody: dto.WithdrawRequest{
				Order: "4561261212345467",
				Sum:   decimal.NewFromFloat(100.00),
			},
			contentType: "application/json",
			userID:      999,
			mockWithdraw: func(ctx context.Context, userID int64, orderNumber string, sum decimal.Decimal) (service.WithdrawResult, error) {
				return 0, database.ErrNotFound
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "internal error",
			requestBody: dto.WithdrawRequest{
				Order: "4561261212345467",
				Sum:   decimal.NewFromFloat(100.00),
			},
			contentType: "application/json",
			userID:      1,
			mockWithdraw: func(ctx context.Context, userID int64, orderNumber string, sum decimal.Decimal) (service.WithdrawResult, error) {
				return 0, errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockBalanceService{
				WithdrawFunc: tt.mockWithdraw,
			}
			handler := NewBalanceHandler(mockService)

			var body []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatal(err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
			req.Header.Set("Content-Type", tt.contentType)

			ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.Withdraw(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestBalanceHandler_Withdraw_Unauthorized(t *testing.T) {
	mockService := &MockBalanceService{}
	handler := NewBalanceHandler(mockService)

	body, _ := json.Marshal(dto.WithdrawRequest{
		Order: "4561261212345467",
		Sum:   decimal.NewFromFloat(100.00),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	handler.Withdraw(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestBalanceHandler_Withdrawals(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name               string
		userID             int64
		mockListWithdrawal func(ctx context.Context, userID int64) ([]*repository.Withdrawal, error)
		expectedStatus     int
		checkJSON          bool
	}{
		{
			name:   "successful list with withdrawals",
			userID: 1,
			mockListWithdrawal: func(ctx context.Context, userID int64) ([]*repository.Withdrawal, error) {
				return []*repository.Withdrawal{
					{
						ID:          1,
						UserID:      1,
						OrderNumber: "4561261212345467",
						Sum:         decimal.NewFromFloat(100.50),
						ProcessedAt: now,
					},
					{
						ID:          2,
						UserID:      1,
						OrderNumber: "4561261212345468",
						Sum:         decimal.NewFromFloat(200.00),
						ProcessedAt: now.Add(-time.Hour),
					},
				}, nil
			},
			expectedStatus: http.StatusOK,
			checkJSON:      true,
		},
		{
			name:   "no withdrawals",
			userID: 1,
			mockListWithdrawal: func(ctx context.Context, userID int64) ([]*repository.Withdrawal, error) {
				return []*repository.Withdrawal{}, nil
			},
			expectedStatus: http.StatusNoContent,
			checkJSON:      false,
		},
		{
			name:   "internal error",
			userID: 1,
			mockListWithdrawal: func(ctx context.Context, userID int64) ([]*repository.Withdrawal, error) {
				return nil, errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
			checkJSON:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockBalanceService{
				ListWithdrawalsFunc: tt.mockListWithdrawal,
			}
			handler := NewBalanceHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)

			ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.Withdrawals(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkJSON {
				var response []dto.WithdrawalResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("failed to unmarshal response: %v", err)
				}
				if len(response) == 0 {
					t.Error("expected non-empty response")
				}
				expectedSum := decimal.NewFromFloat(100.50)
				if !response[0].Sum.Equal(expectedSum) {
					t.Errorf("expected Sum 100.50, got %s", response[0].Sum)
				}
			}
		})
	}
}

func TestBalanceHandler_Withdrawals_Unauthorized(t *testing.T) {
	mockService := &MockBalanceService{}
	handler := NewBalanceHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)

	w := httptest.NewRecorder()

	handler.Withdrawals(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
