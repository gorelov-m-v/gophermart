package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gophermart/internal/log"
)

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, args ...any) {}
func (m *mockLogger) Info(msg string, args ...any)  {}
func (m *mockLogger) Warn(msg string, args ...any)  {}
func (m *mockLogger) Error(msg string, args ...any) {}
func (m *mockLogger) Fatal(msg string, args ...any) {}
func (m *mockLogger) With(args ...any) log.Logger   { return m }
func (m *mockLogger) Sync() error                   { return nil }

func TestNewClient(t *testing.T) {
	cfg := Config{
		BaseURL:        "http://localhost:8080",
		RequestTimeout: 5 * time.Second,
		Logger:         &mockLogger{},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClient_EmptyBaseURL(t *testing.T) {
	cfg := Config{}
	client, err := NewClient(cfg)

	if err == nil {
		t.Error("expected error for empty base URL")
	}
	if client != nil {
		t.Error("expected nil client")
	}
}

func TestNewClient_InvalidBaseURL(t *testing.T) {
	cfg := Config{
		BaseURL: "://invalid",
	}
	client, err := NewClient(cfg)

	if err == nil {
		t.Error("expected error for invalid base URL")
	}
	if client != nil {
		t.Error("expected nil client")
	}
}

func TestGetOrderInfo_Success_Processed(t *testing.T) {
	accrual := decimal.NewFromFloat(500.5)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/orders/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := AccrualResponse{
			Order:   "12345",
			Status:  "PROCESSED",
			Accrual: &accrual,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Found {
		t.Error("expected order to be found")
	}
	if result.Status != StatusProcessed {
		t.Errorf("expected status PROCESSED, got %s", result.Status)
	}
	if !result.Accrual.Valid {
		t.Fatal("expected accrual to be set")
	}
	if !result.Accrual.Decimal.Equal(accrual) {
		t.Errorf("expected accrual %s, got %s", accrual, result.Accrual.Decimal)
	}
}

func TestGetOrderInfo_Success_Processing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := AccrualResponse{
			Order:  "12345",
			Status: "PROCESSING",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Found {
		t.Error("expected order to be found")
	}
	if result.Status != StatusProcessing {
		t.Errorf("expected status PROCESSING, got %s", result.Status)
	}
	if result.Accrual.Valid {
		t.Error("expected no accrual for PROCESSING status")
	}
}

func TestGetOrderInfo_Success_Registered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := AccrualResponse{
			Order:  "12345",
			Status: "REGISTERED",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Found {
		t.Error("expected order to be found")
	}
	if result.Status != StatusRegistered {
		t.Errorf("expected status REGISTERED, got %s", result.Status)
	}
}

func TestGetOrderInfo_Success_Invalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := AccrualResponse{
			Order:  "12345",
			Status: "INVALID",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Found {
		t.Error("expected order to be found")
	}
	if result.Status != StatusInvalid {
		t.Errorf("expected status INVALID, got %s", result.Status)
	}
}

func TestGetOrderInfo_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "99999")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Found {
		t.Error("expected order not to be found")
	}
}

func TestGetOrderInfo_TooManyRequests(t *testing.T) {
	retryAfter := 60
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}, MaxAttempts: 1}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if result != nil {
		t.Error("expected nil result")
	}

	var errTooMany *ErrTooManyRequests
	if !errors.As(err, &errTooMany) {
		t.Fatalf("expected ErrTooManyRequests, got %T: %v", err, err)
	}

	expectedDuration := time.Duration(retryAfter) * time.Second
	if errTooMany.RetryAfter != expectedDuration {
		t.Errorf("expected retry after %v, got %v", expectedDuration, errTooMany.RetryAfter)
	}
}

func TestGetOrderInfo_TooManyRequests_NoRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}, MaxAttempts: 1}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if result != nil {
		t.Error("expected nil result")
	}

	var errTooMany *ErrTooManyRequests
	if !errors.As(err, &errTooMany) {
		t.Fatalf("expected ErrTooManyRequests, got %T: %v", err, err)
	}

	if errTooMany.RetryAfter != 60*time.Second {
		t.Errorf("expected retry after 60s, got %v", errTooMany.RetryAfter)
	}
}

func TestGetOrderInfo_InternalServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}, MaxAttempts: 1}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if result != nil {
		t.Error("expected nil result")
	}

	var errUnavail *ErrExternalUnavailable
	if !errors.As(err, &errUnavail) {
		t.Fatalf("expected ErrExternalUnavailable, got %T: %v", err, err)
	}

	if errUnavail.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", errUnavail.StatusCode)
	}
}

func TestGetOrderInfo_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}, MaxAttempts: 1}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if result != nil {
		t.Error("expected nil result")
	}

	var errBadResp *ErrBadResponse
	if !errors.As(err, &errBadResp) {
		t.Fatalf("expected ErrBadResponse, got %T: %v", err, err)
	}
}

func TestGetOrderInfo_OrderNumberMismatch(t *testing.T) {
	accrual := decimal.NewFromFloat(100.0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := AccrualResponse{
			Order:   "99999",
			Status:  "PROCESSED",
			Accrual: &accrual,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}, MaxAttempts: 1}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if result != nil {
		t.Error("expected nil result")
	}

	var errBadResp *ErrBadResponse
	if !errors.As(err, &errBadResp) {
		t.Fatalf("expected ErrBadResponse, got %T: %v", err, err)
	}

	if !strings.Contains(errBadResp.Reason, "order mismatch") {
		t.Errorf("expected order mismatch error, got: %s", errBadResp.Reason)
	}
}

func TestGetOrderInfo_InvalidStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := AccrualResponse{
			Order:  "12345",
			Status: "UNKNOWN_STATUS",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}, MaxAttempts: 1}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if result != nil {
		t.Error("expected nil result")
	}

	var errBadResp *ErrBadResponse
	if !errors.As(err, &errBadResp) {
		t.Fatalf("expected ErrBadResponse, got %T: %v", err, err)
	}

	if !strings.Contains(errBadResp.Reason, "invalid status") {
		t.Errorf("expected invalid status error, got: %s", errBadResp.Reason)
	}
}

func TestGetOrderInfo_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}}
	client, _ := NewClient(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := client.GetOrderInfo(ctx, "12345")

	if result != nil {
		t.Error("expected nil result")
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetOrderInfo_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL:        server.URL,
		RequestTimeout: 100 * time.Millisecond,
		Logger:         &mockLogger{},
		MaxAttempts:    1,
	}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if result != nil {
		t.Error("expected nil result")
	}
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestGetOrderInfo_ZeroAccrual(t *testing.T) {
	accrual := decimal.NewFromFloat(0.0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := AccrualResponse{
			Order:   "12345",
			Status:  "PROCESSED",
			Accrual: &accrual,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}}
	client, _ := NewClient(cfg)
	result, err := client.GetOrderInfo(context.Background(), "12345")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Found {
		t.Error("expected order to be found")
	}
	if !result.Accrual.Valid {
		t.Fatal("expected accrual to be set")
	}
	if !result.Accrual.Decimal.IsZero() {
		t.Errorf("expected accrual 0.0, got %s", result.Accrual.Decimal)
	}
}

func TestGetOrderInfo_UnexpectedStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"Bad Request", http.StatusBadRequest},
		{"Forbidden", http.StatusForbidden},
		{"Not Found", http.StatusNotFound},
		{"Conflict", http.StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			cfg := Config{BaseURL: server.URL, Logger: &mockLogger{}, MaxAttempts: 1}
			client, _ := NewClient(cfg)
			result, err := client.GetOrderInfo(context.Background(), "12345")

			if result != nil {
				t.Error("expected nil result")
			}

			var errBadResp *ErrBadResponse
			if !errors.As(err, &errBadResp) {
				t.Fatalf("expected ErrBadResponse, got %T: %v", err, err)
			}

			if !strings.Contains(errBadResp.Reason, fmt.Sprintf("unexpected status code %d", tt.statusCode)) {
				t.Errorf("expected unexpected status code error, got: %s", errBadResp.Reason)
			}
		})
	}
}

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		status string
		valid  bool
	}{
		{"REGISTERED", true},
		{"PROCESSING", true},
		{"PROCESSED", true},
		{"INVALID", true},
		{"UNKNOWN", false},
		{"registered", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := ValidateStatus(tt.status)
			if result != tt.valid {
				t.Errorf("expected %v for status %q, got %v", tt.valid, tt.status, result)
			}
		})
	}
}
