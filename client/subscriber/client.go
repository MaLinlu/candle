package subscriber

import (
	"context"
	"log"
)

// Client handles gRPC connection to candles service
type Client struct {
	serverAddress string
}

// NewClient creates a new candle client
func NewClient(serverAddress string) *Client {
	return &Client{
		serverAddress: serverAddress,
	}
}

// Connect establishes connection to the candles service
func (c *Client) Connect() error {
	// TODO: Implement gRPC connection after proto generation
	log.Printf("Connecting to candles service at %s", c.serverAddress)
	return nil
}

// Subscribe subscribes to candle updates for given pairs
func (c *Client) Subscribe(ctx context.Context, pairs []string) error {
	// TODO: Implement subscription logic after proto generation
	log.Printf("Subscribing to pairs: %v", pairs)
	return nil
}

// Close closes the connection
func (c *Client) Close() error {
	// TODO: Implement connection cleanup
	return nil
}
