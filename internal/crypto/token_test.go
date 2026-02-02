package crypto

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateToken_Success(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}

	_, err = base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Errorf("token is not valid base64 URL encoding: %v", err)
	}
}

func TestGenerateToken_Length(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		t.Errorf("failed to decode token: %v", err)
	}

	if len(decoded) != TokenLength {
		t.Errorf("expected token length %d, got %d", TokenLength, len(decoded))
	}
}

func TestGenerateToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		token, err := GenerateToken()
		if err != nil {
			t.Errorf("error on iteration %d: %v", i, err)
			continue
		}

		if tokens[token] {
			t.Errorf("duplicate token found: %s", token)
		}

		tokens[token] = true
	}

	if len(tokens) != iterations {
		t.Errorf("expected %d unique tokens, got %d", iterations, len(tokens))
	}
}

func TestGenerateToken_NoSpecialCharacters(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	allowedChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_="
	for _, char := range token {
		if !strings.ContainsRune(allowedChars, char) {
			t.Errorf("token contains invalid character: %c", char)
		}
	}
}

func TestGenerateToken_Multiple(t *testing.T) {
	for i := 0; i < 10; i++ {
		token, err := GenerateToken()
		if err != nil {
			t.Errorf("error on iteration %d: %v", i, err)
		}

		if token == "" {
			t.Errorf("empty token on iteration %d", i)
		}
	}
}

func TestHashToken_Success(t *testing.T) {
	token := "test-token-123"

	hash := HashToken(token)

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	_, err := base64.URLEncoding.DecodeString(hash)
	if err != nil {
		t.Errorf("hash is not valid base64 URL encoding: %v", err)
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	token := "test-token-456"

	hash1 := HashToken(token)
	hash2 := HashToken(token)

	if hash1 != hash2 {
		t.Errorf("expected same hash for same token, got %s and %s", hash1, hash2)
	}
}

func TestHashToken_DifferentTokens(t *testing.T) {
	token1 := "token1"
	token2 := "token2"

	hash1 := HashToken(token1)
	hash2 := HashToken(token2)

	if hash1 == hash2 {
		t.Error("expected different hashes for different tokens")
	}
}

func TestHashToken_EmptyToken(t *testing.T) {
	token := ""

	hash := HashToken(token)

	if hash == "" {
		t.Error("expected non-empty hash even for empty token")
	}

	_, err := base64.URLEncoding.DecodeString(hash)
	if err != nil {
		t.Errorf("failed to decode hash: %v", err)
	}
}

func TestHashToken_Length(t *testing.T) {
	token := "some-token"

	hash := HashToken(token)

	decoded, err := base64.URLEncoding.DecodeString(hash)
	if err != nil {
		t.Errorf("failed to decode hash: %v", err)
	}

	if len(decoded) != 32 {
		t.Errorf("expected hash length 32, got %d", len(decoded))
	}
}

func TestHashToken_CaseSensitive(t *testing.T) {
	token1 := "Token"
	token2 := "token"

	hash1 := HashToken(token1)
	hash2 := HashToken(token2)

	if hash1 == hash2 {
		t.Error("expected different hashes for different case")
	}
}

func TestHashToken_Whitespace(t *testing.T) {
	token1 := "token"
	token2 := "token "
	token3 := " token"

	hash1 := HashToken(token1)
	hash2 := HashToken(token2)
	hash3 := HashToken(token3)

	if hash1 == hash2 {
		t.Error("expected different hashes for token with trailing space")
	}

	if hash1 == hash3 {
		t.Error("expected different hashes for token with leading space")
	}

	if hash2 == hash3 {
		t.Error("expected different hashes for different whitespace positions")
	}
}

func TestHashToken_SpecialCharacters(t *testing.T) {
	token := "token!@#$%^&*()"

	hash := HashToken(token)

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	hash2 := HashToken(token)
	if hash != hash2 {
		t.Error("expected same hash for token with special characters")
	}
}

func TestHashToken_Unicode(t *testing.T) {
	token := "токен密码🔒"

	hash := HashToken(token)

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	hash2 := HashToken(token)
	if hash != hash2 {
		t.Error("expected same hash for unicode token")
	}
}

func TestHashToken_LongToken(t *testing.T) {
	token := strings.Repeat("a", 1000)

	hash := HashToken(token)

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	decoded, err := base64.URLEncoding.DecodeString(hash)
	if err != nil {
		t.Errorf("failed to decode hash: %v", err)
	}

	if len(decoded) != 32 {
		t.Errorf("expected hash length 32, got %d", len(decoded))
	}
}

func TestHashToken_Consistency(t *testing.T) {
	token := "consistent-token"

	hashes := make([]string, 5)
	for i := 0; i < 5; i++ {
		hashes[i] = HashToken(token)
	}

	for i := 1; i < len(hashes); i++ {
		if hashes[0] != hashes[i] {
			t.Errorf("inconsistent hash on iteration %d: expected %s, got %s", i, hashes[0], hashes[i])
		}
	}
}

func TestGenerateToken_AndHash_Integration(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Errorf("failed to generate token: %v", err)
	}

	hash := HashToken(token)

	if hash == "" {
		t.Error("expected non-empty hash")
	}

	hash2 := HashToken(token)
	if hash != hash2 {
		t.Error("expected consistent hash for generated token")
	}

	token2, err := GenerateToken()
	if err != nil {
		t.Errorf("failed to generate second token: %v", err)
	}

	hash3 := HashToken(token2)
	if hash == hash3 {
		t.Error("expected different hashes for different generated tokens")
	}
}

func TestHashToken_NoCollisions(t *testing.T) {
	tokens := []string{
		"token1",
		"token2",
		"token11",
		"token21",
		"1token",
		"2token",
	}

	hashes := make(map[string]string)
	for _, token := range tokens {
		hash := HashToken(token)
		if existing, found := hashes[hash]; found {
			t.Errorf("hash collision: %s and %s produce same hash", token, existing)
		}
		hashes[hash] = token
	}
}

func TestGenerateToken_NotPredictable(t *testing.T) {
	token1, err := GenerateToken()
	if err != nil {
		t.Errorf("failed to generate first token: %v", err)
	}

	token2, err := GenerateToken()
	if err != nil {
		t.Errorf("failed to generate second token: %v", err)
	}

	if token1 == token2 {
		t.Error("tokens should not be identical")
	}

	decoded1, _ := base64.URLEncoding.DecodeString(token1)
	decoded2, _ := base64.URLEncoding.DecodeString(token2)

	differentBytes := 0
	for i := 0; i < len(decoded1); i++ {
		if decoded1[i] != decoded2[i] {
			differentBytes++
		}
	}

	if differentBytes < 5 {
		t.Errorf("tokens are too similar, only %d bytes differ", differentBytes)
	}
}
