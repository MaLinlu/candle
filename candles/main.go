package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/linluma/candle/candles/exchanges"
	"github.com/linluma/candle/candles/ohlc"
	"github.com/linluma/candle/candles/server"
	"github.com/linluma/candle/proto"
	"github.com/linluma/candle/shared/config"
	"github.com/linluma/candle/shared/models"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.ParseCandlesFlags()

	log.Printf("🚀 Starting Candles Service...")
	log.Printf("📊 Config: Interval=%v, Pairs=%v, Port=%d", cfg.Interval, cfg.Pairs, cfg.Port)

	// Initialize exchange connectors
	connectors := []exchanges.Exchange{
		exchanges.NewBinanceExchange(),
		exchanges.NewBybitExchange(),
		exchanges.NewOKXExchange(),
	}

	// Subscribe to pairs on all exchanges
	log.Printf("📡 Subscribing to pairs: %v", cfg.Pairs)
	for _, ex := range connectors {
		if err := ex.Subscribe(cfg.Pairs); err != nil {
			log.Printf("❌ Failed to subscribe %s: %v", ex.Name(), err)
			continue
		}
		log.Printf("✅ %s connected and subscribed", ex.Name())
	}

	// Central trade collection channel
	tradeCh := make(chan models.Trade, 1000)

	// Launch trade listeners for each exchange
	var wg sync.WaitGroup
	for _, ex := range connectors {
		wg.Add(1)
		go func(e exchanges.Exchange) {
			defer wg.Done()

			// Trade forwarding
			for trade := range e.Events() {
				select {
				case tradeCh <- trade:
					// Trade forwarded to OHLC builder
				default:
					log.Printf("⚠️ Trade channel full, dropping %s trade for %s", e.Name(), trade.Pair)
				}
			}
			log.Printf("📡 %s trade listener stopped", e.Name())
		}(ex)
	}

	// Initialize OHLC Builder
	interval := cfg.Interval
	realClock := clock.New()
	builder := ohlc.NewBuilder(interval, tradeCh, realClock)
	builder.Start()

	// Get candle channel from builder
	candleCh := builder.GetCandleChannel()

	// Initialize gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		log.Fatalf("❌ Failed to listen: %v", err)
	}

	s := grpc.NewServer()

	// Register candle service
	candleServer := server.NewCandleServer(candleCh, cfg.Pairs)
	proto.RegisterCandleServiceServer(s, candleServer)

	// Start gRPC server in goroutine
	go func() {
		log.Printf("🌐 gRPC server listening on port %d", cfg.Port)
		if err := s.Serve(lis); err != nil {
			log.Printf("❌ gRPC server error: %v", err)
		}
	}()

	// Graceful shutdown handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Log system status
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				incompleteCandles := builder.GetIncompleteCandles()
				log.Printf("📈 System Status: %d active candles, %d exchanges connected",
					len(incompleteCandles), len(connectors))

			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for shutdown signal
	<-sigCh
	log.Println("🛑 Shutdown signal received, initiating graceful shutdown...")

	// Stop gRPC server
	s.GracefulStop()

	// Stop OHLC builder
	builder.Stop()

	// Disconnect exchanges
	for _, ex := range connectors {
		if err := ex.Disconnect(); err != nil {
			log.Printf("⚠️ Error disconnecting %s: %v", ex.Name(), err)
		}
	}

	// Close trade channel
	close(tradeCh)

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for cleanup with timeout
	select {
	case <-done:
		log.Println("✅ All goroutines finished")
	case <-time.After(5 * time.Second):
		log.Println("⚠️ Shutdown timeout reached, forcing exit")
	}

	log.Println("👋 Candles Service stopped")
}
