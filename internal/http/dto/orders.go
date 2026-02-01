package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// OrderResponse represents the response for order endpoint.
type OrderResponse struct {
	Number     string           `json:"number"`
	Status     string           `json:"status"`
	Accrual    *decimal.Decimal `json:"accrual,omitempty"`
	UploadedAt string           `json:"uploaded_at"`
}

// OrderToResponse converts order data to response format.
func OrderToResponse(number string, status string, accrual decimal.NullDecimal, uploadedAt time.Time) OrderResponse {
	var accrualPtr *decimal.Decimal
	if accrual.Valid {
		accrualPtr = &accrual.Decimal
	}
	return OrderResponse{
		Number:     number,
		Status:     status,
		Accrual:    accrualPtr,
		UploadedAt: uploadedAt.Format(time.RFC3339),
	}
}
