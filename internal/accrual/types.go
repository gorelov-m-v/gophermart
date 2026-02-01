// Package accrual provides client for the accrual system.
package accrual

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// AccrualStatus represents the status of an order in the accrual system.
type AccrualStatus string

// Accrual status constants.
const (
	StatusRegistered AccrualStatus = "REGISTERED"
	StatusInvalid    AccrualStatus = "INVALID"
	StatusProcessing AccrualStatus = "PROCESSING"
	StatusProcessed  AccrualStatus = "PROCESSED"
)

// AccrualResult represents the result of an accrual query.
type AccrualResult struct {
	Found   bool
	Status  AccrualStatus
	Accrual decimal.NullDecimal
}

// AccrualResponse represents the JSON response from the accrual system.
type AccrualResponse struct {
	Order   string           `json:"order"`
	Status  string           `json:"status"`
	Accrual *decimal.Decimal `json:"accrual,omitempty"`
}

// ErrTooManyRequests is returned when the accrual system rate limits requests.
type ErrTooManyRequests struct {
	RetryAfter    time.Duration
	RawRetryAfter string
}

func (e *ErrTooManyRequests) Error() string {
	return "accrual: rate limited"
}

// ErrExternalUnavailable is returned when the accrual system is unavailable.
type ErrExternalUnavailable struct {
	StatusCode int
	Err        error
}

func (e *ErrExternalUnavailable) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("accrual: external unavailable (status %d)", e.StatusCode)
	}
	return "accrual: external unavailable"
}

func (e *ErrExternalUnavailable) Unwrap() error {
	return e.Err
}

// ErrBadResponse is returned when the accrual system returns an invalid response.
type ErrBadResponse struct {
	Reason string
}

func (e *ErrBadResponse) Error() string {
	return fmt.Sprintf("accrual: bad response (%s)", e.Reason)
}

// ValidateStatus checks if the status is a valid accrual status.
func ValidateStatus(status string) bool {
	switch AccrualStatus(status) {
	case StatusRegistered, StatusInvalid, StatusProcessing, StatusProcessed:
		return true
	default:
		return false
	}
}
