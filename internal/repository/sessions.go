package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gophermart/internal/database"
	"time"
)

// Session represents a user session entity in the database.
type Session struct {
	ID         int64
	UserID     int64
	TokenHash  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	LastSeenAt *time.Time
}

// SessionsRepository defines the interface for session data operations.
type SessionsRepository interface {
	Create(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) (int64, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*Session, error)
	Revoke(ctx context.Context, tokenHash string) error
	UpdateLastSeen(ctx context.Context, tokenHash string) error
}

type sessionsRepository struct {
	db *sql.DB
}

// NewSessionsRepository creates a new SessionsRepository instance.
func NewSessionsRepository(db *sql.DB) SessionsRepository {
	return &sessionsRepository{db: db}
}

func (r *sessionsRepository) Create(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) (int64, error) {
	var sessionID int64
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO user_sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3) RETURNING id`,
		userID, tokenHash, expiresAt,
	).Scan(&sessionID)

	if err != nil {
		if database.IsUniqueViolation(err) {
			return 0, database.ErrConflict
		}
		return 0, fmt.Errorf("failed to create session: %w", err)
	}

	return sessionID, nil
}

func (r *sessionsRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	session := &Session{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, user_id, token_hash, created_at, expires_at, revoked_at, last_seen_at
		 FROM user_sessions WHERE token_hash = $1`,
		tokenHash,
	).Scan(&session.ID, &session.UserID, &session.TokenHash, &session.CreatedAt,
		&session.ExpiresAt, &session.RevokedAt, &session.LastSeenAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, database.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get session by token hash: %w", err)
	}

	return session, nil
}

func (r *sessionsRepository) Revoke(ctx context.Context, tokenHash string) error {
	result, err := r.db.ExecContext(
		ctx,
		`UPDATE user_sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
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

func (r *sessionsRepository) UpdateLastSeen(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE user_sessions SET last_seen_at = now() WHERE token_hash = $1`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("failed to update last seen: %w", err)
	}

	return nil
}
