package crypto

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword_Success(t *testing.T) {
	password := "mySecurePassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Errorf("expected bcrypt hash, got: %s", hash)
	}

	if !VerifyPassword(hash, password) {
		t.Error("failed to verify password with generated hash")
	}
}

func TestHashPassword_DifferentPasswords(t *testing.T) {
	password1 := "password1"
	password2 := "password2"

	hash1, err := HashPassword(password1)
	if err != nil {
		t.Errorf("unexpected error for password1: %v", err)
	}

	hash2, err := HashPassword(password2)
	if err != nil {
		t.Errorf("unexpected error for password2: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected different hashes for different passwords")
	}
}

func TestHashPassword_SamePasswordDifferentHashes(t *testing.T) {
	password := "samePassword"

	hash1, err := HashPassword(password)
	if err != nil {
		t.Errorf("unexpected error for first hash: %v", err)
	}

	hash2, err := HashPassword(password)
	if err != nil {
		t.Errorf("unexpected error for second hash: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected different hashes for same password (different salt)")
	}

	if !VerifyPassword(hash1, password) {
		t.Error("failed to verify password with first hash")
	}

	if !VerifyPassword(hash2, password) {
		t.Error("failed to verify password with second hash")
	}
}

func TestHashPassword_EmptyPassword(t *testing.T) {
	password := ""

	hash, err := HashPassword(password)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty hash even for empty password")
	}

	if !VerifyPassword(hash, password) {
		t.Error("failed to verify empty password")
	}
}

func TestHashPassword_LongPassword(t *testing.T) {
	password := strings.Repeat("a", 72)

	hash, err := HashPassword(password)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	if !VerifyPassword(hash, password) {
		t.Error("failed to verify long password")
	}
}

func TestHashPassword_TooLongPassword(t *testing.T) {
	password := strings.Repeat("a", 100)

	_, err := HashPassword(password)
	if err == nil {
		t.Error("expected error for password longer than 72 bytes")
	}
}

func TestHashPassword_SpecialCharacters(t *testing.T) {
	password := "p@ssw0rd!#$%^&*()"

	hash, err := HashPassword(password)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !VerifyPassword(hash, password) {
		t.Error("failed to verify password with special characters")
	}
}

func TestHashPassword_Unicode(t *testing.T) {
	password := "пароль密码🔒"

	hash, err := HashPassword(password)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !VerifyPassword(hash, password) {
		t.Error("failed to verify password with unicode characters")
	}
}

func TestVerifyPassword_Success(t *testing.T) {
	password := "correctPassword"
	hash, _ := HashPassword(password)

	result := VerifyPassword(hash, password)
	if !result {
		t.Error("expected verification to succeed")
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	password := "correctPassword"
	wrongPassword := "wrongPassword"
	hash, _ := HashPassword(password)

	result := VerifyPassword(hash, wrongPassword)
	if result {
		t.Error("expected verification to fail for wrong password")
	}
}

func TestVerifyPassword_EmptyPassword(t *testing.T) {
	password := "somePassword"
	hash, _ := HashPassword(password)

	result := VerifyPassword(hash, "")
	if result {
		t.Error("expected verification to fail for empty password")
	}
}

func TestVerifyPassword_EmptyHash(t *testing.T) {
	password := "somePassword"

	result := VerifyPassword("", password)
	if result {
		t.Error("expected verification to fail for empty hash")
	}
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	password := "somePassword"
	invalidHash := "not-a-valid-bcrypt-hash"

	result := VerifyPassword(invalidHash, password)
	if result {
		t.Error("expected verification to fail for invalid hash")
	}
}

func TestVerifyPassword_CaseSensitive(t *testing.T) {
	password := "Password123"
	hash, _ := HashPassword(password)

	result1 := VerifyPassword(hash, "password123")
	if result1 {
		t.Error("expected verification to fail for different case")
	}

	result2 := VerifyPassword(hash, "PASSWORD123")
	if result2 {
		t.Error("expected verification to fail for different case")
	}

	result3 := VerifyPassword(hash, "Password123")
	if !result3 {
		t.Error("expected verification to succeed for correct case")
	}
}

func TestVerifyPassword_Whitespace(t *testing.T) {
	password := "password"
	hash, _ := HashPassword(password)

	result1 := VerifyPassword(hash, " password")
	if result1 {
		t.Error("expected verification to fail for leading space")
	}

	result2 := VerifyPassword(hash, "password ")
	if result2 {
		t.Error("expected verification to fail for trailing space")
	}

	result3 := VerifyPassword(hash, "pass word")
	if result3 {
		t.Error("expected verification to fail for space in middle")
	}
}

func TestHashPassword_BCryptDefaultCost(t *testing.T) {
	password := "testPassword"

	hash, err := HashPassword(password)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Errorf("failed to get cost: %v", err)
	}

	if cost != bcrypt.DefaultCost {
		t.Errorf("expected cost %d, got %d", bcrypt.DefaultCost, cost)
	}
}

func TestVerifyPassword_MultipleAttempts(t *testing.T) {
	password := "password123"
	hash, _ := HashPassword(password)

	for i := 0; i < 5; i++ {
		if !VerifyPassword(hash, password) {
			t.Errorf("verification failed on attempt %d", i+1)
		}
	}
}

func TestHashPassword_Consistency(t *testing.T) {
	password := "testPassword"

	for i := 0; i < 3; i++ {
		hash, err := HashPassword(password)
		if err != nil {
			t.Errorf("error on iteration %d: %v", i, err)
		}

		if !VerifyPassword(hash, password) {
			t.Errorf("verification failed on iteration %d", i)
		}

		if VerifyPassword(hash, "wrongPassword") {
			t.Errorf("verification succeeded for wrong password on iteration %d", i)
		}
	}
}
