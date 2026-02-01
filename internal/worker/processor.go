// Package worker provides background processing for order accrual updates.
package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/shopspring/decimal"
	"gophermart/internal/accrual"
	"gophermart/internal/log"
	"gophermart/internal/repository"
	"gophermart/internal/service"
)

// Config holds the configuration for the order processor.
type Config struct {
	PollInterval       time.Duration
	BatchSize          int
	Concurrency        int
	NormalPollInterval time.Duration
	MaxBackoff         time.Duration
}

// OrderProcessor processes orders by polling the accrual system.
type OrderProcessor struct {
	ordersRepo    repository.OrdersRepository
	usersRepo     repository.UsersRepository
	accrualClient accrual.Client
	txManager     service.TransactionManager
	config        Config
	logger        log.Logger
}

// NewOrderProcessor creates a new OrderProcessor instance.
func NewOrderProcessor(
	ordersRepo repository.OrdersRepository,
	usersRepo repository.UsersRepository,
	accrualClient accrual.Client,
	txManager service.TransactionManager,
	config Config,
	logger log.Logger,
) *OrderProcessor {
	return &OrderProcessor{
		ordersRepo:    ordersRepo,
		usersRepo:     usersRepo,
		accrualClient: accrualClient,
		txManager:     txManager,
		config:        config,
		logger:        logger,
	}
}

// Start begins the order processing loop.
func (p *OrderProcessor) Start(ctx context.Context) error {
	p.logger.Info("order processor started",
		"poll_interval", p.config.PollInterval,
		"batch_size", p.config.BatchSize,
		"concurrency", p.config.Concurrency,
	)

	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("order processor stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := p.processBatch(ctx); err != nil {
				p.logger.Error("failed to process batch", "error", err)
			}
		}
	}
}

func (p *OrderProcessor) processBatch(ctx context.Context) error {
	now := time.Now()

	var orders []*repository.Order
	err := p.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		var err error
		orders, err = p.ordersRepo.PickDueOrdersForUpdate(ctx, tx, p.config.BatchSize, now)
		if err != nil {
			return fmt.Errorf("failed to pick orders: %w", err)
		}

		if len(orders) > 0 {
			orderIDs := make([]int64, len(orders))
			for i, order := range orders {
				orderIDs[i] = order.ID
			}
			minimalDelay := now.Add(1 * time.Second)
			return p.ordersRepo.UpdatePickedOrders(ctx, tx, orderIDs, minimalDelay)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to pick orders: %w", err)
	}

	if len(orders) == 0 {
		return nil
	}

	p.logger.Debug("picked orders for processing", "count", len(orders))

	sem := make(chan struct{}, p.config.Concurrency)
	errChan := make(chan error, len(orders))

	for _, order := range orders {
		order := order

		sem <- struct{}{}

		go func() {
			defer func() { <-sem }()

			if err := p.processOrder(ctx, order); err != nil {
				p.logger.Error("failed to process order",
					"order_id", order.ID,
					"order_number", order.Number,
					"error", err,
				)
				errChan <- err
			} else {
				errChan <- nil
			}
		}()
	}

	for i := 0; i < len(orders); i++ {
		<-errChan
	}

	return nil
}

func (p *OrderProcessor) processOrder(ctx context.Context, order *repository.Order) error {
	startTime := time.Now()

	result, err := p.accrualClient.GetOrderInfo(ctx, order.Number)

	latency := time.Since(startTime)

	if err != nil {
		return p.handleError(ctx, order, err, latency)
	}

	return p.handleResult(ctx, order, result, latency)
}

func (p *OrderProcessor) handleResult(ctx context.Context, order *repository.Order, result *accrual.AccrualResult, latency time.Duration) error {
	now := time.Now()
	nullAccrual := decimal.NullDecimal{}

	if !result.Found {
		nextPollAt := now.Add(p.config.NormalPollInterval)
		pollAttempts := order.PollAttempts + 1

		err := p.ordersRepo.UpdateAfterPoll(ctx, order.ID, order.Status, nullAccrual, nil, &nextPollAt, pollAttempts, nil)
		if err != nil {
			return fmt.Errorf("failed to update after 204: %w", err)
		}

		p.logger.Debug("order not registered in accrual system",
			"order_id", order.ID,
			"order_number", order.Number,
			"user_id", order.UserID,
			"poll_attempts", pollAttempts,
			"next_poll_at", nextPollAt,
			"latency_ms", latency.Milliseconds(),
		)

		return nil
	}

	if result.Status == accrual.StatusRegistered || result.Status == accrual.StatusProcessing {
		nextPollAt := now.Add(p.config.NormalPollInterval)
		pollAttempts := order.PollAttempts + 1
		status := repository.OrderStatusProcessing

		err := p.ordersRepo.UpdateAfterPoll(ctx, order.ID, status, nullAccrual, nil, &nextPollAt, pollAttempts, nil)
		if err != nil {
			return fmt.Errorf("failed to update after PROCESSING: %w", err)
		}

		p.logger.Info("order still processing",
			"order_id", order.ID,
			"order_number", order.Number,
			"user_id", order.UserID,
			"status", result.Status,
			"poll_attempts", pollAttempts,
			"next_poll_at", nextPollAt,
			"latency_ms", latency.Milliseconds(),
		)

		return nil
	}

	if result.Status == accrual.StatusInvalid {
		processedAt := now
		pollAttempts := order.PollAttempts + 1

		err := p.ordersRepo.UpdateAfterPoll(ctx, order.ID, repository.OrderStatusInvalid, nullAccrual, &processedAt, nil, pollAttempts, nil)
		if err != nil {
			return fmt.Errorf("failed to update after INVALID: %w", err)
		}

		p.logger.Info("order marked as invalid",
			"order_id", order.ID,
			"order_number", order.Number,
			"user_id", order.UserID,
			"poll_attempts", pollAttempts,
			"latency_ms", latency.Milliseconds(),
		)

		return nil
	}

	if result.Status == accrual.StatusProcessed {
		processedAt := now
		pollAttempts := order.PollAttempts + 1

		if !result.Accrual.Valid || !result.Accrual.Decimal.IsPositive() {
			err := p.ordersRepo.UpdateAfterPoll(ctx, order.ID, repository.OrderStatusProcessed, result.Accrual, &processedAt, nil, pollAttempts, nil)
			if err != nil {
				return fmt.Errorf("failed to update after PROCESSED (no accrual): %w", err)
			}

			p.logger.Info("order processed without accrual",
				"order_id", order.ID,
				"order_number", order.Number,
				"user_id", order.UserID,
				"poll_attempts", pollAttempts,
				"latency_ms", latency.Milliseconds(),
			)

			return nil
		}

		var credited bool
		err := p.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
			var err error
			credited, err = p.ordersRepo.FinalizeOrderTx(ctx, tx, order.ID, result.Accrual.Decimal, processedAt)
			if err != nil || !credited {
				return err
			}
			return p.usersRepo.CreditBalanceTx(ctx, tx, order.UserID, result.Accrual.Decimal)
		})

		if err != nil {
			return fmt.Errorf("failed to finalize processed order: %w", err)
		}

		if credited {
			p.logger.Info("order processed and credited",
				"order_id", order.ID,
				"order_number", order.Number,
				"user_id", order.UserID,
				"accrual", result.Accrual.Decimal,
				"poll_attempts", pollAttempts,
				"latency_ms", latency.Milliseconds(),
			)
		} else {
			p.logger.Warn("order already credited, skipped double credit",
				"order_id", order.ID,
				"order_number", order.Number,
				"user_id", order.UserID,
			)
		}

		return nil
	}

	return fmt.Errorf("unknown accrual status: %s", result.Status)
}

func (p *OrderProcessor) handleError(ctx context.Context, order *repository.Order, err error, latency time.Duration) error {
	now := time.Now()
	pollAttempts := order.PollAttempts + 1
	nullAccrual := decimal.NullDecimal{}

	var errTooMany *accrual.ErrTooManyRequests
	if errors.As(err, &errTooMany) {
		nextPollAt := now.Add(errTooMany.RetryAfter)
		lastError := "rate limited"

		updateErr := p.ordersRepo.UpdateAfterPoll(ctx, order.ID, repository.OrderStatusProcessing, nullAccrual, nil, &nextPollAt, pollAttempts, &lastError)
		if updateErr != nil {
			return fmt.Errorf("failed to update after 429: %w", updateErr)
		}

		p.logger.Warn("rate limited by accrual system",
			"order_id", order.ID,
			"order_number", order.Number,
			"user_id", order.UserID,
			"retry_after", errTooMany.RetryAfter,
			"next_poll_at", nextPollAt,
			"poll_attempts", pollAttempts,
			"latency_ms", latency.Milliseconds(),
		)

		return nil
	}

	var errBadResp *accrual.ErrBadResponse
	if errors.As(err, &errBadResp) {
		const badResponseBackoff = 5 * time.Minute
		nextPollAt := now.Add(badResponseBackoff)
		lastError := fmt.Sprintf("bad response: %s", errBadResp.Reason)

		updateErr := p.ordersRepo.UpdateAfterPoll(ctx, order.ID, repository.OrderStatusProcessing, nullAccrual, nil, &nextPollAt, pollAttempts, &lastError)
		if updateErr != nil {
			return fmt.Errorf("failed to update after bad response: %w", updateErr)
		}

		p.logger.Error("bad response from accrual system",
			"order_id", order.ID,
			"order_number", order.Number,
			"user_id", order.UserID,
			"reason", errBadResp.Reason,
			"next_poll_at", nextPollAt,
			"poll_attempts", pollAttempts,
			"latency_ms", latency.Milliseconds(),
		)

		return nil
	}

	backoff := p.calculateBackoff(pollAttempts)
	nextPollAt := now.Add(backoff)
	lastError := fmt.Sprintf("external error: %v", err)

	updateErr := p.ordersRepo.UpdateAfterPoll(ctx, order.ID, repository.OrderStatusProcessing, nullAccrual, nil, &nextPollAt, pollAttempts, &lastError)
	if updateErr != nil {
		return fmt.Errorf("failed to update after error: %w", updateErr)
	}

	p.logger.Warn("external system error",
		"order_id", order.ID,
		"order_number", order.Number,
		"user_id", order.UserID,
		"error", err,
		"backoff", backoff,
		"next_poll_at", nextPollAt,
		"poll_attempts", pollAttempts,
		"latency_ms", latency.Milliseconds(),
	)

	return nil
}

func (p *OrderProcessor) calculateBackoff(attempts int) time.Duration {
	seconds := math.Pow(2, float64(attempts))
	backoff := time.Duration(seconds) * time.Second

	if backoff > p.config.MaxBackoff {
		backoff = p.config.MaxBackoff
	}

	return backoff
}
