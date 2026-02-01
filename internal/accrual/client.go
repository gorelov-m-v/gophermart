package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gophermart/internal/log"
)

// Default configuration values.
const (
	defaultTimeout               = 5 * time.Second
	defaultMaxConcurrentRequests = 10
	defaultMaxAttempts           = 3
	defaultRetryAfter            = 60 * time.Second
	initialBackoff               = 100 * time.Millisecond
)

// Client defines the interface for accrual system operations.
type Client interface {
	GetOrderInfo(ctx context.Context, orderNumber string) (*AccrualResult, error)
}

// Config holds the configuration for the accrual client.
type Config struct {
	BaseURL               string
	RequestTimeout        time.Duration
	MaxConcurrentRequests int
	MaxAttempts           int
	Logger                log.Logger
}

type client struct {
	baseURL     string
	httpClient  *http.Client
	logger      log.Logger
	semaphore   chan struct{}
	maxAttempts int
}

// NewClient creates a new accrual system client.
func NewClient(cfg Config) (Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("accrual base URL is required")
	}

	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("invalid accrual base URL: %w", err)
	}

	timeout := cfg.RequestTimeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	maxConcurrent := cfg.MaxConcurrentRequests
	if maxConcurrent == 0 {
		maxConcurrent = defaultMaxConcurrentRequests
	}

	maxAttempts := cfg.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = defaultMaxAttempts
	}

	return &client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger:      cfg.Logger,
		semaphore:   make(chan struct{}, maxConcurrent),
		maxAttempts: maxAttempts,
	}, nil
}

func (c *client) GetOrderInfo(ctx context.Context, orderNumber string) (*AccrualResult, error) {
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			backoff = time.Duration(float64(backoff) * 1.5)
		}

		result, err := c.doRequest(ctx, orderNumber)

		if err == nil {
			if c.logger != nil && attempt > 0 {
				c.logger.Info("accrual request succeeded after retry",
					"order", orderNumber,
					"attempt", attempt+1)
			}
			return result, nil
		}

		if _, ok := err.(*ErrTooManyRequests); ok {
			return nil, err
		}
		if _, ok := err.(*ErrBadResponse); ok {
			return nil, err
		}

		lastErr = err

		if c.logger != nil {
			c.logger.Warn("accrual request failed, will retry",
				"order", orderNumber,
				"attempt", attempt+1,
				"error", err)
		}
	}

	if c.logger != nil {
		c.logger.Error("accrual request failed after all attempts",
			"order", orderNumber,
			"attempts", c.maxAttempts,
			"error", lastErr)
	}

	return nil, lastErr
}

func (c *client) doRequest(ctx context.Context, orderNumber string) (*AccrualResult, error) {
	start := time.Now()

	reqURL := fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, &ErrExternalUnavailable{Err: err}
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", "Gophermart/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ErrExternalUnavailable{Err: err}
	}
	defer resp.Body.Close()

	latency := time.Since(start)

	switch resp.StatusCode {
	case http.StatusOK:
		return c.handle200(resp, orderNumber, latency)

	case http.StatusNoContent:
		if c.logger != nil {
			c.logger.Debug("accrual order not registered",
				"order", orderNumber,
				"latency_ms", latency.Milliseconds())
		}
		return &AccrualResult{Found: false}, nil

	case http.StatusTooManyRequests:
		return nil, c.handle429(resp, orderNumber)

	case http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		if c.logger != nil {
			c.logger.Warn("accrual server error",
				"order", orderNumber,
				"status", resp.StatusCode,
				"latency_ms", latency.Milliseconds())
		}
		return nil, &ErrExternalUnavailable{StatusCode: resp.StatusCode}

	default:
		if c.logger != nil {
			c.logger.Error("unexpected accrual response status",
				"order", orderNumber,
				"status", resp.StatusCode)
		}
		return nil, &ErrBadResponse{
			Reason: fmt.Sprintf("unexpected status code %d", resp.StatusCode),
		}
	}
}

func (c *client) handle200(resp *http.Response, orderNumber string, latency time.Duration) (*AccrualResult, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ErrBadResponse{Reason: "failed to read response body"}
	}

	var accrualResp AccrualResponse
	if err := json.Unmarshal(body, &accrualResp); err != nil {
		return nil, &ErrBadResponse{Reason: "invalid JSON"}
	}

	if accrualResp.Order != orderNumber {
		return nil, &ErrBadResponse{
			Reason: fmt.Sprintf("order mismatch: expected %s, got %s", orderNumber, accrualResp.Order),
		}
	}

	if !ValidateStatus(accrualResp.Status) {
		return nil, &ErrBadResponse{
			Reason: fmt.Sprintf("invalid status: %s", accrualResp.Status),
		}
	}

	var accrual decimal.NullDecimal
	if accrualResp.Accrual != nil {
		accrual = decimal.NullDecimal{Decimal: *accrualResp.Accrual, Valid: true}
	}

	result := &AccrualResult{
		Found:   true,
		Status:  AccrualStatus(accrualResp.Status),
		Accrual: accrual,
	}

	if c.logger != nil {
		c.logger.Debug("accrual order info received",
			"order", orderNumber,
			"status", result.Status,
			"accrual", result.Accrual,
			"latency_ms", latency.Milliseconds())
	}

	return result, nil
}

func (c *client) handle429(resp *http.Response, orderNumber string) error {
	retryAfter := defaultRetryAfter
	rawRetryAfter := resp.Header.Get("Retry-After")

	if rawRetryAfter != "" {
		if seconds, err := strconv.Atoi(rawRetryAfter); err == nil && seconds > 0 {
			retryAfter = time.Duration(seconds) * time.Second
		} else {
			if t, err := http.ParseTime(rawRetryAfter); err == nil {
				delta := time.Until(t)
				if delta > 0 {
					retryAfter = delta
				}
			}
		}
	}

	if retryAfter > 10*time.Minute {
		retryAfter = 10 * time.Minute
	}

	if c.logger != nil {
		c.logger.Warn("accrual rate limited",
			"order", orderNumber,
			"retry_after_sec", int(math.Ceil(retryAfter.Seconds())),
			"raw_retry_after", rawRetryAfter)
	}

	return &ErrTooManyRequests{
		RetryAfter:    retryAfter,
		RawRetryAfter: rawRetryAfter,
	}
}
