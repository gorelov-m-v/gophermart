// Package dto provides data transfer objects for HTTP requests and responses.
package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// BalanceResponse represents the response for balance endpoint.
type BalanceResponse struct {
	Current   decimal.Decimal `json:"current"`
	Withdrawn decimal.Decimal `json:"withdrawn"`
}

// WithdrawRequest represents the request for withdrawal endpoint.
type WithdrawRequest struct {
	Order string          `json:"order"`
	Sum   decimal.Decimal `json:"sum"`
}

// WithdrawalResponse represents a single withdrawal in the response.
type WithdrawalResponse struct {
	Order       string          `json:"order"`
	Sum         decimal.Decimal `json:"sum"`
	ProcessedAt string          `json:"processed_at"`
}

// FormatWithdrawalTime formats a time value for withdrawal response.
func FormatWithdrawalTime(t time.Time) string {
	return t.Format(time.RFC3339)
}
