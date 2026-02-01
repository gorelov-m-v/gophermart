package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"gophermart/internal/http/dto"
	"gophermart/internal/http/middleware"
	"gophermart/internal/httputil"
	"gophermart/internal/repository"
	"gophermart/internal/service"
	"gophermart/internal/validator"
)

const maxOrderBodySize = 4096

// OrdersService defines the interface for order operations.
type OrdersService interface {
	SubmitOrder(ctx context.Context, userID int64, number string) (service.SubmitResult, error)
	ListOrders(ctx context.Context, userID int64) ([]*repository.Order, error)
}

// OrdersHandler handles order-related HTTP requests.
type OrdersHandler struct {
	ordersService OrdersService
}

// NewOrdersHandler creates a new OrdersHandler instance.
func NewOrdersHandler(ordersService OrdersService) *OrdersHandler {
	return &OrdersHandler{
		ordersService: ordersService,
	}
}

// Upload handles order upload requests.
func (h *OrdersHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") {
		httputil.WriteError(w, http.StatusBadRequest, "invalid content type, expected text/plain")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxOrderBodySize))
	if err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	defer r.Body.Close()

	orderNumber, err := validator.ValidateOrderNumber(string(body))
	if err != nil {
		if errors.Is(err, validator.ErrEmptyOrderNumber) ||
			errors.Is(err, validator.ErrInvalidOrderNumber) ||
			errors.Is(err, validator.ErrInvalidLuhn) {
			httputil.WriteError(w, http.StatusUnprocessableEntity, "invalid order number format")
			return
		}
		httputil.WriteError(w, http.StatusBadRequest, "invalid request")
		return
	}

	result, err := h.ordersService.SubmitOrder(r.Context(), userID, orderNumber)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	switch result {
	case service.SubmitResultCreated:
		w.WriteHeader(http.StatusAccepted)
	case service.SubmitResultAlreadyOwned:
		w.WriteHeader(http.StatusOK)
	case service.SubmitResultOwnedByAnother:
		httputil.WriteError(w, http.StatusConflict, "order already uploaded by another user")
	}
}

// List handles order listing requests.
func (h *OrdersHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orders, err := h.ordersService.ListOrders(r.Context(), userID)
	if err != nil {
		httputil.WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	response := make([]dto.OrderResponse, 0, len(orders))
	for _, order := range orders {
		response = append(response, dto.OrderToResponse(
			order.Number,
			order.Status,
			order.Accrual,
			order.UploadedAt,
		))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}
