package ohlc

import (
	"time"

	"github.com/linluma/candle/shared/models"
)

// Builder builds OHLC candles from trades
type Builder struct {
	interval time.Duration
	candles  map[string]*models.Candle // key: pair + start_time
}

// NewBuilder creates a new OHLC builder
func NewBuilder(interval time.Duration) *Builder {
	return &Builder{
		interval: interval,
		candles:  make(map[string]*models.Candle),
	}
}

// AddTrade adds a trade to the appropriate candle
func (b *Builder) AddTrade(trade models.Trade) *models.Candle {
	// TODO: Implement trade addition logic
	// Calculate candle start time based on interval
	// Add trade to existing candle or create new one
	// Return completed candle if interval is finished

	return nil // placeholder
}

// GetIncompleteCandles returns all incomplete candles
func (b *Builder) GetIncompleteCandles() []*models.Candle {
	// TODO: Implement incomplete candle retrieval
	return nil
}
