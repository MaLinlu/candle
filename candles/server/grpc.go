package server

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/linluma/candle/proto"
	"github.com/linluma/candle/shared/models"
)

// CandleServer implements the gRPC CandleService
type CandleServer struct {
	proto.UnimplementedCandleServiceServer
	candleCh    <-chan models.Candle
	mu          sync.RWMutex
	subscribers map[string]*subscriber
	nextID      int
	availablePairs []string
}

// subscriber represents a client subscription
type subscriber struct {
	id     string
	pairs  map[string]bool
	stream proto.CandleService_SubscribeServer
	ctx    context.Context
	cancel context.CancelFunc
}

// NewCandleServer creates a new gRPC candle server
func NewCandleServer(candleCh <-chan models.Candle, availablePairs []string) *CandleServer {
	server := &CandleServer{
		candleCh:    candleCh,
		subscribers: make(map[string]*subscriber),
		nextID:      1,
		availablePairs: availablePairs,
	}

	// Start candle distribution goroutine
	go server.distributeCandles()

	return server
}

// GetAvailablePairs returns the list of available trading pairs
func (s *CandleServer) GetAvailablePairs(ctx context.Context, req *proto.Empty) (*proto.PairsResponse, error) {
	return &proto.PairsResponse{
		Pairs: s.availablePairs,
	}, nil
}

// Subscribe handles client subscriptions to candle updates
func (s *CandleServer) Subscribe(req *proto.CandleRequest, stream proto.CandleService_SubscribeServer) error {
	// Create subscriber
	sub := &subscriber{
		id:     fmt.Sprintf("sub_%d", s.nextID),
		pairs:  make(map[string]bool),
		stream: stream,
	}
	s.nextID++

	// Set up context with cancellation
	sub.ctx, sub.cancel = context.WithCancel(stream.Context())

	// Add pairs to subscription
	for _, pair := range req.Pairs {
		sub.pairs[pair] = true
	}

	// Register subscriber
	s.mu.Lock()
	s.subscribers[sub.id] = sub
	s.mu.Unlock()

	log.Printf("📡 New subscription %s for pairs: %v", sub.id, req.Pairs)

	// Wait for context cancellation (client disconnect or error)
	<-sub.ctx.Done()

	// Clean up subscriber
	s.mu.Lock()
	delete(s.subscribers, sub.id)
	s.mu.Unlock()

	log.Printf("📡 Subscription %s ended", sub.id)
	return nil
}

// distributeCandles distributes candles to all subscribed clients
func (s *CandleServer) distributeCandles() {
	for candle := range s.candleCh {
		// Convert internal candle to protobuf response
		response := s.convertToProto(candle)

		// Send to all subscribers interested in this pair
		s.mu.RLock()
		subscribers := make([]*subscriber, 0, len(s.subscribers))
		for _, sub := range s.subscribers {
			if sub.pairs[candle.Pair] {
				subscribers = append(subscribers, sub)
			}
		}
		s.mu.RUnlock()

		// Send to each subscriber
		for _, sub := range subscribers {
			select {
			case <-sub.ctx.Done():
				// Subscriber disconnected, skip
				continue
			default:
				// Try to send candle
				if err := sub.stream.Send(response); err != nil {
					log.Printf("❌ Failed to send candle to %s: %v", sub.id, err)
					sub.cancel() // Cancel subscription on error
				}
			}
		}
	}
}

// convertToProto converts internal candle to protobuf response
func (s *CandleServer) convertToProto(candle models.Candle) *proto.CandleResponse {
	// Convert exchange contributions
	contributions := make([]*proto.ExchangeContribution, 0, len(candle.Contributions))
	for _, contrib := range candle.Contributions {
		contributions = append(contributions, &proto.ExchangeContribution{
			Exchange:   string(contrib.Exchange),
			Volume:     contrib.Volume,
			TradeCount: contrib.TradeCount,
		})
	}

	return &proto.CandleResponse{
		Pair:          candle.Pair,
		StartTime:     candle.StartTime.Unix(),
		EndTime:       candle.EndTime.Unix(),
		Open:          candle.Open,
		High:          candle.High,
		Low:           candle.Low,
		Close:         candle.Close,
		Volume:        candle.Volume,
		TradeCount:    candle.TradeCount,
		Complete:      candle.Complete,
		Contributions: contributions,
	}
}
