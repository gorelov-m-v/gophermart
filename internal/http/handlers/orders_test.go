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
	"gophermart/internal/http/middleware"
	"gophermart/internal/repository"
	"gophermart/internal/service"
)

type MockOrdersService struct {
	SubmitOrderFunc func(ctx context.Context, userID int64, number string) (service.SubmitResult, error)
	ListOrdersFunc  func(ctx context.Context, userID int64) ([]*repository.Order, error)
}

func (m *MockOrdersService) SubmitOrder(ctx context.Context, userID int64, number string) (service.SubmitResult, error) {
	if m.SubmitOrderFunc != nil {
		return m.SubmitOrderFunc(ctx, userID, number)
	}
	return 0, errors.New("not implemented")
}

func (m *MockOrdersService) ListOrders(ctx context.Context, userID int64) ([]*repository.Order, error) {
	if m.ListOrdersFunc != nil {
		return m.ListOrdersFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func TestOrdersHandler_Upload(t *testing.T) {
	tests := []struct {
		name           string
		orderNumber    string
		contentType    string
		userID         int64
		mockSubmit     func(ctx context.Context, userID int64, number string) (service.SubmitResult, error)
		expectedStatus int
	}{
		{
			name:        "successful upload - new order",
			orderNumber: "4561261212345467",
			contentType: "text/plain",
			userID:      1,
			mockSubmit: func(ctx context.Context, userID int64, number string) (service.SubmitResult, error) {
				return service.SubmitResultCreated, nil
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:        "already uploaded by same user",
			orderNumber: "4561261212345467",
			contentType: "text/plain",
			userID:      1,
			mockSubmit: func(ctx context.Context, userID int64, number string) (service.SubmitResult, error) {
				return service.SubmitResultAlreadyOwned, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:        "uploaded by another user",
			orderNumber: "4561261212345467",
			contentType: "text/plain",
			userID:      1,
			mockSubmit: func(ctx context.Context, userID int64, number string) (service.SubmitResult, error) {
				return service.SubmitResultOwnedByAnother, nil
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "invalid content type",
			orderNumber:    "4561261212345467",
			contentType:    "application/json",
			userID:         1,
			mockSubmit:     nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid order number format",
			orderNumber:    "invalid",
			contentType:    "text/plain",
			userID:         1,
			mockSubmit:     nil,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "empty order number",
			orderNumber:    "",
			contentType:    "text/plain",
			userID:         1,
			mockSubmit:     nil,
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:        "internal error",
			orderNumber: "4561261212345467",
			contentType: "text/plain",
			userID:      1,
			mockSubmit: func(ctx context.Context, userID int64, number string) (service.SubmitResult, error) {
				return 0, errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockOrdersService{
				SubmitOrderFunc: tt.mockSubmit,
			}
			handler := NewOrdersHandler(mockService)

			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString(tt.orderNumber))
			req.Header.Set("Content-Type", tt.contentType)

			ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.Upload(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestOrdersHandler_Upload_Unauthorized(t *testing.T) {
	mockService := &MockOrdersService{}
	handler := NewOrdersHandler(mockService)

	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("4561261212345467"))
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()

	handler.Upload(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestOrdersHandler_List(t *testing.T) {
	now := time.Now()
	accrual := decimal.NewFromFloat(500.00)

	tests := []struct {
		name           string
		userID         int64
		mockList       func(ctx context.Context, userID int64) ([]*repository.Order, error)
		expectedStatus int
		checkJSON      bool
	}{
		{
			name:   "successful list with orders",
			userID: 1,
			mockList: func(ctx context.Context, userID int64) ([]*repository.Order, error) {
				return []*repository.Order{
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
						UploadedAt: now,
					},
				}, nil
			},
			expectedStatus: http.StatusOK,
			checkJSON:      true,
		},
		{
			name:   "no orders",
			userID: 1,
			mockList: func(ctx context.Context, userID int64) ([]*repository.Order, error) {
				return []*repository.Order{}, nil
			},
			expectedStatus: http.StatusNoContent,
			checkJSON:      false,
		},
		{
			name:   "internal error",
			userID: 1,
			mockList: func(ctx context.Context, userID int64) ([]*repository.Order, error) {
				return nil, errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
			checkJSON:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &MockOrdersService{
				ListOrdersFunc: tt.mockList,
			}
			handler := NewOrdersHandler(mockService)

			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)

			ctx := context.WithValue(req.Context(), middleware.UserIDKey, tt.userID)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.List(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkJSON {
				var response []map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Errorf("failed to unmarshal response: %v", err)
				}
				if len(response) == 0 {
					t.Error("expected non-empty response")
				}
			}
		})
	}
}

func TestOrdersHandler_List_Unauthorized(t *testing.T) {
	mockService := &MockOrdersService{}
	handler := NewOrdersHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)

	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
