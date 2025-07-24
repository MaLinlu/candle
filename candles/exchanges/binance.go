package exchanges

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gorilla/websocket"
	"github.com/linluma/candle/shared/models"
)

// BinanceTradeEvent represents a trade event from Binance WebSocket
type BinanceTradeEvent struct {
	EventType string `json:"e"` // Event type
	EventTime int64  `json:"E"` // Event time
	Symbol    string `json:"s"` // Symbol
	TradeID   int64  `json:"t"` // Trade ID
	Price     string `json:"p"` // Price
	Quantity  string `json:"q"` // Quantity
	BuyerID   int64  `json:"b"` // Buyer order ID
	SellerID  int64  `json:"a"` // Seller order ID
	TradeTime int64  `json:"T"` // Trade time
	IsMaker   bool   `json:"m"` // Is the buyer the market maker?
}

// BinanceStreamMessage represents the wrapper for Binance stream messages
type BinanceStreamMessage struct {
	Stream string            `json:"stream"`
	Data   BinanceTradeEvent `json:"data"`
}

// BinanceExchange implements the Exchange interface for Binance Futures
type BinanceExchange struct {
	conn      *websocket.Conn
	connected bool
	pairs     []string
	eventsCh  chan models.Trade
	mutex     sync.RWMutex
	done      chan struct{}

	// O(1) symbol conversion caches
	canonicalToExchange map[string]string // BTC-USDT -> BTCUSDT
	exchangeToCanonical map[string]string // BTCUSDT -> BTC-USDT

	// Retry logic components
	retryConfig    RetryConfig
	healthStatus   ConnectionHealth
	recentFailures []time.Time

	// Allow test override of connect function
	connectFunc func([]string) error
}

// NewBinanceExchange creates a new Binance exchange connector
func NewBinanceExchange() *BinanceExchange {
	return &BinanceExchange{
		eventsCh:            make(chan models.Trade, 1000),
		done:                make(chan struct{}),
		canonicalToExchange: make(map[string]string),
		exchangeToCanonical: make(map[string]string),
		recentFailures:      make([]time.Time, 0),
		retryConfig: RetryConfig{
			InitialDelay:   1 * time.Second,
			MaxDelay:       30 * time.Second,
			MaxRetries:     10,
			StormThreshold: 5,
			BackoffFactor:  2.0,
			Jitter:         true,
		},
	}
}

// Name returns the exchange name
func (b *BinanceExchange) Name() models.ExchangeName {
	return models.ExchangeBinance
}

// SetRetryConfig configures retry behavior
func (b *BinanceExchange) SetRetryConfig(config RetryConfig) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.retryConfig = config
}

// GetConnectionHealth returns current connection health status
func (b *BinanceExchange) GetConnectionHealth() ConnectionHealth {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.healthStatus
}

// Subscribe starts receiving trade data for specified pairs
func (b *BinanceExchange) Subscribe(pairs []string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.connected {
		return fmt.Errorf("already connected")
	}

	binancePairs := b.convertPairsToBinanceFormat(pairs)

	if len(binancePairs) == 0 {
		return fmt.Errorf("no valid pairs to subscribe to")
	}

	// Connect with retry logic
	if err := b.connectWithRetry(binancePairs); err != nil {
		return fmt.Errorf("failed to connect after retries: %w", err)
	}

	b.pairs = pairs
	b.connected = true

	// Start reading messages automatically
	go b.readMessages()

	log.Printf("Successfully connected to Binance WebSocket for pairs: %v", pairs)
	return nil
}

// connectWithRetry implements exponential backoff with storm protection
func (b *BinanceExchange) connectWithRetry(binancePairs []string) error {
	// Create backoff strategy
	backoffStrategy := backoff.NewExponentialBackOff()
	backoffStrategy.InitialInterval = b.retryConfig.InitialDelay
	backoffStrategy.MaxInterval = b.retryConfig.MaxDelay
	backoffStrategy.Multiplier = b.retryConfig.BackoffFactor
	backoffStrategy.MaxElapsedTime = 0 // Retry indefinitely until max attempts

	if b.retryConfig.Jitter {
		backoffStrategy.RandomizationFactor = 0.25 // ±25% jitter
	}

	// Limit max attempts
	backoffWithLimit := backoff.WithMaxRetries(backoffStrategy, uint64(b.retryConfig.MaxRetries))

	operation := func() error {
		// Check if we're in storm mode
		if b.isInStormMode() {
			return backoff.Permanent(fmt.Errorf("exchange in storm mode, skipping retry"))
		}

		// Attempt connection
		var err error
		if b.connectFunc != nil {
			err = b.connectFunc(binancePairs)
		} else {
			err = b.connect(binancePairs)
		}

		if err != nil {
			b.recordFailure()
			log.Printf("Binance connection attempt failed (attempt %d): %v", b.healthStatus.RetryAttempt+1, err)
			return err
		}

		b.recordSuccess()
		return nil
	}

	return backoff.Retry(operation, backoffWithLimit)
}

// connect performs the actual WebSocket connection
func (b *BinanceExchange) connect(binancePairs []string) error {
	// Build WebSocket URL with streams
	streams := make([]string, len(binancePairs))
	for i, pair := range binancePairs {
		streams[i] = pair + "@trade"
	}

	streamParam := strings.Join(streams, "/")
	wsURL := fmt.Sprintf("wss://fstream.binance.com/stream?streams=%s", streamParam)

	log.Printf("Connecting to Binance WebSocket: %s", wsURL)

	// Connect to WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	headers := http.Header{}
	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	b.conn = conn
	return nil
}

// isInStormMode checks if we should enter storm protection mode
func (b *BinanceExchange) isInStormMode() bool {
	now := time.Now()
	oneMinuteAgo := now.Add(-time.Minute)

	// Clean up old failures and count recent ones
	recentFailures := 0
	cleanedFailures := make([]time.Time, 0)

	for _, failureTime := range b.recentFailures {
		if failureTime.After(oneMinuteAgo) {
			recentFailures++
			cleanedFailures = append(cleanedFailures, failureTime)
		}
	}

	b.recentFailures = cleanedFailures

	if recentFailures >= b.retryConfig.StormThreshold {
		if !b.healthStatus.InStormMode {
			log.Printf("⚠️ Binance entering storm mode: %d failures in last minute", recentFailures)
			b.healthStatus.InStormMode = true
		}
		return true
	}

	if b.healthStatus.InStormMode {
		log.Printf("✅ Binance exiting storm mode")
		b.healthStatus.InStormMode = false
	}

	return false
}

// recordFailure records a connection failure
func (b *BinanceExchange) recordFailure() {
	now := time.Now()
	b.healthStatus.FailureCount++
	b.healthStatus.ConsecutiveFails++
	b.healthStatus.RetryAttempt++
	b.healthStatus.LastFailureTime = now
	b.recentFailures = append(b.recentFailures, now)
}

// recordSuccess records a successful connection
func (b *BinanceExchange) recordSuccess() {
	b.healthStatus.ConsecutiveFails = 0
	b.healthStatus.RetryAttempt = 0
	b.healthStatus.InStormMode = false
	log.Printf("✅ Binance connection successful after %d attempts", b.healthStatus.FailureCount)
}

// convertPairsToBinanceFormat converts pairs to Binance format
func (b *BinanceExchange) convertPairsToBinanceFormat(pairs []string) []string {
	//TODO: validate pair from api
	binancePairs := make([]string, len(pairs))
	for i, pair := range pairs {
		exchangeSymbol := strings.ToUpper(strings.ReplaceAll(pair, "-", ""))
		b.exchangeToCanonical[exchangeSymbol] = pair
		b.canonicalToExchange[pair] = exchangeSymbol
		binancePairs[i] = strings.ToLower(exchangeSymbol)
	}
	return binancePairs
}

// FromExchangeSymbol converts Binance symbol to canonical format (O(1) lookup)
func (b *BinanceExchange) FromExchangeSymbol(exchangeSymbol string) (string, error) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	canonical, exists := b.exchangeToCanonical[strings.ToUpper(exchangeSymbol)]
	if !exists {
		return "", fmt.Errorf("exchange symbol %s not found in cache", exchangeSymbol)
	}

	return canonical, nil
}

// Disconnect closes the Binance connection
func (b *BinanceExchange) Disconnect() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if !b.connected {
		return nil
	}

	// Signal done channel to stop goroutines
	close(b.done)

	// Close WebSocket connection
	if b.conn != nil {
		err := b.conn.Close()
		if err != nil {
			log.Printf("Error closing Binance WebSocket connection: %v", err)
		}
		b.conn = nil
	}

	// Close channels
	close(b.eventsCh)

	b.connected = false
	log.Printf("Disconnected from Binance WebSocket")
	return nil
}

// IsConnected returns connection status
func (b *BinanceExchange) IsConnected() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.connected
}

// Events returns the channel for receiving trade events
func (b *BinanceExchange) Events() <-chan models.Trade {
	return b.eventsCh
}

// readMessages reads and processes messages from Binance WebSocket
func (b *BinanceExchange) readMessages() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Binance WebSocket reader panic recovered: %v", r)
		}
	}()

	for {
		select {
		case <-b.done:
			log.Printf("Binance WebSocket reader stopping")
			return
		default:
			// Check if connection is still valid
			b.mutex.RLock()
			if b.conn == nil {
				b.mutex.RUnlock()
				log.Printf("Binance WebSocket connection is nil, stopping reader")
				return
			}
			conn := b.conn // Get a local copy while holding the lock
			b.mutex.RUnlock()

			// Read message from WebSocket
			_, message, err := conn.ReadMessage()
			if err != nil {
				// Log error directly - future enhancement: add centralized error handling, metrics, and alerting
				log.Printf("⚠️ Binance WebSocket read error: %v", err)

				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Binance WebSocket connection closed unexpectedly: %v", err)
				}
				return
			}

			// Process the message
			if err := b.processMessage(message); err != nil {
				log.Printf("Error processing Binance message: %v", err)
			}
		}
	}
}

// processMessage processes a raw WebSocket message
func (b *BinanceExchange) processMessage(message []byte) error {
	var streamMessage BinanceStreamMessage
	if err := json.Unmarshal(message, &streamMessage); err != nil {
		return fmt.Errorf("failed to unmarshal stream message: %w", err)
	}

	// Convert to normalized trade
	trade, err := b.normalizedTradeEvent(streamMessage.Data)
	if err != nil {
		return fmt.Errorf("failed to convert trade: %w", err)
	}

	// Send to events channel
	select {
	case b.eventsCh <- trade:
	default:
		log.Printf("Events channel full, dropping Binance trade for %s", trade.Pair)
	}

	return nil
}

// normalizedTradeEvent converts BinanceTradeEvent to normalized Trade
func (b *BinanceExchange) normalizedTradeEvent(event BinanceTradeEvent) (models.Trade, error) {
	// Parse price
	price, err := strconv.ParseFloat(event.Price, 64)
	if err != nil {
		return models.Trade{}, fmt.Errorf("invalid price %s: %w", event.Price, err)
	}

	// Parse volume
	volume, err := strconv.ParseFloat(event.Quantity, 64)
	if err != nil {
		return models.Trade{}, fmt.Errorf("invalid quantity %s: %w", event.Quantity, err)
	}

	// Convert symbol back to canonical format
	canonicalSymbol, err := b.FromExchangeSymbol(event.Symbol)
	if err != nil {
		return models.Trade{}, fmt.Errorf("failed to convert symbol %s: %w", event.Symbol, err)
	}

	// Convert timestamp
	timestamp := time.Unix(0, event.TradeTime*int64(time.Millisecond))

	return models.Trade{
		Exchange:  models.ExchangeBinance,
		Pair:      canonicalSymbol,
		Price:     price,
		Volume:    volume,
		Timestamp: timestamp,
		TradeID:   fmt.Sprintf("%d", event.TradeID),
	}, nil
}
