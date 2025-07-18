package exchanges

import (
	"github.com/linluma/candle/shared/models"
)

// OKXExchange implements the Exchange interface for OKX
type OKXExchange struct {
	connected bool
}

// NewOKXExchange creates a new OKX exchange connector
func NewOKXExchange() *OKXExchange {
	return &OKXExchange{}
}

// Connect establishes connection to OKX WebSocket
func (o *OKXExchange) Connect(pairs []string) error {
	// TODO: Implement OKX WebSocket connection
	o.connected = true
	return nil
}

// Subscribe starts receiving trade data from OKX
func (o *OKXExchange) Subscribe(tradeCh chan<- models.Trade) error {
	// TODO: Implement OKX trade subscription
	return nil
}

// Disconnect closes the OKX connection
func (o *OKXExchange) Disconnect() error {
	// TODO: Implement OKX disconnection
	o.connected = false
	return nil
}

// Name returns the exchange name
func (o *OKXExchange) Name() string {
	return "okx"
}

// IsConnected returns connection status
func (o *OKXExchange) IsConnected() bool {
	return o.connected
}
