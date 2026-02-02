package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gophermart/internal/service"
)

type MockAuthService struct {
	RegisterFunc        func(ctx context.Context, login, password string) (string, error)
	LoginFunc           func(ctx context.Context, login, password string) (string, error)
	ValidateSessionFunc func(ctx context.Context, token string) (int64, error)
}

func (m *MockAuthService) Register(ctx context.Context, login, password string) (string, error) {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, login, password)
	}
	return "", errors.New("not implemented")
}

func (m *MockAuthService) Login(ctx context.Context, login, password string) (string, error) {
	if m.LoginFunc != nil {
		return m.LoginFunc(ctx, login, password)
	}
	return "", errors.New("not implemented")
}

func (m *MockAuthService) ValidateSession(ctx context.Context, token string) (int64, error) {
	if m.ValidateSessionFunc != nil {
		return m.ValidateSessionFunc(ctx, token)
	}
	return 0, errors.New("not implemented")
}

func TestUserHandler_Register(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		mockRegister   func(ctx context.Context, login, password string) (string, error)
		expectedStatus int
		checkCookie    bool
	}{
		{
			name: "successful registration",
			requestBody: authRequest{
				Login:    "testuser",
				Password: "password123",
			},
			mockRegister: func(ctx context.Context, login, password string) (string, error) {
				return "test-token-123", nil
			},
			expectedStatus: http.StatusOK,
			checkCookie:    true,
		},
		{
			name: "login already taken",
			requestBody: authRequest{
				Login:    "existinguser",
				Password: "password123",
			},
			mockRegister: func(ctx context.Context, login, password string) (string, error) {
				return "", service.ErrLoginTaken
			},
			expectedStatus: http.StatusConflict,
			checkCookie:    false,
		},
		{
			name: "empty login",
			requestBody: authRequest{
				Login:    "",
				Password: "password123",
			},
			mockRegister:   nil,
			expectedStatus: http.StatusBadRequest,
			checkCookie:    false,
		},
		{
			name: "empty password",
			requestBody: authRequest{
				Login:    "testuser",
				Password: "",
			},
			mockRegister:   nil,
			expectedStatus: http.StatusBadRequest,
			checkCookie:    false,
		},
		{
			name:           "invalid json",
			requestBody:    "invalid json",
			mockRegister:   nil,
			expectedStatus: http.StatusBadRequest,
			checkCookie:    false,
		},
		{
			name: "internal error",
			requestBody: authRequest{
				Login:    "testuser",
				Password: "password123",
			},
			mockRegister: func(ctx context.Context, login, password string) (string, error) {
				return "", errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
			checkCookie:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuth := &MockAuthService{
				RegisterFunc: tt.mockRegister,
			}
			handler := NewUserHandler(mockAuth)

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

			req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Register(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkCookie {
				cookies := w.Result().Cookies()
				found := false
				for _, cookie := range cookies {
					if cookie.Name == "Authorization" {
						found = true
						if cookie.Value == "" {
							t.Error("expected cookie value to be non-empty")
						}
					}
				}
				if !found {
					t.Error("expected Authorization cookie to be set")
				}
			}
		})
	}
}

func TestUserHandler_Login(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		mockLogin      func(ctx context.Context, login, password string) (string, error)
		expectedStatus int
		checkCookie    bool
	}{
		{
			name: "successful login",
			requestBody: authRequest{
				Login:    "testuser",
				Password: "password123",
			},
			mockLogin: func(ctx context.Context, login, password string) (string, error) {
				return "test-token-456", nil
			},
			expectedStatus: http.StatusOK,
			checkCookie:    true,
		},
		{
			name: "invalid credentials",
			requestBody: authRequest{
				Login:    "testuser",
				Password: "wrongpassword",
			},
			mockLogin: func(ctx context.Context, login, password string) (string, error) {
				return "", service.ErrInvalidCredentials
			},
			expectedStatus: http.StatusUnauthorized,
			checkCookie:    false,
		},
		{
			name: "empty login",
			requestBody: authRequest{
				Login:    "",
				Password: "password123",
			},
			mockLogin:      nil,
			expectedStatus: http.StatusBadRequest,
			checkCookie:    false,
		},
		{
			name: "empty password",
			requestBody: authRequest{
				Login:    "testuser",
				Password: "",
			},
			mockLogin:      nil,
			expectedStatus: http.StatusBadRequest,
			checkCookie:    false,
		},
		{
			name:           "invalid json",
			requestBody:    "not a json",
			mockLogin:      nil,
			expectedStatus: http.StatusBadRequest,
			checkCookie:    false,
		},
		{
			name: "internal error",
			requestBody: authRequest{
				Login:    "testuser",
				Password: "password123",
			},
			mockLogin: func(ctx context.Context, login, password string) (string, error) {
				return "", errors.New("database error")
			},
			expectedStatus: http.StatusInternalServerError,
			checkCookie:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuth := &MockAuthService{
				LoginFunc: tt.mockLogin,
			}
			handler := NewUserHandler(mockAuth)

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

			req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Login(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.checkCookie {
				cookies := w.Result().Cookies()
				found := false
				for _, cookie := range cookies {
					if cookie.Name == "Authorization" {
						found = true
						if cookie.Value == "" {
							t.Error("expected cookie value to be non-empty")
						}
					}
				}
				if !found {
					t.Error("expected Authorization cookie to be set")
				}
			}
		})
	}
}
