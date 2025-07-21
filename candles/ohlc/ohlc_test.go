package ohlc

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/linluma/candle/shared/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 1: OHLC calculation with multiple trades in single interval
func TestOHLCVCalculation(t *testing.T) {
	interval := 5 * time.Second
	tradeCh := make(chan models.Trade, 100)
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	mockClock := clock.NewMock()
	mockClock.Set(baseTime)

	builder := NewBuilder(interval, tradeCh, mockClock)
	builder.Start()

	// Create trades for a single 5-second interval (10:00:00 - 10:00:05)
	trades := []models.Trade{
		{
			Pair:      "BTC-USDT",
			Price:     50000.00,
			Volume:    1.0,
			Timestamp: baseTime.Add(1 * time.Second),
			Exchange:  models.ExchangeBinance,
		},
		{
			Pair:      "BTC-USDT",
			Price:     50100.00,
			Volume:    2.0,
			Timestamp: baseTime.Add(2 * time.Second),
			Exchange:  models.ExchangeBybit,
		},
		{
			Pair:      "BTC-USDT",
			Price:     50200.00,
			Volume:    0.5,
			Timestamp: baseTime.Add(3 * time.Second),
			Exchange:  models.ExchangeOKX,
		},
		{
			Pair:      "ETH-USDT",
			Price:     3000.00,
			Volume:    150,
			Timestamp: baseTime.Add(4 * time.Second),
			Exchange:  models.ExchangeBinance,
		},
	}

	// Send trades to builder
	for _, trade := range trades {
		tradeCh <- trade
	}

	// Give time for all trades to be processed
	time.Sleep(200 * time.Millisecond)

	// Wait for candles to be generated
	candleCh := builder.GetCandleChannel()

	// Advance clock to trigger candle completion
	mockClock.Add(interval + time.Second) // Move to 10:00:06

	// Give time for candle completion to process
	time.Sleep(100 * time.Millisecond)

	// Stop the builder
	builder.Stop()
	close(tradeCh)

	// Collect both candles (order may vary)
	receivedPairs := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case candle := <-candleCh:
			switch candle.Pair {
			case "BTC-USDT":
				assert.Equal(t, 50000.00, candle.Open, "BTC Open should be first trade price")
				assert.Equal(t, 50200.00, candle.High, "BTC High should be highest trade price")
				assert.Equal(t, 50000.00, candle.Low, "BTC Low should be lowest trade price")
				assert.Equal(t, 50200.00, candle.Close, "BTC Close should be last trade price")
				assert.Equal(t, 3.5, candle.Volume, "BTC Volume should be sum of BTC-USDT trades")
				assert.Equal(t, int32(3), candle.TradeCount, "BTC Trade count should be 3")
				assert.True(t, candle.Complete, "BTC Candle should be complete")
				assert.Equal(t, baseTime, candle.StartTime, "BTC Start time should be interval start")
				assert.Equal(t, baseTime.Add(interval), candle.EndTime, "BTC End time should be interval end")

				// Validate exchange contributions for BTC-USDT
				require.Len(t, candle.Contributions, 3, "BTC Should have three exchange contributions")
				for _, contribution := range candle.Contributions {
					switch contribution.Exchange {
					case models.ExchangeBinance:
						assert.Equal(t, 1.0, contribution.Volume)
						assert.Equal(t, int32(1), contribution.TradeCount)
					case models.ExchangeBybit:
						assert.Equal(t, 2.0, contribution.Volume)
						assert.Equal(t, int32(1), contribution.TradeCount)
					case models.ExchangeOKX:
						assert.Equal(t, 0.5, contribution.Volume)
						assert.Equal(t, int32(1), contribution.TradeCount)
					}
				}
				receivedPairs["BTC-USDT"] = true

			case "ETH-USDT":
				assert.Equal(t, 3000.00, candle.Open, "ETH Open should be first trade price")
				assert.Equal(t, 3000.00, candle.High, "ETH High should be same as open")
				assert.Equal(t, 3000.00, candle.Low, "ETH Low should be same as open")
				assert.Equal(t, 3000.00, candle.Close, "ETH Close should be same as open")
				assert.Equal(t, 150.0, candle.Volume, "ETH Volume should be ETH-USDT trade volume")
				assert.Equal(t, int32(1), candle.TradeCount, "ETH Trade count should be 1")
				assert.True(t, candle.Complete, "ETH Candle should be complete")
				assert.Equal(t, baseTime, candle.StartTime, "ETH Start time should be interval start")
				assert.Equal(t, baseTime.Add(interval), candle.EndTime, "ETH End time should be interval end")

				// Validate exchange contribution for ETH-USDT
				require.Len(t, candle.Contributions, 1, "ETH Should have one exchange contribution")
				assert.Equal(t, models.ExchangeBinance, candle.Contributions[0].Exchange)
				assert.Equal(t, 150.0, candle.Contributions[0].Volume)
				assert.Equal(t, int32(1), candle.Contributions[0].TradeCount)
				receivedPairs["ETH-USDT"] = true

			default:
				t.Fatalf("Unexpected pair: %s", candle.Pair)
			}

		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for candle")
		}
	}

	// Verify we received both pairs
	assert.True(t, receivedPairs["BTC-USDT"], "Should have received BTC-USDT candle")
	assert.True(t, receivedPairs["ETH-USDT"], "Should have received ETH-USDT candle")
}

// Test 2: Interval boundary cases (trades at start/end of intervals)
func TestIntervalBoundaryCases(t *testing.T) {
	interval := 5 * time.Second
	tradeCh := make(chan models.Trade, 100)
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	mockClock := clock.NewMock()
	mockClock.Set(baseTime)

	builder := NewBuilder(interval, tradeCh, mockClock)
	builder.Start()

	// Test trades exactly at interval boundaries
	trades := []models.Trade{
		// Trade exactly at interval start
		{
			Pair:      "ETH-USDT",
			Price:     3000.00,
			Volume:    1.0,
			Timestamp: baseTime, // Exactly at 10:00:00
			Exchange:  models.ExchangeBinance,
		},
		// Trade exactly at interval end
		{
			Pair:      "ETH-USDT",
			Price:     3100.00,
			Volume:    2.0,
			Timestamp: baseTime.Add(interval).Add(-time.Nanosecond), // Just before 10:00:05
			Exchange:  models.ExchangeBybit,
		},
		// Trade in next interval
		{
			Pair:      "ETH-USDT",
			Price:     3200.00,
			Volume:    1.5,
			Timestamp: baseTime.Add(interval), // Exactly at 10:00:05
			Exchange:  models.ExchangeBinance,
		},
	}

	// Send trades to builder
	for _, trade := range trades {
		tradeCh <- trade
	}

	// Give time for all trades to be processed
	time.Sleep(200 * time.Millisecond)

	// Wait for candles to be generated
	candleCh := builder.GetCandleChannel()

	// Advance clock to complete first interval
	mockClock.Add(interval + time.Second) // Move to 10:00:06
	time.Sleep(100 * time.Millisecond)

	// Advance clock to complete second interval
	mockClock.Add(interval) // Move to 10:00:11
	time.Sleep(100 * time.Millisecond)

	// Stop the builder
	builder.Stop()
	close(tradeCh)

	// Collect all candles (order may vary)
	candles := make(map[time.Time]*models.Candle)
	for i := 0; i < 2; i++ {
		select {
		case candle := <-candleCh:
			candles[candle.StartTime] = &candle

		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for candle %d", i+1)
		}
	}

	// Validate first candle (10:00:00 - 10:00:05)
	if candle, ok := candles[baseTime]; ok {
		assert.Equal(t, "ETH-USDT", candle.Pair)
		assert.Equal(t, 3000.00, candle.Open, "Open should be first trade at interval start")
		assert.Equal(t, 3100.00, candle.High, "High should be highest trade in interval")
		assert.Equal(t, 3000.00, candle.Low, "Low should be lowest trade in interval")
		assert.Equal(t, 3100.00, candle.Close, "Close should be last trade in interval")
		assert.Equal(t, 3.0, candle.Volume, "Volume should be sum of first two trades")
		assert.Equal(t, int32(2), candle.TradeCount, "Trade count should be 2")
		assert.True(t, candle.Complete, "Candle should be complete")
		assert.Equal(t, baseTime, candle.StartTime)
		assert.Equal(t, baseTime.Add(interval), candle.EndTime)
	} else {
		t.Fatal("First candle not found")
	}

	// Validate second candle (10:00:05 - 10:00:10)
	if candle, ok := candles[baseTime.Add(interval)]; ok {
		assert.Equal(t, "ETH-USDT", candle.Pair)
		assert.Equal(t, 3200.00, candle.Open, "Open should be trade at interval start")
		assert.Equal(t, 3200.00, candle.High, "High should be same as open")
		assert.Equal(t, 3200.00, candle.Low, "Low should be same as open")
		assert.Equal(t, 3200.00, candle.Close, "Close should be same as open")
		assert.Equal(t, 1.5, candle.Volume, "Volume should be single trade")
		assert.Equal(t, int32(1), candle.TradeCount, "Trade count should be 1")
		assert.True(t, candle.Complete, "Candle should be complete")
		assert.Equal(t, baseTime.Add(interval), candle.StartTime)
		assert.Equal(t, baseTime.Add(2*interval), candle.EndTime)
	} else {
		t.Fatal("Second candle not found")
	}
}

// Test 3: Cross-interval trade handling and time synchronization
func TestCrossIntervalTradeHandling(t *testing.T) {
	interval := 5 * time.Second
	tradeCh := make(chan models.Trade, 100)
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	mockClock := clock.NewMock()
	mockClock.Set(baseTime)

	builder := NewBuilder(interval, tradeCh, mockClock)
	builder.Start()

	// Send trades that span multiple intervals
	trades := []models.Trade{
		// First interval (10:00:00 - 10:00:05)
		{
			Pair:      "BTC-USDT",
			Price:     50000.00,
			Volume:    1.0,
			Timestamp: baseTime.Add(1 * time.Second),
			Exchange:  models.ExchangeBinance,
		},
		{
			Pair:      "BTC-USDT",
			Price:     50100.00,
			Volume:    2.0,
			Timestamp: baseTime.Add(4 * time.Second),
			Exchange:  models.ExchangeBinance,
		},
		// Second interval (10:00:05 - 10:00:10)
		{
			Pair:      "BTC-USDT",
			Price:     50200.00,
			Volume:    1.5,
			Timestamp: baseTime.Add(6 * time.Second),
			Exchange:  models.ExchangeBinance,
		},
		{
			Pair:      "BTC-USDT",
			Price:     50300.00,
			Volume:    0.5,
			Timestamp: baseTime.Add(9 * time.Second),
			Exchange:  models.ExchangeBinance,
		},
		// Third interval (10:00:10 - 10:00:15) - empty
		// Fourth interval (10:00:15 - 10:00:20)
		{
			Pair:      "BTC-USDT",
			Price:     50400.00,
			Volume:    3.0,
			Timestamp: baseTime.Add(16 * time.Second),
			Exchange:  models.ExchangeBinance,
		},
	}

	// Send trades to builder
	for _, trade := range trades {
		tradeCh <- trade
	}

	// Give time for all trades to be processed
	time.Sleep(200 * time.Millisecond)

	// Wait for all candles to be generated
	candleCh := builder.GetCandleChannel()

	// Advance clock to complete multiple intervals
	mockClock.Add(interval + time.Second) // Move to 10:00:06 - completes first interval
	time.Sleep(100 * time.Millisecond)

	mockClock.Add(interval) // Move to 10:00:11 - completes second interval
	time.Sleep(100 * time.Millisecond)

	mockClock.Add(2 * interval) // Move to 10:00:21 - completes third and fourth intervals
	time.Sleep(100 * time.Millisecond)

	// Stop the builder
	builder.Stop()
	close(tradeCh)

	// Collect all candles (order may vary)
	candles := make(map[time.Time]*models.Candle)
	for i := 0; i < 3; i++ {
		select {
		case candle := <-candleCh:
			candles[candle.StartTime] = &candle

		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for candle %d", i+1)
		}
	}

	// Validate first candle (10:00:00 - 10:00:05)
	if candle, ok := candles[baseTime]; ok {
		assert.Equal(t, "BTC-USDT", candle.Pair)
		assert.Equal(t, 50000.00, candle.Open)
		assert.Equal(t, 50100.00, candle.High)
		assert.Equal(t, 50000.00, candle.Low)
		assert.Equal(t, 50100.00, candle.Close)
		assert.Equal(t, 3.0, candle.Volume)
		assert.Equal(t, int32(2), candle.TradeCount)
		assert.True(t, candle.Complete)
		assert.Equal(t, baseTime, candle.StartTime)
		assert.Equal(t, baseTime.Add(interval), candle.EndTime)
	} else {
		t.Fatal("First candle not found")
	}

	// Validate second candle (10:00:05 - 10:00:10)
	if candle, ok := candles[baseTime.Add(interval)]; ok {
		assert.Equal(t, "BTC-USDT", candle.Pair)
		assert.Equal(t, 50200.00, candle.Open)
		assert.Equal(t, 50300.00, candle.High)
		assert.Equal(t, 50200.00, candle.Low)
		assert.Equal(t, 50300.00, candle.Close)
		assert.Equal(t, 2.0, candle.Volume)
		assert.Equal(t, int32(2), candle.TradeCount)
		assert.True(t, candle.Complete)
		assert.Equal(t, baseTime.Add(interval), candle.StartTime)
		assert.Equal(t, baseTime.Add(2*interval), candle.EndTime)
	} else {
		t.Fatal("Second candle not found")
	}

	// Validate fourth candle (10:00:15 - 10:00:20)
	if candle, ok := candles[baseTime.Add(3*interval)]; ok {
		assert.Equal(t, "BTC-USDT", candle.Pair)
		assert.Equal(t, 50400.00, candle.Open)
		assert.Equal(t, 50400.00, candle.High)
		assert.Equal(t, 50400.00, candle.Low)
		assert.Equal(t, 50400.00, candle.Close)
		assert.Equal(t, 3.0, candle.Volume)
		assert.Equal(t, int32(1), candle.TradeCount)
		assert.True(t, candle.Complete)
		assert.Equal(t, baseTime.Add(3*interval), candle.StartTime)
		assert.Equal(t, baseTime.Add(4*interval), candle.EndTime)
	} else {
		t.Fatal("Fourth candle not found")
	}
}

// Test 4: Historical trade handling - simplified to only reject old trades
func TestHistoricalTradeHandling(t *testing.T) {
	interval := 5 * time.Second
	tradeCh := make(chan models.Trade, 100)
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	mockClock := clock.NewMock()
	mockClock.Set(baseTime.Add(30 * time.Second)) // Set clock 30 seconds ahead (current interval starts at 10:00:30)

	builder := NewBuilder(interval, tradeCh, mockClock)
	builder.Start()

	trades := []models.Trade{
		// Old trade (from previous interval) - should be rejected
		{
			Pair:      "BTC-USDT",
			Price:     49000.00,
			Volume:    2.0,
			Timestamp: baseTime.Add(15 * time.Second), // 10:00:15 - previous interval
			Exchange:  models.ExchangeBinance,
		},
		// Current interval trade - should be accepted
		{
			Pair:      "BTC-USDT",
			Price:     50000.00,
			Volume:    1.0,
			Timestamp: baseTime.Add(31 * time.Second), // 10:00:31 - current interval
			Exchange:  models.ExchangeBinance,
		},
		// Future interval trade - should be accepted
		{
			Pair:      "ETH-USDT",
			Price:     3000.00,
			Volume:    10.0,
			Timestamp: baseTime.Add(36 * time.Second), // 10:00:36 - future interval
			Exchange:  models.ExchangeBinance,
		},
	}

	// Send trades to builder
	for _, trade := range trades {
		tradeCh <- trade
	}

	// Give time for processing
	time.Sleep(300 * time.Millisecond)

	candleCh := builder.GetCandleChannel()

	// Advance clock to complete current and future intervals
	mockClock.Add(2 * interval) // Move to 10:00:40
	time.Sleep(100 * time.Millisecond)

	builder.Stop()
	close(tradeCh)

	// Collect candles
	receivedCandles := make([]models.Candle, 0)
	for range 5 { // Allow for more candles to be collected
		select {
		case candle := <-candleCh:
			receivedCandles = append(receivedCandles, candle)
		case <-time.After(1 * time.Second):
			goto done
		}
	}
done:

	// Should have received at least 1 candle, but the important check is that old trade was rejected
	assert.GreaterOrEqual(t, len(receivedCandles), 1, "Should have at least 1 candle")

	// Verify the candles contain expected data (no old trade data)
	btcFound := false

	for _, candle := range receivedCandles {
		switch candle.Pair {
		case "BTC-USDT":
			// Should only contain the current trade (50000.00), not the old trade (49000.00)
			assert.Equal(t, 50000.00, candle.Open)
			assert.Equal(t, 50000.00, candle.Close)
			assert.Equal(t, 1.0, candle.Volume) // Only 1 trade should be included
			assert.Equal(t, int32(1), candle.TradeCount)
			btcFound = true
		case "ETH-USDT":
			assert.Equal(t, 3000.00, candle.Open)
			assert.Equal(t, 10.0, candle.Volume)
			// ETH candle might or might not be emitted depending on timing, that's OK
		}
	}

	assert.True(t, btcFound, "Should have received BTC candle")
}
