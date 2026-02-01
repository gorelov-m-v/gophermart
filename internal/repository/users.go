// Package repository provides data access layer implementations.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gophermart/internal/database"
)

// User represents a user entity in the database.
type User struct {
	ID             int64
	Login          string
	PasswordHash   string
	CurrentBalance decimal.Decimal
	WithdrawnTotal decimal.Decimal
	CreatedAt      time.Time
}

// UsersRepository defines the interface for user data operations.
type UsersRepository interface {
	Create(ctx context.Context, login, passwordHash string) (int64, error)
	GetByLogin(ctx context.Context, login string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	GetBalances(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error)
	GetUserForUpdateTx(ctx context.Context, tx *sql.Tx, userID int64) (*User, error)
	ApplyWithdrawTx(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error
	CreditBalanceTx(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error
}

type usersRepository struct {
	db *sql.DB
}

// NewUsersRepository creates a new UsersRepository instance.
func NewUsersRepository(db *sql.DB) UsersRepository {
	return &usersRepository{db: db}
}

func (r *usersRepository) Create(ctx context.Context, login, passwordHash string) (int64, error) {
	var userID int64
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`,
		login, passwordHash,
	).Scan(&userID)

	if err != nil {
		if database.IsUniqueViolation(err) {
			return 0, database.ErrConflict
		}
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	return userID, nil
}

func (r *usersRepository) GetByLogin(ctx context.Context, login string) (*User, error) {
	user := &User{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, login, password_hash, current_balance, withdrawn_total, created_at
		 FROM users WHERE login = $1`,
		login,
	).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CurrentBalance, &user.WithdrawnTotal, &user.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, database.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by login: %w", err)
	}

	return user, nil
}

func (r *usersRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	user := &User{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, login, password_hash, current_balance, withdrawn_total, created_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CurrentBalance, &user.WithdrawnTotal, &user.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, database.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return user, nil
}

func (r *usersRepository) GetBalances(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
	err = r.db.QueryRowContext(
		ctx,
		`SELECT current_balance, withdrawn_total FROM users WHERE id = $1`,
		userID,
	).Scan(&current, &withdrawn)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return decimal.Zero, decimal.Zero, database.ErrNotFound
		}
		return decimal.Zero, decimal.Zero, fmt.Errorf("failed to get balances: %w", err)
	}

	return current, withdrawn, nil
}

func (r *usersRepository) GetUserForUpdateTx(ctx context.Context, tx *sql.Tx, userID int64) (*User, error) {
	user := &User{}
	err := tx.QueryRowContext(
		ctx,
		`SELECT id, login, password_hash, current_balance, withdrawn_total, created_at
		 FROM users WHERE id = $1 FOR UPDATE`,
		userID,
	).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CurrentBalance, &user.WithdrawnTotal, &user.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, database.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user for update: %w", err)
	}

	return user, nil
}

func (r *usersRepository) ApplyWithdrawTx(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
	result, err := tx.ExecContext(
		ctx,
		`UPDATE users
		 SET current_balance = current_balance - $1,
		     withdrawn_total = withdrawn_total + $1
		 WHERE id = $2`,
		amount, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to apply withdraw: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return database.ErrNotFound
	}

	return nil
}

func (r *usersRepository) CreditBalanceTx(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
	result, err := tx.ExecContext(
		ctx,
		`UPDATE users SET current_balance = current_balance + $1 WHERE id = $2`,
		amount, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to credit balance: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return database.ErrNotFound
	}

	return nil
}
