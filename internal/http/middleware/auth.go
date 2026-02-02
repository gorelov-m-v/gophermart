// Package middleware provides HTTP middleware functions.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"gophermart/internal/httputil"
	"gophermart/internal/service"
)

type contextKey string

// UserIDKey is the context key for storing user ID.
const (
	UserIDKey contextKey = "userID"
)

// AuthValidator defines the interface for session validation.
type AuthValidator interface {
	ValidateSession(ctx context.Context, token string) (int64, error)
}

// Auth returns a middleware that validates user authentication.
func Auth(authService AuthValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			userID, err := authService.ValidateSession(r.Context(), token)
			if err != nil {
				if errors.Is(err, service.ErrInvalidCredentials) {
					httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
					return
				}
				httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" && parts[1] != "" {
			if !strings.HasPrefix(parts[1], " ") {
				return parts[1]
			}
		}
	}

	cookie, err := r.Cookie("Authorization")
	if err == nil {
		return cookie.Value
	}

	return ""
}

// GetUserID extracts the user ID from the context.
func GetUserID(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(UserIDKey).(int64)
	return userID, ok
}
