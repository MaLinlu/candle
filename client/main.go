package main

import (
	"log"

	"github.com/linluma/candle/shared/config"
)

func main() {
	cfg := config.ParseClientFlags()

	log.Printf("Starting Client Service...")
	log.Printf("Config: Server=%s, Pairs=%v, Format=%s", cfg.ServerAddress, cfg.Pairs, cfg.Format)

	// TODO: Initialize gRPC client
	// TODO: Subscribe to candle updates
	// TODO: Handle and output candle data

	log.Println("Client service started")
}
