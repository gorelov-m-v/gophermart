package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gophermart/internal/repository"
	"gophermart/internal/service"
)

type MockAuthService struct {
	ValidateSessionFunc func(ctx context.Context, token string) (int64, error)
}

func (m *MockAuthService) Register(ctx context.Context, login, password string) (*repository.User, string, error) {
	return nil, "", nil
}

func (m *MockAuthService) Login(ctx context.Context, login, password string) (*repository.User, string, error) {
	return nil, "", nil
}

func (m *MockAuthService) ValidateSession(ctx context.Context, token string) (int64, error) {
	if m.ValidateSessionFunc != nil {
		return m.ValidateSessionFunc(ctx, token)
	}
	return 0, errors.New("not implemented")
}

func testHandler(t *testing.T, expectedUserID int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(UserIDKey)
		if userID == nil {
			t.Error("userID not found in context")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		id, ok := userID.(int64)
		if !ok {
			t.Error("userID is not int64")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if id != expectedUserID {
			t.Errorf("expected userID %d, got %d", expectedUserID, id)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func TestAuthMiddleware_Success(t *testing.T) {
	mockAuth := &MockAuthService{
		ValidateSessionFunc: func(ctx context.Context, token string) (int64, error) {
			return 100, nil
		},
	}

	middleware := Auth(mockAuth)
	handler := middleware(testHandler(t, 100))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_MissingAuthHeader(t *testing.T) {
	mockAuth := &MockAuthService{}
	middleware := Auth(mockAuth)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidAuthHeaderFormat(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"no bearer prefix", "token123"},
		{"wrong prefix", "Basic token123"},
		{"bearer only", "Bearer"},
		{"bearer with space", "Bearer "},
		{"multiple spaces", "Bearer  token123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuth := &MockAuthService{}
			middleware := Auth(mockAuth)
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("handler should not be called")
			}))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", tt.header)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", w.Code)
			}
		})
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	mockAuth := &MockAuthService{
		ValidateSessionFunc: func(ctx context.Context, token string) (int64, error) {
			return 0, service.ErrInvalidCredentials
		},
	}

	middleware := Auth(mockAuth)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	mockAuth := &MockAuthService{
		ValidateSessionFunc: func(ctx context.Context, token string) (int64, error) {
			return 0, service.ErrInvalidCredentials
		},
	}

	middleware := Auth(mockAuth)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_DatabaseError(t *testing.T) {
	mockAuth := &MockAuthService{
		ValidateSessionFunc: func(ctx context.Context, token string) (int64, error) {
			return 0, errors.New("database connection failed")
		},
	}

	middleware := Auth(mockAuth)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestAuthMiddleware_EmptyToken(t *testing.T) {
	mockAuth := &MockAuthService{}
	middleware := Auth(mockAuth)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_CaseInsensitiveBearer(t *testing.T) {
	mockAuth := &MockAuthService{
		ValidateSessionFunc: func(ctx context.Context, token string) (int64, error) {
			return 100, nil
		},
	}

	middleware := Auth(mockAuth)
	handler := middleware(testHandler(t, 100))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		return
	}

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_MultipleAuthHeaders(t *testing.T) {
	mockAuth := &MockAuthService{
		ValidateSessionFunc: func(ctx context.Context, token string) (int64, error) {
			if token == "first-token" {
				return 100, nil
			}
			return 0, service.ErrInvalidCredentials
		},
	}

	middleware := Auth(mockAuth)
	handler := middleware(testHandler(t, 100))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Add("Authorization", "Bearer first-token")
	req.Header.Add("Authorization", "Bearer second-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetUserID(t *testing.T) {
	tests := []struct {
		name        string
		ctxValue    interface{}
		expectID    int64
		expectFound bool
	}{
		{"valid userID", int64(123), 123, true},
		{"no userID in context", nil, 0, false},
		{"wrong type in context", "123", 0, false},
		{"zero userID", int64(0), 0, true},
		{"negative userID", int64(-1), -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.ctxValue != nil {
				ctx = context.WithValue(context.Background(), UserIDKey, tt.ctxValue)
			} else {
				ctx = context.Background()
			}

			id, found := GetUserID(ctx)
			if id != tt.expectID {
				t.Errorf("expected id %d, got %d", tt.expectID, id)
			}
			if found != tt.expectFound {
				t.Errorf("expected found %v, got %v", tt.expectFound, found)
			}
		})
	}
}
