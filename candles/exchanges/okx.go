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

// OKXTradeEvent represents a trade event from OKX WebSocket
type OKXTradeEvent struct {
	Arg struct {
		Channel string `json:"channel"`
		InstID  string `json:"instId"`
	} `json:"arg"`
	Data []struct {
		InstID    string `json:"instId"`
		TradeID   string `json:"tradeId"`
		Price     string `json:"px"`
		Size      string `json:"sz"`
		Side      string `json:"side"`
		Timestamp string `json:"ts"`
	} `json:"data"`
}

// OKXExchange implements the Exchange interface for OKX
type OKXExchange struct {
	conn      *websocket.Conn
	connected bool
	pairs     []string
	eventsCh  chan models.Trade
	mutex     sync.RWMutex
	done      chan struct{}

	// O(1) symbol conversion caches
	canonicalToExchange map[string]string // BTC-USDT -> BTC-USDT-SWAP
	exchangeToCanonical map[string]string // BTC-USDT-SWAP -> BTC-USDT

	// Retry logic components
	retryConfig    RetryConfig
	healthStatus   ConnectionHealth
	recentFailures []time.Time

	// Allow test override of connect function
	connectFunc func([]string) error
}

// NewOKXExchange creates a new OKX exchange connector
func NewOKXExchange() *OKXExchange {
	return &OKXExchange{
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
func (o *OKXExchange) Name() models.ExchangeName {
	return models.ExchangeOKX
}

// SetRetryConfig configures retry behavior
func (o *OKXExchange) SetRetryConfig(config RetryConfig) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.retryConfig = config
}

// GetConnectionHealth returns current connection health status
func (o *OKXExchange) GetConnectionHealth() ConnectionHealth {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	return o.healthStatus
}

// Subscribe starts receiving trade data for specified pairs
func (o *OKXExchange) Subscribe(pairs []string) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.connected {
		return fmt.Errorf("already connected")
	}

	okxPairs := o.convertPairsToOKXFormat(pairs)

	if len(okxPairs) == 0 {
		return fmt.Errorf("no valid pairs to subscribe to")
	}

	// Connect with retry logic
	if err := o.connectWithRetry(okxPairs); err != nil {
		return fmt.Errorf("failed to connect after retries: %w", err)
	}

	o.pairs = pairs
	o.connected = true

	// Start reading messages automatically
	go o.readMessages()

	log.Printf("Successfully connected to OKX WebSocket for pairs: %v", pairs)
	return nil
}

// connectWithRetry implements exponential backoff with storm protection
func (o *OKXExchange) connectWithRetry(okxPairs []string) error {
	// Create backoff strategy
	backoffStrategy := backoff.NewExponentialBackOff()
	backoffStrategy.InitialInterval = o.retryConfig.InitialDelay
	backoffStrategy.MaxInterval = o.retryConfig.MaxDelay
	backoffStrategy.Multiplier = o.retryConfig.BackoffFactor
	backoffStrategy.MaxElapsedTime = 0 // Retry indefinitely until max attempts

	if o.retryConfig.Jitter {
		backoffStrategy.RandomizationFactor = 0.25 // ±25% jitter
	}

	// Limit max attempts
	backoffWithLimit := backoff.WithMaxRetries(backoffStrategy, uint64(o.retryConfig.MaxRetries))

	operation := func() error {
		// Check if we're in storm mode
		if o.isInStormMode() {
			return backoff.Permanent(fmt.Errorf("exchange in storm mode, skipping retry"))
		}

		// Attempt connection
		var err error
		if o.connectFunc != nil {
			err = o.connectFunc(okxPairs)
		} else {
			err = o.connect(okxPairs)
		}

		if err != nil {
			o.recordFailure()
			log.Printf("OKX connection attempt failed (attempt %d): %v", o.healthStatus.RetryAttempt+1, err)
			return err
		}

		o.recordSuccess()
		return nil
	}

	return backoff.Retry(operation, backoffWithLimit)
}

// connect performs the actual WebSocket connection
func (o *OKXExchange) connect(okxPairs []string) error {
	// OKX WebSocket URL
	wsURL := "wss://ws.okx.com:8443/ws/v5/public"

	log.Printf("Connecting to OKX WebSocket: %s", wsURL)

	// Connect to WebSocket
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	headers := http.Header{}
	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		return fmt.Errorf("failed to dial WebSocket: %w", err)
	}

	o.conn = conn

	// Subscribe to trade streams after connection
	return o.subscribeToStreams(okxPairs)
}

// subscribeToStreams sends subscription messages for trading pairs
func (o *OKXExchange) subscribeToStreams(okxPairs []string) error {
	args := make([]map[string]string, len(okxPairs))
	for i, pair := range okxPairs {
		args[i] = map[string]string{
			"channel": "trades",
			"instId":  pair,
		}
	}

	subscribeMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}

	if err := o.conn.WriteJSON(subscribeMsg); err != nil {
		return fmt.Errorf("failed to send subscription message: %w", err)
	}

	log.Printf("Subscribed to OKX pairs: %v", okxPairs)
	return nil
}

// isInStormMode checks if we should enter storm protection mode
func (o *OKXExchange) isInStormMode() bool {
	now := time.Now()
	oneMinuteAgo := now.Add(-time.Minute)

	// Clean up old failures and count recent ones
	recentFailures := 0
	cleanedFailures := make([]time.Time, 0)

	for _, failureTime := range o.recentFailures {
		if failureTime.After(oneMinuteAgo) {
			recentFailures++
			cleanedFailures = append(cleanedFailures, failureTime)
		}
	}

	o.recentFailures = cleanedFailures

	if recentFailures >= o.retryConfig.StormThreshold {
		if !o.healthStatus.InStormMode {
			log.Printf("⚠️ OKX entering storm mode: %d failures in last minute", recentFailures)
			o.healthStatus.InStormMode = true
		}
		return true
	}

	if o.healthStatus.InStormMode {
		log.Printf("✅ OKX exiting storm mode")
		o.healthStatus.InStormMode = false
	}

	return false
}

// recordFailure records a connection failure
func (o *OKXExchange) recordFailure() {
	now := time.Now()
	o.healthStatus.FailureCount++
	o.healthStatus.ConsecutiveFails++
	o.healthStatus.RetryAttempt++
	o.healthStatus.LastFailureTime = now
	o.recentFailures = append(o.recentFailures, now)
}

// recordSuccess records a successful connection
func (o *OKXExchange) recordSuccess() {
	o.healthStatus.ConsecutiveFails = 0
	o.healthStatus.RetryAttempt = 0
	o.healthStatus.InStormMode = false
	log.Printf("✅ OKX connection successful after %d attempts", o.healthStatus.FailureCount)
}

// convertPairsToOKXFormat converts pairs to OKX format
func (o *OKXExchange) convertPairsToOKXFormat(pairs []string) []string {
	okxPairs := make([]string, len(pairs))
	for i, pair := range pairs {
		// OKX uses format: BTC-USDT-SWAP for perpetual contracts
		exchangeSymbol := strings.ToUpper(pair) + "-SWAP"
		o.exchangeToCanonical[exchangeSymbol] = pair
		o.canonicalToExchange[pair] = exchangeSymbol
		okxPairs[i] = exchangeSymbol
	}
	return okxPairs
}

// FromExchangeSymbol converts OKX symbol to canonical format (O(1) lookup)
func (o *OKXExchange) FromExchangeSymbol(exchangeSymbol string) (string, error) {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	canonical, exists := o.exchangeToCanonical[strings.ToUpper(exchangeSymbol)]
	if !exists {
		return "", fmt.Errorf("exchange symbol %s not found in cache", exchangeSymbol)
	}

	return canonical, nil
}

// Disconnect closes the OKX connection
func (o *OKXExchange) Disconnect() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if !o.connected {
		return nil
	}

	// Signal done channel to stop goroutines
	close(o.done)

	// Close WebSocket connection
	if o.conn != nil {
		err := o.conn.Close()
		if err != nil {
			log.Printf("Error closing OKX WebSocket connection: %v", err)
		}
		o.conn = nil
	}

	// Close channels
	close(o.eventsCh)

	o.connected = false
	log.Printf("Disconnected from OKX WebSocket")
	return nil
}

// IsConnected returns connection status
func (o *OKXExchange) IsConnected() bool {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	return o.connected
}

// Events returns the channel for receiving trade events
func (o *OKXExchange) Events() <-chan models.Trade {
	return o.eventsCh
}

// readMessages reads and processes messages from OKX WebSocket
func (o *OKXExchange) readMessages() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("OKX WebSocket reader panic recovered: %v", r)
		}
	}()

	for {
		select {
		case <-o.done:
			log.Printf("OKX WebSocket reader stopping")
			return
		default:
			// Read message from WebSocket
			_, message, err := o.conn.ReadMessage()
			if err != nil {
				// Log error directly - future enhancement: add centralized error handling, metrics, and alerting
				log.Printf("⚠️ OKX WebSocket read error: %v", err)

				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("OKX WebSocket connection closed unexpectedly: %v", err)
				}
				return
			}

			// Process the message
			if err := o.processMessage(message); err != nil {
				log.Printf("Error processing OKX message: %v", err)
			}
		}
	}
}

// processMessage processes a raw WebSocket message
func (o *OKXExchange) processMessage(message []byte) error {
	var tradeEvent OKXTradeEvent
	if err := json.Unmarshal(message, &tradeEvent); err != nil {
		return fmt.Errorf("failed to unmarshal trade message: %w", err)
	}

	// Skip non-trade messages
	if tradeEvent.Arg.Channel != "trades" || len(tradeEvent.Data) == 0 {
		return nil
	}

	// Process each trade in the data array
	for _, trade := range tradeEvent.Data {
		normalizedTrade, err := o.normalizedTradeEvent(trade)
		if err != nil {
			log.Printf("Error converting OKX trade: %v", err)
			continue
		}

		// Send to events channel
		select {
		case o.eventsCh <- normalizedTrade:
		default:
			log.Printf("Events channel full, dropping OKX trade for %s", normalizedTrade.Pair)
		}
	}

	return nil
}

// normalizedTradeEvent converts OKX trade data to normalized Trade
func (o *OKXExchange) normalizedTradeEvent(tradeData struct {
	InstID    string `json:"instId"`
	TradeID   string `json:"tradeId"`
	Price     string `json:"px"`
	Size      string `json:"sz"`
	Side      string `json:"side"`
	Timestamp string `json:"ts"`
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

	// Parse timestamp
	timestampMs, err := strconv.ParseInt(tradeData.Timestamp, 10, 64)
	if err != nil {
		return models.Trade{}, fmt.Errorf("invalid timestamp %s: %w", tradeData.Timestamp, err)
	}

	// Convert symbol back to canonical format
	canonicalSymbol, err := o.FromExchangeSymbol(tradeData.InstID)
	if err != nil {
		return models.Trade{}, fmt.Errorf("failed to convert symbol %s: %w", tradeData.InstID, err)
	}

	// Convert timestamp
	timestamp := time.Unix(0, timestampMs*int64(time.Millisecond))

	return models.Trade{
		Exchange:  models.ExchangeOKX,
		Pair:      canonicalSymbol,
		Price:     price,
		Volume:    volume,
		Timestamp: timestamp,
		TradeID:   tradeData.TradeID,
	}, nil
}
