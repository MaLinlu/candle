package exchanges

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// TestBinanceRetryLogic tests the retry logic with exponential backoff
func TestBinanceRetryLogic(t *testing.T) {
	t.Run("ConnectionSuccessNoRetry", func(t *testing.T) {
		// Create a test WebSocket server that accepts connections
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upgrader := websocket.Upgrader{}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Errorf("Failed to upgrade connection: %v", err)
				return
			}
			defer conn.Close()

			// Keep connection open for a short time
			time.Sleep(100 * time.Millisecond)
		}))
		defer server.Close()

		exchange := NewBinanceExchange()

		// Override the connect function for testing
		exchange.connectFunc = func(binancePairs []string) error {
			wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

			dialer := websocket.Dialer{
				HandshakeTimeout: 5 * time.Second,
			}

			conn, _, err := dialer.Dial(wsURL, nil)
			if err != nil {
				return err
			}

			exchange.conn = conn
			return nil
		}

		// Test successful connection (should not retry)
		startTime := time.Now()
		err := exchange.Subscribe([]string{"BTC-USDT"})
		duration := time.Since(startTime)

		assert.NoError(t, err)
		assert.True(t, exchange.IsConnected())
		assert.Less(t, duration, 2*time.Second, "Should connect quickly without retries")

		health := exchange.GetConnectionHealth()
		assert.Equal(t, 0, health.ConsecutiveFails)
		assert.Equal(t, 0, health.RetryAttempt)
		assert.False(t, health.InStormMode)

		exchange.Disconnect()
	})

	t.Run("ConnectionFailureWithRetry", func(t *testing.T) {
		exchange := NewBinanceExchange()

		// Configure fast retry for testing
		fastRetryConfig := RetryConfig{
			InitialDelay:   100 * time.Millisecond,
			MaxDelay:       500 * time.Millisecond,
			MaxRetries:     3,
			StormThreshold: 5,
			BackoffFactor:  2.0,
			Jitter:         false, // Disable jitter for predictable testing
		}
		exchange.SetRetryConfig(fastRetryConfig)

		failureCount := 0

		// Override connect to fail the first few times, then succeed
		exchange.connectFunc = func(binancePairs []string) error {
			failureCount++
			if failureCount < 3 {
				return assert.AnError // Simulate connection failure
			}

			// Don't set a connection for this test - we'll disconnect manually
			return nil
		}

		startTime := time.Now()
		err := exchange.Subscribe([]string{"BTC-USDT"})
		duration := time.Since(startTime)

		assert.NoError(t, err)
		assert.Equal(t, 3, failureCount, "Should have attempted 3 times")
		assert.GreaterOrEqual(t, duration, 200*time.Millisecond, "Should have delayed for retries")

		health := exchange.GetConnectionHealth()
		assert.Equal(t, 0, health.ConsecutiveFails, "Should reset on success")
		assert.Equal(t, 0, health.RetryAttempt, "Should reset on success")
		assert.False(t, health.InStormMode)

		// Manually set connected to false since we didn't create a real connection
		exchange.connected = false
	})

	t.Run("MaxRetriesExceeded", func(t *testing.T) {
		exchange := NewBinanceExchange()

		// Configure fast retry with low max retries
		fastRetryConfig := RetryConfig{
			InitialDelay:   50 * time.Millisecond,
			MaxDelay:       200 * time.Millisecond,
			MaxRetries:     2,
			StormThreshold: 5,
			BackoffFactor:  2.0,
			Jitter:         false,
		}
		exchange.SetRetryConfig(fastRetryConfig)

		failureCount := 0

		// Override connect to always fail
		exchange.connectFunc = func(binancePairs []string) error {
			failureCount++
			return assert.AnError // Always fail
		}

		err := exchange.Subscribe([]string{"BTC-USDT"})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to connect after retries")
		assert.Equal(t, 3, failureCount, "Should have attempted max retries + 1") // +1 for initial attempt
		assert.False(t, exchange.IsConnected())

		health := exchange.GetConnectionHealth()
		assert.Greater(t, health.FailureCount, 0)
		assert.Greater(t, health.ConsecutiveFails, 0)
	})

	t.Run("StormProtection", func(t *testing.T) {
		exchange := NewBinanceExchange()

		// Configure storm protection with low threshold
		stormConfig := RetryConfig{
			InitialDelay:   10 * time.Millisecond,
			MaxDelay:       50 * time.Millisecond,
			MaxRetries:     10,
			StormThreshold: 3, // Low threshold for testing
			BackoffFactor:  2.0,
			Jitter:         false,
		}
		exchange.SetRetryConfig(stormConfig)

		// Simulate multiple failures to trigger storm mode
		now := time.Now()
		for i := 0; i < 4; i++ {
			exchange.recentFailures = append(exchange.recentFailures, now.Add(-time.Duration(i)*time.Second))
		}

		// Override connect to always fail
		exchange.connectFunc = func(binancePairs []string) error {
			return assert.AnError
		}

		// Should enter storm mode and stop retrying
		err := exchange.Subscribe([]string{"BTC-USDT"})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storm mode")

		health := exchange.GetConnectionHealth()
		assert.True(t, health.InStormMode)
	})
}

// TestBinanceSymbolConversion tests the O(1) symbol conversion
func TestBinanceSymbolConversion(t *testing.T) {
	exchange := NewBinanceExchange()

	// Simulate subscription to populate cache
	pairs := []string{"BTC-USDT", "ETH-USDT", "SOL-USDT"}
	exchange.convertPairsToBinanceFormat(pairs)

	t.Run("ExchangeToCanonical", func(t *testing.T) {
		canonical, err := exchange.FromExchangeSymbol("BTCUSDT")
		assert.NoError(t, err)
		assert.Equal(t, "BTC-USDT", canonical)

		canonical, err = exchange.FromExchangeSymbol("ETHUSDT")
		assert.NoError(t, err)
		assert.Equal(t, "ETH-USDT", canonical)

		// Test case insensitive
		canonical, err = exchange.FromExchangeSymbol("btcusdt")
		assert.NoError(t, err)
		assert.Equal(t, "BTC-USDT", canonical)

		// Test non-existent symbol
		_, err = exchange.FromExchangeSymbol("INVALID")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found in cache")
	})
}

// TestBinanceHealthTracking tests connection health tracking
func TestBinanceHealthTracking(t *testing.T) {
	exchange := NewBinanceExchange()

	// Test initial health
	health := exchange.GetConnectionHealth()
	assert.Equal(t, 0, health.FailureCount)
	assert.Equal(t, 0, health.ConsecutiveFails)
	assert.Equal(t, 0, health.RetryAttempt)
	assert.False(t, health.InStormMode)

	// Record some failures
	exchange.recordFailure()
	exchange.recordFailure()
	exchange.recordFailure()

	health = exchange.GetConnectionHealth()
	assert.Equal(t, 3, health.FailureCount)
	assert.Equal(t, 3, health.ConsecutiveFails)
	assert.Equal(t, 3, health.RetryAttempt)
	assert.False(t, health.LastFailureTime.IsZero())

	// Record success - should reset consecutive fails and retry attempts
	exchange.recordSuccess()

	health = exchange.GetConnectionHealth()
	assert.Equal(t, 3, health.FailureCount)     // Total failures preserved
	assert.Equal(t, 0, health.ConsecutiveFails) // Reset
	assert.Equal(t, 0, health.RetryAttempt)     // Reset
	assert.False(t, health.InStormMode)         // Reset
}

// TestBinanceRetryConfiguration tests retry configuration
func TestBinanceRetryConfiguration(t *testing.T) {
	exchange := NewBinanceExchange()

	// Test default configuration
	defaultHealth := exchange.GetConnectionHealth()
	assert.False(t, defaultHealth.InStormMode)

	// Test custom configuration
	customConfig := RetryConfig{
		InitialDelay:   2 * time.Second,
		MaxDelay:       60 * time.Second,
		MaxRetries:     5,
		StormThreshold: 10,
		BackoffFactor:  3.0,
		Jitter:         false,
	}

	exchange.SetRetryConfig(customConfig)

	// Verify configuration was set (we can test this indirectly through behavior)
	assert.Equal(t, customConfig.InitialDelay, exchange.retryConfig.InitialDelay)
	assert.Equal(t, customConfig.MaxDelay, exchange.retryConfig.MaxDelay)
	assert.Equal(t, customConfig.MaxRetries, exchange.retryConfig.MaxRetries)
	assert.Equal(t, customConfig.StormThreshold, exchange.retryConfig.StormThreshold)
	assert.Equal(t, customConfig.BackoffFactor, exchange.retryConfig.BackoffFactor)
	assert.Equal(t, customConfig.Jitter, exchange.retryConfig.Jitter)
}
