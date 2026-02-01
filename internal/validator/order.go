// Package validator provides validation utilities.
package validator

import (
	"errors"
	"strconv"
	"strings"
)

// Order validation errors.
var (
	ErrEmptyOrderNumber   = errors.New("order number cannot be empty")
	ErrInvalidOrderNumber = errors.New("order number must contain only digits")
	ErrInvalidLuhn        = errors.New("order number failed Luhn validation")
)

// ValidateOrderNumber validates and normalizes an order number.
func ValidateOrderNumber(raw string) (string, error) {
	normalized := strings.TrimSpace(raw)

	if normalized == "" {
		return "", ErrEmptyOrderNumber
	}

	for _, r := range normalized {
		if r < '0' || r > '9' {
			return "", ErrInvalidOrderNumber
		}
	}

	if !ValidateLuhn(normalized) {
		return "", ErrInvalidLuhn
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
