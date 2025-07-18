package server

import (
	"context"

	"github.com/linluma/candle/shared/models"
)

// CandleServer implements the gRPC CandleService
type CandleServer struct {
	// TODO: Add proto generated interface when available
	candleCh <-chan models.Candle
}

// NewCandleServer creates a new gRPC candle server
func NewCandleServer(candleCh <-chan models.Candle) *CandleServer {
	return &CandleServer{
		candleCh: candleCh,
	}
}

// Subscribe handles client subscriptions to candle updates
func (s *CandleServer) Subscribe(ctx context.Context) error {
	// TODO: Implement subscription logic after proto generation
	return nil
}

// GetAvailablePairs returns available trading pairs
func (s *CandleServer) GetAvailablePairs(ctx context.Context) ([]string, error) {
	// TODO: Implement available pairs logic
	return []string{"BTC-USDT", "ETH-USDT", "SOL-USDT"}, nil
}
