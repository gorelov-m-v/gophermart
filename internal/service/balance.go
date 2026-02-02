package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gophermart/internal/database"
	"gophermart/internal/log"
	"gophermart/internal/repository"
)

// WithdrawResult represents the result of a withdrawal operation.
type WithdrawResult int

const (
	// WithdrawOk indicates successful withdrawal.
	WithdrawOk WithdrawResult = iota
	// WithdrawInsufficientFunds indicates not enough balance.
	WithdrawInsufficientFunds
	// WithdrawAlreadyProcessed indicates duplicate withdrawal request.
	WithdrawAlreadyProcessed
)

// BalanceService handles user balance and withdrawal operations.
type BalanceService struct {
	usersRepo       repository.UsersRepository
	withdrawalsRepo repository.WithdrawalsRepository
	txManager       TransactionManager
	logger          log.Logger
}

// NewBalanceService creates a new BalanceService instance.
func NewBalanceService(
	usersRepo repository.UsersRepository,
	withdrawalsRepo repository.WithdrawalsRepository,
	txManager TransactionManager,
	logger log.Logger,
) *BalanceService {
	return &BalanceService{
		usersRepo:       usersRepo,
		withdrawalsRepo: withdrawalsRepo,
		txManager:       txManager,
		logger:          logger,
	}
}

// GetBalance returns the current balance and total withdrawn amount for a user.
func (s *BalanceService) GetBalance(ctx context.Context, userID int64) (current decimal.Decimal, withdrawn decimal.Decimal, err error) {
	return s.usersRepo.GetBalances(ctx, userID)
}

// Withdraw processes a withdrawal request for a user.
func (s *BalanceService) Withdraw(ctx context.Context, userID int64, orderNumber string, sum decimal.Decimal) (WithdrawResult, error) {
	var result WithdrawResult
	var user *repository.User

	err := s.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		exists, err := s.withdrawalsRepo.ExistsByUserAndOrderTx(ctx, tx, userID, orderNumber)
		if err != nil {
			return fmt.Errorf("failed to check withdrawal existence: %w", err)
		}

		if exists {
			s.logger.Info("withdrawal already processed (idempotent)",
				"user_id", userID,
				"order_number", orderNumber,
				"sum", sum,
			)
			result = WithdrawAlreadyProcessed
			return nil
		}

		user, err = s.usersRepo.GetUserForUpdateTx(ctx, tx, userID)
		if err != nil {
			if errors.Is(err, database.ErrNotFound) {
				return database.ErrNotFound
			}
			return fmt.Errorf("failed to lock user: %w", err)
		}

		if user.CurrentBalance.LessThan(sum) {
			s.logger.Warn("insufficient funds for withdrawal",
				"user_id", userID,
				"order_number", orderNumber,
				"requested", sum,
				"available", user.CurrentBalance,
			)
			result = WithdrawInsufficientFunds
			return nil
		}

		err = s.usersRepo.ApplyWithdrawTx(ctx, tx, userID, sum)
		if err != nil {
			return fmt.Errorf("failed to apply withdraw: %w", err)
		}

		processedAt := time.Now()
		err = s.withdrawalsRepo.InsertTx(ctx, tx, userID, orderNumber, sum, processedAt)
		if err != nil {
			return fmt.Errorf("failed to insert withdrawal record: %w", err)
		}

		result = WithdrawOk
		return nil
	})

	if err != nil {
		return 0, err
	}

	if result == WithdrawOk {
		s.logger.Info("withdrawal processed successfully",
			"user_id", userID,
			"order_number", orderNumber,
			"sum", sum,
			"new_balance", user.CurrentBalance.Sub(sum),
		)
	}

	return result, nil
}

// ListWithdrawals returns all withdrawals for a user.
func (s *BalanceService) ListWithdrawals(ctx context.Context, userID int64) ([]*repository.Withdrawal, error) {
	return s.withdrawalsRepo.ListByUser(ctx, userID)
}
