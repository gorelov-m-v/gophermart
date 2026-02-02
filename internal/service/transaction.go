package service

import (
	"context"
	"database/sql"
	"fmt"
)

// TransactionManager provides database transaction management.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error
}

type transactionManager struct {
	db *sql.DB
}

// NewTransactionManager creates a new TransactionManager instance.
func NewTransactionManager(db *sql.DB) TransactionManager {
	return &transactionManager{db: db}
}

func (tm *transactionManager) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := tm.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
