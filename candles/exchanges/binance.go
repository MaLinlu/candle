package exchanges

import (
	"github.com/linluma/candle/shared/models"
)

// BinanceExchange implements the Exchange interface for Binance
type BinanceExchange struct {
	connected bool
}

// NewBinanceExchange creates a new Binance exchange connector
func NewBinanceExchange() *BinanceExchange {
	return &BinanceExchange{}
}

// Connect establishes connection to Binance WebSocket
func (b *BinanceExchange) Connect(pairs []string) error {
	// TODO: Implement Binance WebSocket connection
	b.connected = true
	return nil
}

// Subscribe starts receiving trade data from Binance
func (b *BinanceExchange) Subscribe(tradeCh chan<- models.Trade) error {
	// TODO: Implement Binance trade subscription
	return nil
}

// Disconnect closes the Binance connection
func (b *BinanceExchange) Disconnect() error {
	// TODO: Implement Binance disconnection
	b.connected = false
	return nil
}

// Name returns the exchange name
func (b *BinanceExchange) Name() string {
	return "binance"
}

// IsConnected returns connection status
func (b *BinanceExchange) IsConnected() bool {
	return b.connected
}
