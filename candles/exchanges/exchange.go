package exchanges

import "github.com/linluma/candle/shared/models"

// Exchange defines the interface for connecting to crypto exchanges
type Exchange interface {
	// Connect establishes connection to the exchange for given trading pairs
	Connect(pairs []string) error

	// Subscribe starts receiving trade data and sends it to the provided channel
	Subscribe(tradeCh chan<- models.Trade) error

	// Disconnect closes the connection to the exchange
	Disconnect() error

	// Name returns the name of the exchange
	Name() string

	// IsConnected returns whether the exchange is currently connected
	IsConnected() bool
}
