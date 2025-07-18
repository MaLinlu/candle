package aggregator

import (
	"github.com/linluma/candle/shared/models"
)

// Consolidator aggregates trades from multiple exchanges
type Consolidator struct {
	tradeCh  chan models.Trade
	candleCh chan models.Candle
}

// NewConsolidator creates a new trade consolidator
func NewConsolidator() *Consolidator {
	return &Consolidator{
		tradeCh:  make(chan models.Trade, 1000),
		candleCh: make(chan models.Candle, 100),
	}
}

// Start begins the consolidation process
func (c *Consolidator) Start() {
	// TODO: Implement trade consolidation logic
	go c.processTrades()
}

// GetTradeChannel returns the channel for receiving trades
func (c *Consolidator) GetTradeChannel() chan<- models.Trade {
	return c.tradeCh
}

// GetCandleChannel returns the channel for emitting candles
func (c *Consolidator) GetCandleChannel() <-chan models.Candle {
	return c.candleCh
}

// processTrades processes incoming trades and builds candles
func (c *Consolidator) processTrades() {
	// TODO: Implement trade processing and candle building
	for trade := range c.tradeCh {
		_ = trade // placeholder to avoid unused variable error
		// TODO: Process trade and build candles
	}
}
