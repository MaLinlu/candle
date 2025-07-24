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

// BybitTradeEvent represents a trade event from Bybit WebSocket
type BybitTradeEvent struct {
	Topic     string `json:"topic"`
	Type      string `json:"type"`
	Timestamp int64  `json:"ts"`
	Data      []struct {
		Symbol    string `json:"s"`
		Price     string `json:"p"`
		Size      string `json:"v"`
		Side      string `json:"S"`
		TradeTime int64  `json:"T"`
		TradeID   string `json:"i"`
	} `json:"data"`
}

// BybitExchange implements the Exchange interface for Bybit
type BybitExchange struct {
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

// NewBybitExchange creates a new Bybit exchange connector
func NewBybitExchange() *BybitExchange {
	return &BybitExchange{
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
func (b *BybitExchange) Name() models.ExchangeName {
	return models.ExchangeBybit
}

// SetRetryConfig configures retry behavior
func (b *BybitExchange) SetRetryConfig(config RetryConfig) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.retryConfig = config
}

// GetConnectionHealth returns current connection health status
func (b *BybitExchange) GetConnectionHealth() ConnectionHealth {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.healthStatus
}

// Subscribe starts receiving trade data for specified pairs
func (b *BybitExchange) Subscribe(pairs []string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.connected {
		return fmt.Errorf("already connected")
	}

	bybitPairs := b.convertPairsToBybitFormat(pairs)

	if len(bybitPairs) == 0 {
		return fmt.Errorf("no valid pairs to subscribe to")
	}

	// Connect with retry logic
	if err := b.connectWithRetry(bybitPairs); err != nil {
		return fmt.Errorf("failed to connect after retries: %w", err)
	}

	b.pairs = pairs
	b.connected = true

	// Start reading messages automatically
	go b.readMessages()

	log.Printf("Successfully connected to Bybit WebSocket for pairs: %v", pairs)
	return nil
}

// connectWithRetry implements exponential backoff with storm protection
func (b *BybitExchange) connectWithRetry(bybitPairs []string) error {
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
			err = b.connectFunc(bybitPairs)
		} else {
			err = b.connect(bybitPairs)
		}

		if err != nil {
			b.recordFailure()
			log.Printf("Bybit connection attempt failed (attempt %d): %v", b.healthStatus.RetryAttempt+1, err)
			return err
		}

		b.recordSuccess()
		return nil
	}

	return backoff.Retry(operation, backoffWithLimit)
}

// connect performs the actual WebSocket connection
func (b *BybitExchange) connect(bybitPairs []string) error {
	// Bybit uses a different WebSocket URL structure
	wsURL := "wss://stream.bybit.com/v5/public/linear"

	log.Printf("Connecting to Bybit WebSocket: %s", wsURL)

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

	// Subscribe to trade streams after connection
	return b.subscribeToStreams(bybitPairs)
}

// subscribeToStreams sends subscription messages for trading pairs
func (b *BybitExchange) subscribeToStreams(bybitPairs []string) error {
	topics := make([]string, len(bybitPairs))
	for i, pair := range bybitPairs {
		topics[i] = fmt.Sprintf("publicTrade.%s", pair)
	}

	subscribeMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": topics,
	}

	if err := b.conn.WriteJSON(subscribeMsg); err != nil {
		return fmt.Errorf("failed to send subscription message: %w", err)
	}

	log.Printf("Subscribed to Bybit topics: %v", topics)
	return nil
}

// isInStormMode checks if we should enter storm protection mode
func (b *BybitExchange) isInStormMode() bool {
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
			log.Printf("⚠️ Bybit entering storm mode: %d failures in last minute", recentFailures)
			b.healthStatus.InStormMode = true
		}
		return true
	}

	if b.healthStatus.InStormMode {
		log.Printf("✅ Bybit exiting storm mode")
		b.healthStatus.InStormMode = false
	}

	return false
}

// recordFailure records a connection failure
func (b *BybitExchange) recordFailure() {
	now := time.Now()
	b.healthStatus.FailureCount++
	b.healthStatus.ConsecutiveFails++
	b.healthStatus.RetryAttempt++
	b.healthStatus.LastFailureTime = now
	b.recentFailures = append(b.recentFailures, now)
}

// recordSuccess records a successful connection
func (b *BybitExchange) recordSuccess() {
	b.healthStatus.ConsecutiveFails = 0
	b.healthStatus.RetryAttempt = 0
	b.healthStatus.InStormMode = false
	log.Printf("✅ Bybit connection successful after %d attempts", b.healthStatus.FailureCount)
}

// convertPairsToBybitFormat converts pairs to Bybit format
func (b *BybitExchange) convertPairsToBybitFormat(pairs []string) []string {
	bybitPairs := make([]string, len(pairs))
	for i, pair := range pairs {
		// Bybit uses same format as Binance: remove hyphen, uppercase
		exchangeSymbol := strings.ToUpper(strings.ReplaceAll(pair, "-", ""))
		b.exchangeToCanonical[exchangeSymbol] = pair
		b.canonicalToExchange[pair] = exchangeSymbol
		bybitPairs[i] = exchangeSymbol // Keep uppercase for Bybit
	}
	return bybitPairs
}

// FromExchangeSymbol converts Bybit symbol to canonical format (O(1) lookup)
func (b *BybitExchange) FromExchangeSymbol(exchangeSymbol string) (string, error) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	canonical, exists := b.exchangeToCanonical[strings.ToUpper(exchangeSymbol)]
	if !exists {
		return "", fmt.Errorf("exchange symbol %s not found in cache", exchangeSymbol)
	}

	return canonical, nil
}

// Disconnect closes the Bybit connection
func (b *BybitExchange) Disconnect() error {
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
			log.Printf("Error closing Bybit WebSocket connection: %v", err)
		}
		b.conn = nil
	}

	// Close channels
	close(b.eventsCh)

	b.connected = false
	log.Printf("Disconnected from Bybit WebSocket")
	return nil
}

// IsConnected returns connection status
func (b *BybitExchange) IsConnected() bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.connected
}

// Events returns the channel for receiving trade events
func (b *BybitExchange) Events() <-chan models.Trade {
	return b.eventsCh
}

// readMessages reads and processes messages from Bybit WebSocket
func (b *BybitExchange) readMessages() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Bybit WebSocket reader panic recovered: %v", r)
		}
	}()

	for {
		select {
		case <-b.done:
			log.Printf("Bybit WebSocket reader stopping")
			return
		default:
			// Check if connection is still valid
			b.mutex.RLock()
			if b.conn == nil {
				b.mutex.RUnlock()
				log.Printf("Bybit WebSocket connection is nil, stopping reader")
				return
			}
			conn := b.conn // Get a local copy while holding the lock
			b.mutex.RUnlock()

			// Read message from WebSocket
			_, message, err := conn.ReadMessage()
			if err != nil {
				// Log error directly - future enhancement: add centralized error handling, metrics, and alerting
				log.Printf("⚠️ Bybit WebSocket read error: %v", err)

				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Bybit WebSocket connection closed unexpectedly: %v", err)
				}
				return
			}

			// Process the message
			if err := b.processMessage(message); err != nil {
				log.Printf("Error processing Bybit message: %v", err)
			}
		}
	}
}

// processMessage processes a raw WebSocket message
func (b *BybitExchange) processMessage(message []byte) error {
	var tradeEvent BybitTradeEvent
	if err := json.Unmarshal(message, &tradeEvent); err != nil {
		return fmt.Errorf("failed to unmarshal trade message: %w", err)
	}

	// Skip non-trade messages
	if tradeEvent.Topic == "" || !strings.HasPrefix(tradeEvent.Topic, "publicTrade.") {
		return nil
	}

	// Process each trade in the data array
	for _, trade := range tradeEvent.Data {
		normalizedTrade, err := b.normalizedTradeEvent(trade)
		if err != nil {
			log.Printf("Error converting Bybit trade: %v", err)
			continue
		}

		// Send to events channel
		select {
		case b.eventsCh <- normalizedTrade:
		default:
			log.Printf("Events channel full, dropping Bybit trade for %s", normalizedTrade.Pair)
		}
	}

	return nil
}

// normalizedTradeEvent converts Bybit trade data to normalized Trade
func (b *BybitExchange) normalizedTradeEvent(tradeData struct {
	Symbol    string `json:"s"`
	Price     string `json:"p"`
	Size      string `json:"v"`
	Side      string `json:"S"`
	TradeTime int64  `json:"T"`
	TradeID   string `json:"i"`
}) (models.Trade, error) {
	// Parse price
	price, err := strconv.ParseFloat(tradeData.Price, 64)
	if err != nil {
		return models.Trade{}, fmt.Errorf("invalid price %s: %w", tradeData.Price, err)
	}

	// Parse volume
	volume, err := strconv.ParseFloat(tradeData.Size, 64)
	if err != nil {
		return models.Trade{}, fmt.Errorf("invalid size %s: %w", tradeData.Size, err)
	}

	// Convert symbol back to canonical format
	canonicalSymbol, err := b.FromExchangeSymbol(tradeData.Symbol)
	if err != nil {
		return models.Trade{}, fmt.Errorf("failed to convert symbol %s: %w", tradeData.Symbol, err)
	}

	// Convert timestamp
	timestamp := time.Unix(0, tradeData.TradeTime*int64(time.Millisecond))

	return models.Trade{
		Exchange:  models.ExchangeBybit,
		Pair:      canonicalSymbol,
		Price:     price,
		Volume:    volume,
		Timestamp: timestamp,
		TradeID:   tradeData.TradeID,
	}, nil
}
