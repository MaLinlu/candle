package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"time"
)

func main() {
	// Parse command line flags
	serverAddr := flag.String("server", "localhost:50051", "gRPC server address")
	pairs := flag.String("pairs", "BTC-USDT,ETH-USDT", "Comma-separated list of trading pairs")
	duration := flag.Duration("duration", 30*time.Second, "Duration to run the demo")
	flag.Parse()

	// Parse pairs
	pairList := strings.Split(*pairs, ",")
	for i, pair := range pairList {
		pairList[i] = strings.TrimSpace(pair)
	}

	log.Printf("🚀 Starting Candle Client Demo")
	log.Printf("📡 Server: %s", *serverAddr)
	log.Printf("📊 Pairs: %v", pairList)
	log.Printf("⏰ Duration: %v", *duration)

	// Run the demo
	if err := demoClient(*serverAddr, pairList, *duration); err != nil {
		log.Fatalf("❌ Demo failed: %v", err)
	}

	log.Println("✅ Demo completed successfully")
}

// demoClient connects to the candles service and subscribes to candle updates
func demoClient(serverAddr string, pairs []string, duration time.Duration) error {
	// Create and connect client
	client := NewCandleClient(serverAddr)
	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	// Get available pairs
	log.Printf("📋 Getting available pairs...")
	availablePairs, err := client.GetAvailablePairs()
	if err != nil {
		return err
	}

	// skip pair if not in available pairs
	for i, pair := range pairs {
		if !availablePairs[pair] {
			log.Printf("❌ Pair %s not found in available pairs, skipping...", pair)
			pairs = append(pairs[:i], pairs[i+1:]...)
		}
	}

	// Subscribe to candles
	log.Printf("📡 Subscribing to pairs: %v", pairs)
	// Set up context - run forever if duration is 0
	var ctx context.Context
	var cancel context.CancelFunc

	if duration == 0 {
		ctx, cancel = context.WithCancel(context.Background())
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), duration)
	}
	defer cancel()

	candleCh, err := client.Subscribe(ctx, pairs)
	if err != nil {
		return err
	}

	if duration == 0 {
		log.Printf("📊 Receiving candles continuously...")
	} else {
		log.Printf("📊 Receiving candles for %v...", duration)
	}

	// Receive and display candles
	for candle := range candleCh {
		DisplayCandle(candle)
	}

	return nil
}
