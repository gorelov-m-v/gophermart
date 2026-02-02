// Package validator provides validation utilities.
package validator

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrEmptyOrderNumber is returned when order number is empty.
var ErrEmptyOrderNumber = errors.New("order number cannot be empty")

// ErrInvalidOrderNumber is returned when order number contains non-digit characters.
type ErrInvalidOrderNumber struct {
	OrderNumber string
}

func (e *ErrInvalidOrderNumber) Error() string {
	return fmt.Sprintf("order number %q must contain only digits", e.OrderNumber)
}

// ErrInvalidLuhn is returned when order number fails Luhn validation.
type ErrInvalidLuhn struct {
	OrderNumber string
}

func (e *ErrInvalidLuhn) Error() string {
	return fmt.Sprintf("order number %q failed Luhn validation", e.OrderNumber)
}

// ValidateOrderNumber validates and normalizes an order number.
func ValidateOrderNumber(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)

	if normalized == "" {
		return "", ErrEmptyOrderNumber
	}

	for _, r := range normalized {
		if r < '0' || r > '9' {
			return "", &ErrInvalidOrderNumber{OrderNumber: normalized}
		}
	}

	if !ValidateLuhn(normalized) {
		return "", &ErrInvalidLuhn{OrderNumber: normalized}
	}

	return normalized, nil
}

// ValidateLuhn checks if a number passes the Luhn algorithm validation.
func ValidateLuhn(number string) bool {
	var sum int
	var alternate bool

	for i := len(number) - 1; i >= 0; i-- {
		digit, err := strconv.Atoi(string(number[i]))
		if err != nil {
			return false
		}

		if alternate {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
		alternate = !alternate
	}

	return sum%10 == 0
}
