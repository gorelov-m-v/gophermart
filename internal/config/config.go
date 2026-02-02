// Package config provides application configuration management.
package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// Config holds all application configuration values.
type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string

	OrderPollInterval       time.Duration
	OrderPollBatchSize      int
	OrderPollConcurrency    int
	OrderNormalPollInterval time.Duration
	OrderMaxBackoff         time.Duration
}

var (
	flagOnce             sync.Once
	flagRunAddress       string
	flagDatabaseURI      string
	flagAccrualSystemAddress string
)

func parseFlags() {
	flagOnce.Do(func() {
		flag.StringVar(&flagRunAddress, "a", ":8080", "Server run address")
		flag.StringVar(&flagDatabaseURI, "d", "", "Database connection URI")
		flag.StringVar(&flagAccrualSystemAddress, "r", "", "Accrual system address")
		flag.Parse()
	})
}

// Load loads configuration from flags and environment variables.
func Load() (*Config, error) {
	parseFlags()

	cfg := &Config{
		RunAddress:           flagRunAddress,
		DatabaseURI:          flagDatabaseURI,
		AccrualSystemAddress: flagAccrualSystemAddress,
		OrderPollInterval:       1 * time.Second,
		OrderPollBatchSize:      50,
		OrderPollConcurrency:    10,
		OrderNormalPollInterval: 5 * time.Second,
		OrderMaxBackoff:         10 * time.Minute,
	}

	if envRunAddress := os.Getenv("RUN_ADDRESS"); envRunAddress != "" && !isFlagPassed("a") {
		cfg.RunAddress = envRunAddress
	}
	if envDatabaseURI := os.Getenv("DATABASE_URI"); envDatabaseURI != "" && !isFlagPassed("d") {
		cfg.DatabaseURI = envDatabaseURI
	}
	if envAccrualSystemAddress := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envAccrualSystemAddress != "" && !isFlagPassed("r") {
		cfg.AccrualSystemAddress = envAccrualSystemAddress
	}

	if envPollInterval := os.Getenv("ORDER_POLL_INTERVAL"); envPollInterval != "" {
		d, err := time.ParseDuration(envPollInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid ORDER_POLL_INTERVAL %q: %w", envPollInterval, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("ORDER_POLL_INTERVAL must be positive, got %v", d)
		}
		cfg.OrderPollInterval = d
	}

	if envBatchSize := os.Getenv("ORDER_POLL_BATCH_SIZE"); envBatchSize != "" {
		n, err := strconv.Atoi(envBatchSize)
		if err != nil {
			return nil, fmt.Errorf("invalid ORDER_POLL_BATCH_SIZE %q: %w", envBatchSize, err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("ORDER_POLL_BATCH_SIZE must be positive, got %d", n)
		}
		if n > 1000 {
			return nil, fmt.Errorf("ORDER_POLL_BATCH_SIZE must be <= 1000, got %d", n)
		}
		cfg.OrderPollBatchSize = n
	}

	if envConcurrency := os.Getenv("ORDER_POLL_CONCURRENCY"); envConcurrency != "" {
		n, err := strconv.Atoi(envConcurrency)
		if err != nil {
			return nil, fmt.Errorf("invalid ORDER_POLL_CONCURRENCY %q: %w", envConcurrency, err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("ORDER_POLL_CONCURRENCY must be positive, got %d", n)
		}
		if n > 100 {
			return nil, fmt.Errorf("ORDER_POLL_CONCURRENCY must be <= 100, got %d", n)
		}
		cfg.OrderPollConcurrency = n
	}

	if envNormalPollInterval := os.Getenv("ORDER_NORMAL_POLL_INTERVAL"); envNormalPollInterval != "" {
		d, err := time.ParseDuration(envNormalPollInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid ORDER_NORMAL_POLL_INTERVAL %q: %w", envNormalPollInterval, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("ORDER_NORMAL_POLL_INTERVAL must be positive, got %v", d)
		}
		cfg.OrderNormalPollInterval = d
	}

	if envMaxBackoff := os.Getenv("ORDER_MAX_BACKOFF"); envMaxBackoff != "" {
		d, err := time.ParseDuration(envMaxBackoff)
		if err != nil {
			return nil, fmt.Errorf("invalid ORDER_MAX_BACKOFF %q: %w", envMaxBackoff, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("ORDER_MAX_BACKOFF must be positive, got %v", d)
		}
		cfg.OrderMaxBackoff = d
	}

	if cfg.OrderMaxBackoff < cfg.OrderPollInterval {
		return nil, fmt.Errorf("ORDER_MAX_BACKOFF (%v) must be >= ORDER_POLL_INTERVAL (%v)",
			cfg.OrderMaxBackoff, cfg.OrderPollInterval)
	}

	return cfg, nil
}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
