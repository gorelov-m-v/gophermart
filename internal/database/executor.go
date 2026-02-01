package database

import (
	"context"
	"database/sql"
)

// Executor defines the interface for database query execution.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

// GetExecutor returns the appropriate executor based on whether a transaction is provided.
func GetExecutor(db *sql.DB, tx *sql.Tx) Executor {
	if tx != nil {
		return tx
	}
	return db
}
