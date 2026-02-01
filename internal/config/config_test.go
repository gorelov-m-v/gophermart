package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.RunAddress)
	assert.Equal(t, 50, cfg.OrderPollBatchSize)
	assert.Equal(t, 10, cfg.OrderPollConcurrency)
}

func TestLoad_ValidEnvValues(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_POLL_INTERVAL", "5s")
	t.Setenv("ORDER_POLL_BATCH_SIZE", "100")
	t.Setenv("ORDER_POLL_CONCURRENCY", "20")
	t.Setenv("ORDER_NORMAL_POLL_INTERVAL", "10s")
	t.Setenv("ORDER_MAX_BACKOFF", "15m")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, 5*time.Second, cfg.OrderPollInterval)
	assert.Equal(t, 100, cfg.OrderPollBatchSize)
	assert.Equal(t, 20, cfg.OrderPollConcurrency)
}

func TestLoad_InvalidPollInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_POLL_INTERVAL", "invalid")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORDER_POLL_INTERVAL")
	assert.Contains(t, err.Error(), "invalid")
}

func TestLoad_NegativePollInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_POLL_INTERVAL", "-5s")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORDER_POLL_INTERVAL")
	assert.Contains(t, err.Error(), "positive")
}

func TestLoad_InvalidBatchSize(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_POLL_BATCH_SIZE", "not-a-number")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORDER_POLL_BATCH_SIZE")
	assert.Contains(t, err.Error(), "not-a-number")
}

func TestLoad_NegativeBatchSize(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_POLL_BATCH_SIZE", "-10")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORDER_POLL_BATCH_SIZE")
	assert.Contains(t, err.Error(), "positive")
}

func TestLoad_BatchSizeTooLarge(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_POLL_BATCH_SIZE", "5000")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORDER_POLL_BATCH_SIZE")
	assert.Contains(t, err.Error(), "<= 1000")
}

func TestLoad_InvalidConcurrency(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_POLL_CONCURRENCY", "abc")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORDER_POLL_CONCURRENCY")
}

func TestLoad_ConcurrencyTooLarge(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_POLL_CONCURRENCY", "500")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORDER_POLL_CONCURRENCY")
	assert.Contains(t, err.Error(), "<= 100")
}

func TestLoad_InvalidNormalPollInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_NORMAL_POLL_INTERVAL", "five minutes")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORDER_NORMAL_POLL_INTERVAL")
}

func TestLoad_InvalidMaxBackoff(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_MAX_BACKOFF", "wrong")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORDER_MAX_BACKOFF")
}

func TestLoad_MaxBackoffLessThanPollInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("ORDER_POLL_INTERVAL", "1m")
	t.Setenv("ORDER_MAX_BACKOFF", "30s")

	_, err := Load()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ORDER_MAX_BACKOFF")
	assert.Contains(t, err.Error(), "ORDER_POLL_INTERVAL")
}

func clearEnv(t *testing.T) {
	t.Helper()
	envVars := []string{
		"RUN_ADDRESS",
		"DATABASE_URI",
		"ACCRUAL_SYSTEM_ADDRESS",
		"ORDER_POLL_INTERVAL",
		"ORDER_POLL_BATCH_SIZE",
		"ORDER_POLL_CONCURRENCY",
		"ORDER_NORMAL_POLL_INTERVAL",
		"ORDER_MAX_BACKOFF",
	}
	for _, env := range envVars {
		os.Unsetenv(env)
	}
}
