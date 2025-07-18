package main

import (
	"fmt"
	"log"
	"net"

	"github.com/linluma/candle/shared/config"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.ParseCandlesFlags()

	log.Printf("Starting Candles Service...")
	log.Printf("Config: Interval=%v, Pairs=%v, Port=%d", cfg.Interval, cfg.Pairs, cfg.Port)

	// TODO: Initialize exchanges
	// TODO: Initialize aggregator
	// TODO: Initialize OHLC builder
	// TODO: Initialize gRPC server

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	// TODO: Register candle service

	log.Printf("Candles service listening on port %d", cfg.Port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
