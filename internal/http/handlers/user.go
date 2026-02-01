// Package handlers provides HTTP request handlers.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"gophermart/internal/httputil"
	"gophermart/internal/service"
)

// AuthService defines the interface for authentication operations.
type AuthService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
	ValidateSession(ctx context.Context, token string) (int64, error)
}

// UserHandler handles user-related HTTP requests.
type UserHandler struct {
	authService AuthService
}

// NewUserHandler creates a new UserHandler instance.
func NewUserHandler(authService AuthService) *UserHandler {
	return &UserHandler{
		authService: authService,
	}
}

type authRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Register handles user registration requests.
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req authRequest

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request format")
		return
	}

	req.Login = strings.TrimSpace(req.Login)
	req.Password = strings.TrimSpace(req.Password)

	if req.Login == "" || req.Password == "" {
		httputil.WriteError(w, http.StatusBadRequest, "login and password are required")
		return
	}

	token, err := h.authService.Register(r.Context(), req.Login, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrLoginTaken) {
			httputil.WriteError(w, http.StatusConflict, "login already exists")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	setAuthCookie(w, token)

	w.WriteHeader(http.StatusOK)
}

// Login handles user login requests.
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req authRequest

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request format")
		return
	}

	req.Login = strings.TrimSpace(req.Login)
	req.Password = strings.TrimSpace(req.Password)

	if req.Login == "" || req.Password == "" {
		httputil.WriteError(w, http.StatusBadRequest, "login and password are required")
		return
	}

	token, err := h.authService.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			httputil.WriteError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	setAuthCookie(w, token)

	w.WriteHeader(http.StatusOK)
}

func setAuthCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     "Authorization",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(24 * time.Hour.Seconds()),
	}
	http.SetCookie(w, cookie)
}
