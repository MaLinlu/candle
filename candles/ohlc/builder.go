package ohlc

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/linluma/candle/shared/models"
)

// Builder builds OHLC candles from trades
type Builder struct {
	interval  time.Duration
	tradeCh   <-chan models.Trade
	candleCh  chan models.Candle
	candles   map[string]*models.Candle // key: pair_startTime
	mutex     sync.RWMutex
	done      chan struct{}
	clock     clock.Clock
	startTime time.Time
}

// NewBuilder creates a new OHLC builder
func NewBuilder(interval time.Duration, tradeCh <-chan models.Trade, clk clock.Clock) *Builder {
	return &Builder{
		interval:  interval,
		tradeCh:   tradeCh,
		candleCh:  make(chan models.Candle, 100),
		candles:   make(map[string]*models.Candle),
		done:      make(chan struct{}),
		clock:     clk,
		startTime: clk.Now(),
	}
}

// Start begins processing trades and building candles
func (b *Builder) Start() {
	log.Printf("🕯️ Starting OHLC Builder with %v interval at %v", b.interval, b.startTime)

	// Start candle completion timer
	go b.candleTimer()

	// Process incoming trades
	go func() {
		for {
			select {
			case trade, ok := <-b.tradeCh:
				if !ok {
					log.Println("Trade channel closed, stopping OHLC builder")
					return
				}
				b.processTrade(trade)

			case <-b.done:
				log.Println("OHLC Builder stopped")
				return
			}
		}
	}()
}

// GetCandleChannel returns the channel for receiving completed candles
func (b *Builder) GetCandleChannel() <-chan models.Candle {
	return b.candleCh
}

// Stop gracefully stops the OHLC builder
func (b *Builder) Stop() {
	// Signal goroutines to stop first
	close(b.done)

	// Give a moment for goroutines to finish
	time.Sleep(300 * time.Millisecond)

	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Emit any remaining candles before closing channels
	for _, candle := range b.candles {
		select {
		case b.candleCh <- *candle:
			log.Printf("✅ Final candle %s: O=%.2f H=%.2f L=%.2f C=%.2f V=%.6f T=%d",
				candle.Pair, candle.Open, candle.High, candle.Low, candle.Close, candle.Volume, candle.TradeCount)
		default:
			log.Printf("⚠️ Candle channel full, dropping final candle for %s", candle.Pair)
		}
	}

	close(b.candleCh)
	log.Println("OHLC Builder stopped")
}

// processTrade adds a trade to the appropriate candle
func (b *Builder) processTrade(trade models.Trade) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Calculate candle start time (align to interval boundaries)
	startTime := b.alignToInterval(trade.Timestamp)

	// Ensure the first interval starts after the builder's start time
	// This ensures the first candle is complete
	if startTime.Before(b.startTime) {
		log.Printf("Ignoring trade before builder start time for %s: trade_time=%v, builder_start_time=%v",
			trade.Pair, trade.Timestamp, b.startTime)
		return
	}

	candleKey := fmt.Sprintf("%s_%d", trade.Pair, startTime.Unix())

	// Check if this candle already exists and is complete
	if existingCandle, exists := b.candles[candleKey]; exists && existingCandle.Complete {
		log.Printf("⚠️ Attempting to update completed candle for %s at %v, ignoring trade",
			trade.Pair, startTime)
		return
	}

	// Get or create candle
	candle, exists := b.candles[candleKey]
	if !exists {
		candle = &models.Candle{
			Pair:          trade.Pair,
			StartTime:     startTime,
			EndTime:       startTime.Add(b.interval),
			Open:          trade.Price,
			High:          trade.Price,
			Low:           trade.Price,
			Close:         trade.Price,
			Volume:        0,
			TradeCount:    0,
			Complete:      false,
			Contributions: make([]models.ExchangeContribution, 0),
		}
		b.candles[candleKey] = candle
	}

	// Update candle with trade data
	b.updateCandle(candle, trade)

	log.Printf("📊 Updated candle %s: O=%.2f H=%.2f L=%.2f C=%.2f V=%.6f T=%d",
		trade.Pair, candle.Open, candle.High, candle.Low, candle.Close, candle.Volume, candle.TradeCount)
}

// updateCandle updates candle OHLC data with new trade
func (b *Builder) updateCandle(candle *models.Candle, trade models.Trade) {
	// Update OHLC
	if trade.Price > candle.High {
		candle.High = trade.Price
	}
	if trade.Price < candle.Low {
		candle.Low = trade.Price
	}
	
	candle.Close = trade.Price // Most recent trade becomes close, out of order trade may cause this to be incorrect

	// Update volume and trade count
	candle.Volume += trade.Volume
	candle.TradeCount++

	// Update exchange contributions
	found := false
	for i := range candle.Contributions {
		if candle.Contributions[i].Exchange == trade.Exchange {
			candle.Contributions[i].Volume += trade.Volume
			candle.Contributions[i].TradeCount++
			found = true
			break
		}
	}

	if !found {
		candle.Contributions = append(candle.Contributions, models.ExchangeContribution{
			Exchange:   trade.Exchange,
			Volume:     trade.Volume,
			TradeCount: 1,
		})
	}
}

// candleTimer periodically checks for completed candles
func (b *Builder) candleTimer() {
	ticker := b.clock.Ticker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			b.completeExpiredCandles(now)
		case <-b.done:
			return
		}
	}
}

// completeExpiredCandles marks candles as complete and emits them
func (b *Builder) completeExpiredCandles(now time.Time) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	for key, candle := range b.candles {
		if !candle.Complete && !now.Before(candle.EndTime) {
			candle.Complete = true

			// Emit completed candle
			select {
			case b.candleCh <- *candle:
				log.Printf("✅ Completed candle %s: O=%.2f H=%.2f L=%.2f C=%.2f V=%.6f T=%d",
					candle.Pair, candle.Open, candle.High, candle.Low, candle.Close, candle.Volume, candle.TradeCount)
			default:
				log.Printf("⚠️ Candle channel full, dropping candle for %s", candle.Pair)
			}

			// Remove completed candle from memory
			delete(b.candles, key)
		}
	}
}

// alignToInterval aligns timestamp to interval boundaries
func (b *Builder) alignToInterval(timestamp time.Time) time.Time {
	intervalSeconds := int64(b.interval.Seconds())
	aligned := timestamp.Unix() / intervalSeconds * intervalSeconds
	return time.Unix(aligned, 0).UTC()
}

// GetIncompleteCandles returns all incomplete candles (for debugging)
func (b *Builder) GetIncompleteCandles() []*models.Candle {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	candles := make([]*models.Candle, 0, len(b.candles))
	for _, candle := range b.candles {
		if !candle.Complete {
			candleCopy := *candle
			candles = append(candles, &candleCopy)
		}
	}
	return candles
}
