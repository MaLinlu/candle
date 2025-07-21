package exchanges

import (
	"fmt"
	"testing"
	"time"

	"github.com/linluma/candle/shared/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOKXExchange_RetryLogic(t *testing.T) {
	exchange := NewOKXExchange()

	// Configure faster retry for testing
	exchange.SetRetryConfig(RetryConfig{
		InitialDelay:   10 * time.Millisecond,
		MaxDelay:       100 * time.Millisecond,
		MaxRetries:     3,
		StormThreshold: 2,
		BackoffFactor:  2.0,
		Jitter:         false, // Disable jitter for predictable tests
	})

	t.Run("SuccessWithoutRetry", func(t *testing.T) {
		exchange := NewOKXExchange()
		exchange.SetRetryConfig(RetryConfig{
			InitialDelay:   10 * time.Millisecond,
			MaxDelay:       100 * time.Millisecond,
			MaxRetries:     3,
			StormThreshold: 2,
			BackoffFactor:  2.0,
			Jitter:         false,
		})

		// Mock successful connection
		exchange.connectFunc = func(pairs []string) error {
			return nil
		}

		err := exchange.Subscribe([]string{"BTC-USDT"})
		assert.NoError(t, err)
		assert.True(t, exchange.IsConnected())

		health := exchange.GetConnectionHealth()
		assert.Equal(t, 0, health.ConsecutiveFails)
		assert.Equal(t, 0, health.RetryAttempt)
		assert.False(t, health.InStormMode)
	})

	t.Run("FailureWithRetries", func(t *testing.T) {
		exchange := NewOKXExchange()
		exchange.SetRetryConfig(RetryConfig{
			InitialDelay:   10 * time.Millisecond,
			MaxDelay:       100 * time.Millisecond,
			MaxRetries:     3,
			StormThreshold: 5, // High threshold to avoid storm mode
			BackoffFactor:  2.0,
			Jitter:         false,
		})

		attempts := 0
		exchange.connectFunc = func(pairs []string) error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("connection failed, attempt %d", attempts)
			}
			return nil // Success on 3rd attempt
		}

		err := exchange.Subscribe([]string{"BTC-USDT"})
		assert.NoError(t, err)
		assert.True(t, exchange.IsConnected())
		assert.Equal(t, 3, attempts)

		health := exchange.GetConnectionHealth()
		assert.Equal(t, 0, health.ConsecutiveFails) // Reset after success
		assert.Equal(t, 0, health.RetryAttempt)     // Reset after success
		assert.False(t, health.InStormMode)
	})

	t.Run("MaxRetriesExceeded", func(t *testing.T) {
		exchange := NewOKXExchange()
		exchange.SetRetryConfig(RetryConfig{
			InitialDelay:   10 * time.Millisecond,
			MaxDelay:       100 * time.Millisecond,
			MaxRetries:     2,
			StormThreshold: 5, // High threshold to avoid storm mode
			BackoffFactor:  2.0,
			Jitter:         false,
		})

		attempts := 0
		exchange.connectFunc = func(pairs []string) error {
			attempts++
			return fmt.Errorf("connection always fails, attempt %d", attempts)
		}

		err := exchange.Subscribe([]string{"BTC-USDT"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect after retries")
		assert.False(t, exchange.IsConnected())
		assert.Equal(t, 3, attempts) // Initial + 2 retries

		health := exchange.GetConnectionHealth()
		assert.Greater(t, health.ConsecutiveFails, 0)
		assert.Greater(t, health.FailureCount, 0)
	})

	t.Run("StormProtection", func(t *testing.T) {
		exchange := NewOKXExchange()
		exchange.SetRetryConfig(RetryConfig{
			InitialDelay:   1 * time.Millisecond,
			MaxDelay:       10 * time.Millisecond,
			MaxRetries:     10,
			StormThreshold: 2, // Low threshold to trigger storm mode
			BackoffFactor:  2.0,
			Jitter:         false,
		})

		// Simulate multiple failures to trigger storm mode
		for i := 0; i < 3; i++ {
			exchange.recordFailure()
		}

		// Should be in storm mode now
		assert.True(t, exchange.isInStormMode())

		// Connection should fail due to storm mode
		exchange.connectFunc = func(pairs []string) error {
			return nil // Would succeed, but storm mode prevents it
		}

		err := exchange.Subscribe([]string{"BTC-USDT"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storm mode")
		assert.False(t, exchange.IsConnected())
	})
}

func TestOKXExchange_SymbolConversion(t *testing.T) {
	exchange := NewOKXExchange()
	pairs := []string{"BTC-USDT", "ETH-USDT", "SOL-USDT"}
	exchange.convertPairsToOKXFormat(pairs)

	t.Run("ExchangeToCanonical", func(t *testing.T) {
		symbol, err := exchange.FromExchangeSymbol("BTC-USDT-SWAP")
		assert.NoError(t, err)
		assert.Equal(t, "BTC-USDT", symbol)

		symbol, err = exchange.FromExchangeSymbol("ETH-USDT-SWAP")
		assert.NoError(t, err)
		assert.Equal(t, "ETH-USDT", symbol)

		// Test non-existent symbol
		_, err = exchange.FromExchangeSymbol("INVALID-PAIR-SWAP")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found in cache")
	})

	t.Run("ConvertPairsToOKXFormat", func(t *testing.T) {
		okxPairs := exchange.convertPairsToOKXFormat([]string{"BTC-USDT", "ETH-USDT"})
		assert.Equal(t, []string{"BTC-USDT-SWAP", "ETH-USDT-SWAP"}, okxPairs)

		// Verify cache was populated
		symbol, err := exchange.FromExchangeSymbol("BTC-USDT-SWAP")
		assert.NoError(t, err)
		assert.Equal(t, "BTC-USDT", symbol)
	})
}

func TestOKXExchange_ConnectionHealthTracking(t *testing.T) {
	exchange := NewOKXExchange()

	// Initial health should be clean
	health := exchange.GetConnectionHealth()
	assert.Equal(t, 0, health.FailureCount)
	assert.Equal(t, 0, health.ConsecutiveFails)
	assert.Equal(t, 0, health.RetryAttempt)
	assert.False(t, health.InStormMode)

	// Record some failures
	exchange.recordFailure()
	exchange.recordFailure()

	health = exchange.GetConnectionHealth()
	assert.Equal(t, 2, health.FailureCount)
	assert.Equal(t, 2, health.ConsecutiveFails)
	assert.Equal(t, 2, health.RetryAttempt)
	assert.False(t, health.LastFailureTime.IsZero())

	// Record success should reset counters
	exchange.recordSuccess()

	health = exchange.GetConnectionHealth()
	assert.Equal(t, 2, health.FailureCount)     // Total count preserved
	assert.Equal(t, 0, health.ConsecutiveFails) // Reset
	assert.Equal(t, 0, health.RetryAttempt)     // Reset
	assert.False(t, health.InStormMode)         // Reset
}

func TestOKXExchange_ConnectionManagement(t *testing.T) {
	exchange := NewOKXExchange()

	// Initially not connected
	assert.False(t, exchange.IsConnected())

	// Mock successful connection
	exchange.connectFunc = func(pairs []string) error {
		return nil
	}

	// Subscribe should connect
	err := exchange.Subscribe([]string{"BTC-USDT"})
	assert.NoError(t, err)
	assert.True(t, exchange.IsConnected())

	// Second subscribe should fail (already connected)
	err = exchange.Subscribe([]string{"ETH-USDT"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already connected")

	// Disconnect should work
	err = exchange.Disconnect()
	assert.NoError(t, err)
	assert.False(t, exchange.IsConnected())

	// Second disconnect should be safe
	err = exchange.Disconnect()
	assert.NoError(t, err)
}

func TestOKXExchange_Channels(t *testing.T) {
	exchange := NewOKXExchange()

	// Events channel should be available
	assert.NotNil(t, exchange.Events())

	// Should be able to read from events channel without blocking (empty)
	select {
	case <-exchange.Events():
		t.Fatal("Should not receive events before connection")
	default:
		// Expected - no events yet
	}
}

func TestOKXExchange_TradeNormalization(t *testing.T) {
	exchange := NewOKXExchange()
	// Setup symbol mapping
	exchange.exchangeToCanonical["BTC-USDT-SWAP"] = "BTC-USDT"

	tradeData := struct {
		InstID    string `json:"instId"`
		TradeID   string `json:"tradeId"`
		Price     string `json:"px"`
		Size      string `json:"sz"`
		Side      string `json:"side"`
		Timestamp string `json:"ts"`
	}{
		InstID:    "BTC-USDT-SWAP",
		TradeID:   "123456",
		Price:     "50000.50",
		Size:      "0.1",
		Side:      "buy",
		Timestamp: "1640995200000", // 2022-01-01 00:00:00 UTC in milliseconds
	}

	trade, err := exchange.normalizedTradeEvent(tradeData)
	require.NoError(t, err)

	assert.Equal(t, models.ExchangeOKX, trade.Exchange)
	assert.Equal(t, "BTC-USDT", trade.Pair)
	assert.Equal(t, 50000.50, trade.Price)
	assert.Equal(t, 0.1, trade.Volume)
	assert.Equal(t, "123456", trade.TradeID)
	assert.Equal(t, time.Unix(0, 1640995200000*int64(time.Millisecond)), trade.Timestamp)
}

func TestOKXExchange_InvalidData(t *testing.T) {
	exchange := NewOKXExchange()
	exchange.exchangeToCanonical["BTC-USDT-SWAP"] = "BTC-USDT"

	t.Run("InvalidPrice", func(t *testing.T) {
		tradeData := struct {
			InstID    string `json:"instId"`
			TradeID   string `json:"tradeId"`
			Price     string `json:"px"`
			Size      string `json:"sz"`
			Side      string `json:"side"`
			Timestamp string `json:"ts"`
		}{
			InstID:    "BTC-USDT-SWAP",
			TradeID:   "123456",
			Price:     "invalid_price",
			Size:      "0.1",
			Side:      "buy",
			Timestamp: "1640995200000",
		}

		_, err := exchange.normalizedTradeEvent(tradeData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid price")
	})

	t.Run("InvalidSize", func(t *testing.T) {
		tradeData := struct {
			InstID    string `json:"instId"`
			TradeID   string `json:"tradeId"`
			Price     string `json:"px"`
			Size      string `json:"sz"`
			Side      string `json:"side"`
			Timestamp string `json:"ts"`
		}{
			InstID:    "BTC-USDT-SWAP",
			TradeID:   "123456",
			Price:     "50000.50",
			Size:      "invalid_size",
			Side:      "buy",
			Timestamp: "1640995200000",
		}

		_, err := exchange.normalizedTradeEvent(tradeData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid size")
	})

	t.Run("InvalidTimestamp", func(t *testing.T) {
		tradeData := struct {
			InstID    string `json:"instId"`
			TradeID   string `json:"tradeId"`
			Price     string `json:"px"`
			Size      string `json:"sz"`
			Side      string `json:"side"`
			Timestamp string `json:"ts"`
		}{
			InstID:    "BTC-USDT-SWAP",
			TradeID:   "123456",
			Price:     "50000.50",
			Size:      "0.1",
			Side:      "buy",
			Timestamp: "invalid_timestamp",
		}

		_, err := exchange.normalizedTradeEvent(tradeData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid timestamp")
	})

	t.Run("UnknownSymbol", func(t *testing.T) {
		tradeData := struct {
			InstID    string `json:"instId"`
			TradeID   string `json:"tradeId"`
			Price     string `json:"px"`
			Size      string `json:"sz"`
			Side      string `json:"side"`
			Timestamp string `json:"ts"`
		}{
			InstID:    "UNKNOWN-PAIR-SWAP",
			TradeID:   "123456",
			Price:     "50000.50",
			Size:      "0.1",
			Side:      "buy",
			Timestamp: "1640995200000",
		}

		_, err := exchange.normalizedTradeEvent(tradeData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found in cache")
	})
}
