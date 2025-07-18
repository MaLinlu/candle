package exchanges

import (
	"github.com/linluma/candle/shared/models"
)

// CoinbaseExchange implements the Exchange interface for Coinbase
type CoinbaseExchange struct {
	connected bool
}

// NewCoinbaseExchange creates a new Coinbase exchange connector
func NewCoinbaseExchange() *CoinbaseExchange {
	return &CoinbaseExchange{}
}

// Connect establishes connection to Coinbase WebSocket
func (c *CoinbaseExchange) Connect(pairs []string) error {
	// TODO: Implement Coinbase WebSocket connection
	c.connected = true
	return nil
}

// Subscribe starts receiving trade data from Coinbase
func (c *CoinbaseExchange) Subscribe(tradeCh chan<- models.Trade) error {
	// TODO: Implement Coinbase trade subscription
	return nil
}

// Disconnect closes the Coinbase connection
func (c *CoinbaseExchange) Disconnect() error {
	// TODO: Implement Coinbase disconnection
	c.connected = false
	return nil
}

// Name returns the exchange name
func (c *CoinbaseExchange) Name() string {
	return "coinbase"
}

// IsConnected returns connection status
func (c *CoinbaseExchange) IsConnected() bool {
	return c.connected
}
