package database

import (
	"errors"
	"github.com/lib/pq"
)

// Common database errors.
var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrAlreadyExists = errors.New("already exists")
)

// IsUniqueViolation checks if the error is a unique constraint violation.
func IsUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	var mockErr *MockUniqueViolationError
	if errors.As(err, &mockErr) {
		return true
	}
	return false
}

// MockUniqueViolationError is used for testing unique constraint violations.
type MockUniqueViolationError struct{}

func (e *MockUniqueViolationError) Error() string {
	return "mock unique violation"
}
