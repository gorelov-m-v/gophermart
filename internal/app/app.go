// Package app provides the main application entry point and lifecycle management.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"gophermart/internal/accrual"
	"gophermart/internal/config"
	"gophermart/internal/database"
	httpserver "gophermart/internal/http"
	"gophermart/internal/log"
	"gophermart/internal/repository"
	"gophermart/internal/service"
	"gophermart/internal/worker"
)

// App represents the main application with all its dependencies.
type App struct {
	logger log.Logger
	config *config.Config
	db     *database.DB
	server *http.Server
}

// New creates a new App instance with the given logger.
func New(logger log.Logger) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &App{
		logger: logger,
		config: cfg,
	}, nil
}

// Run starts the application and blocks until shutdown.
func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a.logger.Info("starting application", "run_address", a.config.RunAddress)

	if a.config.DatabaseURI == "" {
		return errors.New("DATABASE_URI is required")
	}

	db, err := database.NewDB(database.Config{
		DSN:            a.config.DatabaseURI,
		MigrationsPath: "migrations",
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	a.db = db
	defer a.db.Close()

	a.logger.Info("connected to database, migrations applied")

	txManager := service.NewTransactionManager(db.GetConn())

	usersRepo := repository.NewUsersRepository(db.GetConn())
	sessionsRepo := repository.NewSessionsRepository(db.GetConn())
	ordersRepo := repository.NewOrdersRepository(db.GetConn())
	withdrawalsRepo := repository.NewWithdrawalsRepository(db.GetConn())

	authService := service.NewAuthService(usersRepo, sessionsRepo)
	ordersService := service.NewOrdersService(ordersRepo)
	balanceService := service.NewBalanceService(usersRepo, withdrawalsRepo, txManager, a.logger)

	var accrualClient *accrual.Client
	if a.config.AccrualSystemAddress != "" {
		accrualClient, err = accrual.NewClient(accrual.Config{
			BaseURL:               a.config.AccrualSystemAddress,
			RequestTimeout:        5 * time.Second,
			MaxConcurrentRequests: 10,
			MaxAttempts:           3,
			Logger:                a.logger,
		})
		if err != nil {
			return fmt.Errorf("failed to create accrual client: %w", err)
		}
		a.logger.Info("accrual client initialized", "base_url", a.config.AccrualSystemAddress)
	} else {
		a.logger.Warn("accrual system address not configured, accrual client disabled")
	}

	router := httpserver.NewRouter(&httpserver.RouterDeps{
		AuthService:    authService,
		OrdersService:  ordersService,
		BalanceService: balanceService,
	})
	a.server = &http.Server{
		Addr:         a.config.RunAddress,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	var orderProcessor *worker.OrderProcessor
	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	workerErrors := make(chan error, 1)
	if accrualClient != nil {
		orderProcessor = worker.NewOrderProcessor(
			ordersRepo,
			usersRepo,
			accrualClient,
			txManager,
			worker.Config{
				PollInterval:       a.config.OrderPollInterval,
				BatchSize:          a.config.OrderPollBatchSize,
				Concurrency:        a.config.OrderPollConcurrency,
				NormalPollInterval: a.config.OrderNormalPollInterval,
				MaxBackoff:         a.config.OrderMaxBackoff,
			},
			a.logger,
		)

		go func() {
			workerErrors <- orderProcessor.Start(workerCtx)
		}()
		a.logger.Info("order processor started")
	} else {
		a.logger.Info("order processor disabled (no accrual client)")
	}

	serverErrors := make(chan error, 1)
	go func() {
		a.logger.Info("server listening", "address", a.config.RunAddress)
		serverErrors <- a.server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server error: %w", err)
		}
	case err := <-workerErrors:
		if err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Error("worker error", "error", err)
		}
	case <-ctx.Done():
		a.logger.Info("shutdown signal received")

		workerCancel()
		a.logger.Info("worker shutdown initiated")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := a.server.Shutdown(shutdownCtx); err != nil {
			a.logger.Error("graceful shutdown failed", "error", err)
			if err := a.server.Close(); err != nil {
				return fmt.Errorf("could not stop server: %w", err)
			}
		}
	}

	return nil
}
