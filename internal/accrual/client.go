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

// Config holds the configuration for the accrual client.
type Config struct {
	BaseURL               string
	RequestTimeout        time.Duration
	MaxConcurrentRequests int
	MaxAttempts           int
	Logger                log.Logger
}

// Client implements accrual system operations.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	logger      log.Logger
	semaphore   chan struct{}
	maxAttempts int
}

// NewClient creates a new accrual system client.
func NewClient(cfg Config) (*Client, error) {
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

	transport := &retryTransport{
		base:        http.DefaultTransport,
		maxAttempts: maxAttempts,
		logger:      cfg.Logger,
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		logger:      cfg.Logger,
		semaphore:   make(chan struct{}, maxConcurrent),
		maxAttempts: maxAttempts,
	}, nil
}

// retryTransport wraps http.RoundTripper with retry logic.
type retryTransport struct {
	base        http.RoundTripper
	maxAttempts int
	logger      log.Logger
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt < t.maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(backoff):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			backoff = time.Duration(float64(backoff) * 1.5)
		}

		resp, err := t.base.RoundTrip(req)
		if err == nil {
			if t.logger != nil && attempt > 0 {
				t.logger.Debug(fmt.Sprintf("request succeeded on attempt %d", attempt+1),
					"url", req.URL.String())
			}

			if resp.StatusCode == http.StatusTooManyRequests ||
				resp.StatusCode >= 500 {
				return resp, nil
			}

			return resp, nil
		}

		lastErr = err

		if t.logger != nil {
			t.logger.Warn("request failed, will retry",
				"url", req.URL.String(),
				"attempt", attempt+1,
				"error", err)
		}
	}

	if t.logger != nil {
		t.logger.Error("request failed after all attempts",
			"url", req.URL.String(),
			"attempts", t.maxAttempts,
			"error", lastErr)
	}

	return nil, lastErr
}

func (c *Client) GetOrderInfo(ctx context.Context, orderNumber string) (*AccrualResult, error) {
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return c.doRequest(ctx, orderNumber)
}

func (c *Client) doRequest(ctx context.Context, orderNumber string) (*AccrualResult, error) {
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

func (c *Client) handle200(resp *http.Response, orderNumber string, latency time.Duration) (*AccrualResult, error) {
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

func (c *Client) handle429(resp *http.Response, orderNumber string) error {
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
		if c.logger != nil {
			c.logger.Warn("retry-after value capped to maximum",
				"order", orderNumber,
				"original_retry_after_sec", int(retryAfter.Seconds()),
				"capped_to_sec", 600)
		}
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
