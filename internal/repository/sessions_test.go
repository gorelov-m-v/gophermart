package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gophermart/internal/database"
)

func TestSessionsRepository_Create_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSessionsRepository(db)
	ctx := context.Background()

	userID := int64(1)
	tokenHash := "hashedtoken123"
	expiresAt := time.Now().Add(24 * time.Hour)
	expectedID := int64(42)

	rows := sqlmock.NewRows([]string{"id"}).AddRow(expectedID)
	mock.ExpectQuery("INSERT INTO user_sessions").
		WithArgs(userID, tokenHash, expiresAt).
		WillReturnRows(rows)

	sessionID, err := repo.Create(ctx, userID, tokenHash, expiresAt)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if sessionID != expectedID {
		t.Errorf("expected sessionID %d, got %d", expectedID, sessionID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionsRepository_Create_Conflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSessionsRepository(db)
	ctx := context.Background()

	expiresAt := time.Now().Add(24 * time.Hour)

	mock.ExpectQuery("INSERT INTO user_sessions").
		WithArgs(int64(1), "duplicate", expiresAt).
		WillReturnError(&database.MockUniqueViolationError{})

	_, err = repo.Create(ctx, 1, "duplicate", expiresAt)
	if !errors.Is(err, database.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionsRepository_GetByTokenHash_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSessionsRepository(db)
	ctx := context.Background()

	tokenHash := "hashedtoken123"
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	rows := sqlmock.NewRows([]string{"id", "user_id", "token_hash", "created_at", "expires_at", "revoked_at", "last_seen_at"}).
		AddRow(int64(1), int64(10), tokenHash, now, expiresAt, nil, nil)

	mock.ExpectQuery("SELECT id, user_id, token_hash, created_at, expires_at, revoked_at, last_seen_at FROM user_sessions WHERE token_hash").
		WithArgs(tokenHash).
		WillReturnRows(rows)

	session, err := repo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if session.ID != 1 {
		t.Errorf("expected ID 1, got %d", session.ID)
	}
	if session.UserID != 10 {
		t.Errorf("expected UserID 10, got %d", session.UserID)
	}
	if session.TokenHash != tokenHash {
		t.Errorf("expected TokenHash %s, got %s", tokenHash, session.TokenHash)
	}
	if session.RevokedAt != nil {
		t.Errorf("expected RevokedAt to be nil, got %v", session.RevokedAt)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionsRepository_GetByTokenHash_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSessionsRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT id, user_id, token_hash, created_at, expires_at, revoked_at, last_seen_at FROM user_sessions WHERE token_hash").
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetByTokenHash(ctx, "nonexistent")
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionsRepository_GetByTokenHash_Revoked(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSessionsRepository(db)
	ctx := context.Background()

	tokenHash := "revokedtoken"
	now := time.Now()
	revokedAt := now.Add(-1 * time.Hour)

	rows := sqlmock.NewRows([]string{"id", "user_id", "token_hash", "created_at", "expires_at", "revoked_at", "last_seen_at"}).
		AddRow(int64(1), int64(10), tokenHash, now, now.Add(24*time.Hour), &revokedAt, nil)

	mock.ExpectQuery("SELECT id, user_id, token_hash, created_at, expires_at, revoked_at, last_seen_at FROM user_sessions WHERE token_hash").
		WithArgs(tokenHash).
		WillReturnRows(rows)

	session, err := repo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if session.RevokedAt == nil {
		t.Error("expected RevokedAt to be set")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionsRepository_Revoke_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSessionsRepository(db)
	ctx := context.Background()

	tokenHash := "hashedtoken123"

	mock.ExpectExec("UPDATE user_sessions SET revoked_at = now\\(\\) WHERE token_hash").
		WithArgs(tokenHash).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.Revoke(ctx, tokenHash)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionsRepository_Revoke_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSessionsRepository(db)
	ctx := context.Background()

	mock.ExpectExec("UPDATE user_sessions SET revoked_at = now\\(\\) WHERE token_hash").
		WithArgs("nonexistent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.Revoke(ctx, "nonexistent")
	if !errors.Is(err, database.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionsRepository_UpdateLastSeen_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSessionsRepository(db)
	ctx := context.Background()

	tokenHash := "hashedtoken123"

	mock.ExpectExec("UPDATE user_sessions SET last_seen_at = now\\(\\) WHERE token_hash").
		WithArgs(tokenHash).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.UpdateLastSeen(ctx, tokenHash)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionsRepository_UpdateLastSeen_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSessionsRepository(db)
	ctx := context.Background()

	mock.ExpectExec("UPDATE user_sessions SET last_seen_at = now\\(\\) WHERE token_hash").
		WithArgs("nonexistent").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.UpdateLastSeen(ctx, "nonexistent")
	if err != nil {
		t.Errorf("expected nil (UpdateLastSeen doesn't check rows affected), got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSessionsRepository_UpdateLastSeen_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSessionsRepository(db)
	ctx := context.Background()

	expectedErr := errors.New("db error")

	mock.ExpectExec("UPDATE user_sessions SET last_seen_at = now\\(\\) WHERE token_hash").
		WithArgs("token").
		WillReturnError(expectedErr)

	err = repo.UpdateLastSeen(ctx, "token")
	if err == nil {
		t.Error("expected error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
