// Package service provides business logic for the loyalty system.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gophermart/internal/crypto"
	"gophermart/internal/database"
	"gophermart/internal/repository"
)

// SessionTTL defines the duration of a user session.
const SessionTTL = 24 * time.Hour

// ErrInvalidCredentials is returned when login or password is incorrect.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrLoginTaken is returned when attempting to register with an existing login.
var ErrLoginTaken = errors.New("login already taken")

// AuthService handles user registration, authentication, and session management.
type AuthService struct {
	usersRepo    repository.UsersRepository
	sessionsRepo repository.SessionsRepository
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(usersRepo repository.UsersRepository, sessionsRepo repository.SessionsRepository) *AuthService {
	return &AuthService{
		usersRepo:    usersRepo,
		sessionsRepo: sessionsRepo,
	}
}

// Register creates a new user and returns a session token.
func (s *AuthService) Register(ctx context.Context, login, password string) (string, error) {
	passwordHash, err := crypto.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	userID, err := s.usersRepo.Create(ctx, login, passwordHash)
	if err != nil {
		if errors.Is(err, database.ErrConflict) {
			return "", ErrLoginTaken
		}
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	token, err := s.createSession(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return token, nil
}

// Login authenticates a user and returns a session token.
func (s *AuthService) Login(ctx context.Context, login, password string) (string, error) {
	user, err := s.usersRepo.GetByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	if !crypto.VerifyPassword(user.PasswordHash, password) {
		return "", ErrInvalidCredentials
	}

	token, err := s.createSession(ctx, user.ID)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return token, nil
}

// ValidateSession checks if a session token is valid and returns the user ID.
func (s *AuthService) ValidateSession(ctx context.Context, token string) (int64, error) {
	tokenHash := crypto.HashToken(token)

	session, err := s.sessionsRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return 0, ErrInvalidCredentials
		}
		return 0, fmt.Errorf("failed to get session: %w", err)
	}

	if session.RevokedAt != nil {
		return 0, ErrInvalidCredentials
	}

	if time.Now().After(session.ExpiresAt) {
		return 0, ErrInvalidCredentials
	}

	_ = s.sessionsRepo.UpdateLastSeen(ctx, tokenHash)

	return session.UserID, nil
}

func (s *AuthService) createSession(ctx context.Context, userID int64) (string, error) {
	token, err := crypto.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	tokenHash := crypto.HashToken(token)

	expiresAt := time.Now().Add(SessionTTL)
	_, err = s.sessionsRepo.Create(ctx, userID, tokenHash, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to create session in db: %w", err)
	}

	return token, nil
}
