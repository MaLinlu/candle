package models

import "time"

// ExchangeName represents supported exchange names
type ExchangeName string

// Supported exchanges
const (
	ExchangeBinance ExchangeName = "binance"
	ExchangeBybit   ExchangeName = "bybit"
	ExchangeOKX     ExchangeName = "okx"
)

// ConnectionMode represents how an exchange connects to its API
type ConnectionMode int

const (
	WebSocketMode ConnectionMode = iota
	RESTMode                     // Future enhancement
	HybridMode                   // Try WS, fallback to REST
)

// Trade represents a normalized trade from any exchange
type Trade struct {
	Exchange  ExchangeName `json:"exchange"`
	Pair      string       `json:"pair"`
	Price     float64      `json:"price"`
	Volume    float64      `json:"volume"`
	Timestamp time.Time    `json:"timestamp"`
	TradeID   string       `json:"trade_id"`
}

// Candle represents an OHLC candlestick
type Candle struct {
	Pair          string                 `json:"pair"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time"`
	Open          float64                `json:"open"`
	High          float64                `json:"high"`
	Low           float64                `json:"low"`
	Close         float64                `json:"close"`
	Volume        float64                `json:"volume"`
	TradeCount    int32                  `json:"trade_count"`
	Complete      bool                   `json:"complete"`
	Contributions []ExchangeContribution `json:"contributions"`
}

// ExchangeContribution tracks per-exchange contribution to a candle
type ExchangeContribution struct {
	Exchange   ExchangeName `json:"exchange"`
	Volume     float64      `json:"volume"`
	TradeCount int32        `json:"trade_count"`
}
