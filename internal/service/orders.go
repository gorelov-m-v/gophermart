package service

import (
	"context"
	"errors"
	"fmt"

	"gophermart/internal/database"
	"gophermart/internal/repository"
)

// SubmitResult represents the result of submitting an order.
type SubmitResult int

const (
	// SubmitResultCreated indicates a new order was created.
	SubmitResultCreated SubmitResult = iota
	// SubmitResultAlreadyOwned indicates the order was already submitted by this user.
	SubmitResultAlreadyOwned
	// SubmitResultOwnedByAnother indicates the order belongs to another user.
	SubmitResultOwnedByAnother
)

// ErrOrderOwnedByAnotherUser is returned when order belongs to another user.
var ErrOrderOwnedByAnotherUser = errors.New("order already owned by another user")

// ErrOrderAlreadyOwned is returned when user already submitted this order.
var ErrOrderAlreadyOwned = errors.New("order already owned by this user")

// OrdersService handles order submission and retrieval.
type OrdersService struct {
	ordersRepo repository.OrdersRepository
}

// NewOrdersService creates a new OrdersService instance.
func NewOrdersService(ordersRepo repository.OrdersRepository) *OrdersService {
	return &OrdersService{
		ordersRepo: ordersRepo,
	}
}

// SubmitOrder submits a new order for the user.
func (s *OrdersService) SubmitOrder(ctx context.Context, userID int64, number string) (SubmitResult, error) {
	_, err := s.ordersRepo.Create(ctx, userID, number)
	if err == nil {
		return SubmitResultCreated, nil
	}

	if errors.Is(err, database.ErrAlreadyExists) {
		order, err := s.ordersRepo.GetByNumber(ctx, number)
		if err != nil {
			return 0, fmt.Errorf("failed to get order by number: %w", err)
		}

		if order.UserID == userID {
			return SubmitResultAlreadyOwned, nil
		}

		return SubmitResultOwnedByAnother, nil
	}

	return 0, fmt.Errorf("failed to create order: %w", err)
}

// ListOrders returns all orders for the given user.
func (s *OrdersService) ListOrders(ctx context.Context, userID int64) ([]*repository.Order, error) {
	orders, err := s.ordersRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}

	return orders, nil
}
