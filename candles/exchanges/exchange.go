package exchanges

import (
	"time"

	"github.com/linluma/candle/shared/models"
)

// RetryConfig holds retry configuration parameters
type RetryConfig struct {
	InitialDelay   time.Duration // e.g., 1 second
	MaxDelay       time.Duration // e.g., 30 seconds
	MaxRetries     int           // e.g., 10 attempts
	StormThreshold int           // e.g., 5 failures per minute
	BackoffFactor  float64       // e.g., 2.0 (exponential)
	Jitter         bool          // Add randomization to prevent thundering herd
}

// ConnectionHealth tracks exchange connection health
type ConnectionHealth struct {
	FailureCount     int
	LastFailureTime  time.Time
	ConsecutiveFails int
	RetryAttempt     int
	InStormMode      bool
}

// Exchange defines the interface for connecting to crypto exchanges
type Exchange interface {
	// Name returns the name of the exchange
	Name() models.ExchangeName

	// Subscribe starts receiving trade data for specified pairs
	Subscribe(pairs []string) error

	// Events returns a channel for receiving trade events
	Events() <-chan models.Trade

	// Disconnect closes the connection to the exchange
	Disconnect() error

	// IsConnected returns whether the exchange is currently connected
	IsConnected() bool

	FromExchangeSymbol(exchangeSymbol string) (string, error)

	// Retry and health management
	GetConnectionHealth() ConnectionHealth
	SetRetryConfig(config RetryConfig)
}
