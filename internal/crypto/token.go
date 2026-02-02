package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// TokenLength is the number of random bytes used for token generation.
const TokenLength = 32

// GenerateToken generates a cryptographically secure random token.
func GenerateToken() (string, error) {
	b := make([]byte, TokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// HashToken creates a SHA-256 hash of a token for secure storage.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(hash[:])
}
