package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/linluma/candle/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// CandleClient handles gRPC connection to candles service
type CandleClient struct {
	serverAddr string
	conn       *grpc.ClientConn
	client     proto.CandleServiceClient
}

// NewCandleClient creates a new candle client
func NewCandleClient(serverAddr string) *CandleClient {
	return &CandleClient{
		serverAddr: serverAddr,
	}
}

// Connect establishes connection to the candles service
func (c *CandleClient) Connect() error {
	conn, err := grpc.NewClient(c.serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.conn = conn
	c.client = proto.NewCandleServiceClient(conn)
	return nil
}

// Close closes the connection
func (c *CandleClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetAvailablePairs returns available trading pairs
func (c *CandleClient) GetAvailablePairs() (map[string]bool, error) {
	pairsResp, err := c.client.GetAvailablePairs(context.Background(), &proto.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to get available pairs: %w", err)
	}

	pairsMap := make(map[string]bool)
	for _, pair := range pairsResp.Pairs {
		pairsMap[pair] = true
	}
	return pairsMap, nil
}

// Subscribe subscribes to candle updates for given pairs
func (c *CandleClient) Subscribe(ctx context.Context, pairs []string) (<-chan *proto.CandleResponse, error) {
	stream, err := c.client.Subscribe(ctx, &proto.CandleRequest{
		Pairs:             pairs,
		IncludeIncomplete: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	candleCh := make(chan *proto.CandleResponse)

	go func() {
		defer close(candleCh)
		for {
			candle, err := stream.Recv()
			if err != nil {
				log.Printf("Stream receive error: %v", err)
				return
			}
			select {
			case candleCh <- candle:
			case <-ctx.Done():
				return
			}
		}
	}()

	return candleCh, nil
}

// DisplayCandle formats and displays a candle
func DisplayCandle(candle *proto.CandleResponse) {
	// Format timestamp
	startTime := time.Unix(candle.StartTime, 0)
	endTime := time.Unix(candle.EndTime, 0)
	timeRange := fmt.Sprintf("%02d:%02d:%02d-%02d:%02d:%02d",
		startTime.Hour(), startTime.Minute(), startTime.Second(),
		endTime.Hour(), endTime.Minute(), endTime.Second())

	// Format OHLCV
	ohlcv := fmt.Sprintf("O:%.2f H:%.2f L:%.2f C:%.2f V:%.4f",
		candle.Open, candle.High, candle.Low, candle.Close, candle.Volume)

	// Format exchange contributions
	var contributions []string
	for _, contrib := range candle.Contributions {
		contributions = append(contributions, fmt.Sprintf("%s:%.4f", contrib.Exchange, contrib.Volume))
	}
	contribStr := strings.Join(contributions, " ")

	// Choose emoji based on completion status
	emoji := "🟡" // incomplete
	if candle.Complete {
		emoji = "🟢" // complete
	}

	// Display candle
	fmt.Printf("%s %s | %s | %s | %d trades | %s\n",
		emoji, candle.Pair, timeRange, ohlcv, candle.TradeCount, contribStr)
}
