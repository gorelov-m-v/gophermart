// Package crypto provides cryptographic utilities for password hashing and token generation.
package crypto

import (
	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plain text password using bcrypt.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword checks if a plain text password matches a hash.
func VerifyPassword(hash string, plain string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	return err == nil
}
