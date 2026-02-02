package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shopspring/decimal"
	"gophermart/internal/database"
	"gophermart/internal/http/dto"
	"gophermart/internal/http/middleware"
	"gophermart/internal/httputil"
	"gophermart/internal/repository"
	"gophermart/internal/service"
	"gophermart/internal/validator"
)

// BalanceService defines the interface for balance operations.
type BalanceService interface {
	GetBalance(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error)
	Withdraw(ctx context.Context, userID int64, orderNumber string, sum decimal.Decimal) (service.WithdrawResult, error)
	ListWithdrawals(ctx context.Context, userID int64) ([]*repository.Withdrawal, error)
}

// BalanceHandler handles balance-related HTTP requests.
type BalanceHandler struct {
	balanceService BalanceService
}

// NewBalanceHandler creates a new BalanceHandler instance.
func NewBalanceHandler(balanceService BalanceService) *BalanceHandler {
	return &BalanceHandler{
		balanceService: balanceService,
	}
}

// Get handles balance retrieval requests.
func (h *BalanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	current, withdrawn, err := h.balanceService.GetBalance(r.Context(), userID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	response := dto.BalanceResponse{
		Current:   current,
		Withdrawn: withdrawn,
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}

// Withdraw handles withdrawal requests.
func (h *BalanceHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		httputil.WriteError(w, http.StatusBadRequest, "invalid content type")
		return
	}

	const maxWithdrawRequestSize = 10 * 1024
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWithdrawRequestSize))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	defer r.Body.Close()

	var req dto.WithdrawRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}

	orderNumber, err := validator.ValidateOrderNumber(req.Order)
	if err != nil {
		httputil.WriteError(w, http.StatusUnprocessableEntity, fmt.Sprintf("invalid order number: %v", err))
		return
	}

	if !req.Sum.IsPositive() {
		httputil.WriteError(w, http.StatusBadRequest, "sum must be positive")
		return
	}

	result, err := h.balanceService.Withdraw(r.Context(), userID, orderNumber, req.Sum)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			httputil.WriteError(w, http.StatusNotFound, "user not found")
			return
		}
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	switch result {
	case service.WithdrawOk:
		w.WriteHeader(http.StatusOK)
	case service.WithdrawAlreadyProcessed:
		w.WriteHeader(http.StatusOK)
	case service.WithdrawInsufficientFunds:
		httputil.WriteError(w, http.StatusPaymentRequired, "insufficient funds")
	}
}

// Withdrawals handles withdrawal history requests.
func (h *BalanceHandler) Withdrawals(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	withdrawals, err := h.balanceService.ListWithdrawals(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	response := make([]dto.WithdrawalResponse, len(withdrawals))
	for i, wd := range withdrawals {
		response[i] = dto.WithdrawalResponse{
			Order:       wd.OrderNumber,
			Sum:         wd.Sum,
			ProcessedAt: dto.FormatWithdrawalTime(wd.ProcessedAt),
		}
	}

	httputil.WriteJSON(w, http.StatusOK, response)
}
