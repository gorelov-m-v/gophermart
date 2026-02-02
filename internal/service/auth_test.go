package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
	"gophermart/internal/database"
	"gophermart/internal/repository"
)

type MockUsersRepository struct {
	CreateFunc     func(ctx context.Context, login, passwordHash string) (int64, error)
	GetByLoginFunc func(ctx context.Context, login string) (*repository.User, error)
}

func (m *MockUsersRepository) Create(ctx context.Context, login, passwordHash string) (int64, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, login, passwordHash)
	}
	return 0, errors.New("not implemented")
}

func (m *MockUsersRepository) GetByLogin(ctx context.Context, login string) (*repository.User, error) {
	if m.GetByLoginFunc != nil {
		return m.GetByLoginFunc(ctx, login)
	}
	return nil, errors.New("not implemented")
}

func (m *MockUsersRepository) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	return nil, errors.New("not implemented")
}

func (m *MockUsersRepository) GetBalances(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
	return decimal.Zero, decimal.Zero, errors.New("not implemented")
}

func (m *MockUsersRepository) GetUserForUpdateTx(ctx context.Context, tx *sql.Tx, userID int64) (*repository.User, error) {
	return nil, errors.New("not implemented")
}

func (m *MockUsersRepository) ApplyWithdrawTx(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
	return errors.New("not implemented")
}

func (m *MockUsersRepository) CreditBalanceTx(ctx context.Context, tx *sql.Tx, userID int64, amount decimal.Decimal) error {
	return errors.New("not implemented")
}

type MockSessionsRepository struct {
	CreateFunc         func(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) (int64, error)
	GetByTokenHashFunc func(ctx context.Context, tokenHash string) (*repository.Session, error)
	UpdateLastSeenFunc func(ctx context.Context, tokenHash string) error
}

func (m *MockSessionsRepository) Create(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) (int64, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, userID, tokenHash, expiresAt)
	}
	return 0, errors.New("not implemented")
}

func (m *MockSessionsRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*repository.Session, error) {
	if m.GetByTokenHashFunc != nil {
		return m.GetByTokenHashFunc(ctx, tokenHash)
	}
	return nil, errors.New("not implemented")
}

func (m *MockSessionsRepository) Revoke(ctx context.Context, tokenHash string) error {
	return errors.New("not implemented")
}

func (m *MockSessionsRepository) UpdateLastSeen(ctx context.Context, tokenHash string) error {
	if m.UpdateLastSeenFunc != nil {
		return m.UpdateLastSeenFunc(ctx, tokenHash)
	}
	return nil
}

func TestAuthService_Register_Success(t *testing.T) {
	mockUsers := &MockUsersRepository{
		CreateFunc: func(ctx context.Context, login, passwordHash string) (int64, error) {
			if login != "testuser" {
				t.Errorf("expected login 'testuser', got '%s'", login)
			}
			return 123, nil
		},
	}

	mockSessions := &MockSessionsRepository{
		CreateFunc: func(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) (int64, error) {
			if userID != 123 {
				t.Errorf("expected userID 123, got %d", userID)
			}
			return 1, nil
		},
	}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	token, err := service.Register(ctx, "testuser", "password123")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestAuthService_Register_LoginTaken(t *testing.T) {
	mockUsers := &MockUsersRepository{
		CreateFunc: func(ctx context.Context, login, passwordHash string) (int64, error) {
			return 0, database.ErrConflict
		},
	}

	mockSessions := &MockSessionsRepository{}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	_, err := service.Register(ctx, "existinguser", "password123")
	if !errors.Is(err, ErrLoginTaken) {
		t.Errorf("expected ErrLoginTaken, got %v", err)
	}
}

func TestAuthService_Register_CreateUserError(t *testing.T) {
	mockUsers := &MockUsersRepository{
		CreateFunc: func(ctx context.Context, login, passwordHash string) (int64, error) {
			return 0, errors.New("database error")
		},
	}

	mockSessions := &MockSessionsRepository{}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	_, err := service.Register(ctx, "testuser", "password123")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestAuthService_Register_CreateSessionError(t *testing.T) {
	mockUsers := &MockUsersRepository{
		CreateFunc: func(ctx context.Context, login, passwordHash string) (int64, error) {
			return 123, nil
		},
	}

	mockSessions := &MockSessionsRepository{
		CreateFunc: func(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) (int64, error) {
			return 0, errors.New("session creation failed")
		},
	}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	_, err := service.Register(ctx, "testuser", "password123")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	password := "password123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	passwordHash := string(hashedPassword)

	mockUsers := &MockUsersRepository{
		GetByLoginFunc: func(ctx context.Context, login string) (*repository.User, error) {
			return &repository.User{
				ID:           456,
				Login:        login,
				PasswordHash: passwordHash,
			}, nil
		},
	}

	mockSessions := &MockSessionsRepository{
		CreateFunc: func(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) (int64, error) {
			if userID != 456 {
				t.Errorf("expected userID 456, got %d", userID)
			}
			return 1, nil
		},
	}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	token, err := service.Login(ctx, "testuser", password)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	mockUsers := &MockUsersRepository{
		GetByLoginFunc: func(ctx context.Context, login string) (*repository.User, error) {
			return nil, database.ErrNotFound
		},
	}

	mockSessions := &MockSessionsRepository{}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	_, err := service.Login(ctx, "nonexistent", "password123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	password := "password123"
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	passwordHash := string(hashedPassword)

	mockUsers := &MockUsersRepository{
		GetByLoginFunc: func(ctx context.Context, login string) (*repository.User, error) {
			return &repository.User{
				ID:           456,
				Login:        login,
				PasswordHash: passwordHash,
			}, nil
		},
	}

	mockSessions := &MockSessionsRepository{}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	_, err = service.Login(ctx, "testuser", "wrongpassword")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_Login_GetUserError(t *testing.T) {
	mockUsers := &MockUsersRepository{
		GetByLoginFunc: func(ctx context.Context, login string) (*repository.User, error) {
			return nil, errors.New("database error")
		},
	}

	mockSessions := &MockSessionsRepository{}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	_, err := service.Login(ctx, "testuser", "password123")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestAuthService_ValidateSession_Success(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	mockUsers := &MockUsersRepository{}

	mockSessions := &MockSessionsRepository{
		GetByTokenHashFunc: func(ctx context.Context, tokenHash string) (*repository.Session, error) {
			return &repository.Session{
				ID:        1,
				UserID:    789,
				ExpiresAt: expiresAt,
			}, nil
		},
		UpdateLastSeenFunc: func(ctx context.Context, tokenHash string) error {
			return nil
		},
	}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	userID, err := service.ValidateSession(ctx, "valid-token")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if userID != 789 {
		t.Errorf("expected userID 789, got %d", userID)
	}
}

func TestAuthService_ValidateSession_NotFound(t *testing.T) {
	mockUsers := &MockUsersRepository{}

	mockSessions := &MockSessionsRepository{
		GetByTokenHashFunc: func(ctx context.Context, tokenHash string) (*repository.Session, error) {
			return nil, database.ErrNotFound
		},
	}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	_, err := service.ValidateSession(ctx, "invalid-token")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_ValidateSession_Revoked(t *testing.T) {
	now := time.Now()
	revokedAt := now.Add(-1 * time.Hour)
	expiresAt := now.Add(24 * time.Hour)

	mockUsers := &MockUsersRepository{}

	mockSessions := &MockSessionsRepository{
		GetByTokenHashFunc: func(ctx context.Context, tokenHash string) (*repository.Session, error) {
			return &repository.Session{
				ID:        1,
				UserID:    789,
				ExpiresAt: expiresAt,
				RevokedAt: &revokedAt,
			}, nil
		},
	}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	_, err := service.ValidateSession(ctx, "revoked-token")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_ValidateSession_Expired(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(-1 * time.Hour)

	mockUsers := &MockUsersRepository{}

	mockSessions := &MockSessionsRepository{
		GetByTokenHashFunc: func(ctx context.Context, tokenHash string) (*repository.Session, error) {
			return &repository.Session{
				ID:        1,
				UserID:    789,
				ExpiresAt: expiresAt,
			}, nil
		},
	}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	_, err := service.ValidateSession(ctx, "expired-token")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthService_ValidateSession_GetSessionError(t *testing.T) {
	mockUsers := &MockUsersRepository{}

	mockSessions := &MockSessionsRepository{
		GetByTokenHashFunc: func(ctx context.Context, tokenHash string) (*repository.Session, error) {
			return nil, errors.New("database error")
		},
	}

	service := NewAuthService(mockUsers, mockSessions)
	ctx := context.Background()

	_, err := service.ValidateSession(ctx, "some-token")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
