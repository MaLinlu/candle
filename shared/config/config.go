package config

import (
	"flag"
	"strings"
	"time"
)

// CandlesConfig holds configuration for the candles service
type CandlesConfig struct {
	Interval       time.Duration
	Pairs          []string
	EmitIncomplete bool
	Port           int
}

// ClientConfig holds configuration for the client service
type ClientConfig struct {
	ServerAddress string
	Pairs         []string
	Format        string
}

// ParseCandlesFlags parses command line flags for candles service
func ParseCandlesFlags() *CandlesConfig {
	var (
		interval       = flag.Int("interval", 5, "Candle interval in seconds")
		pairs          = flag.String("pairs", "BTC-USDT,ETH-USDT,SOL-USDT", "Comma-separated trading pairs")
		emitIncomplete = flag.Bool("emit-incomplete", false, "Emit live in-progress candles")
		port           = flag.Int("port", 50051, "gRPC server port")
	)
	flag.Parse()

	return &CandlesConfig{
		Interval:       time.Duration(*interval) * time.Second,
		Pairs:          strings.Split(*pairs, ","),
		EmitIncomplete: *emitIncomplete,
		Port:           *port,
	}
}

// ParseClientFlags parses command line flags for client service
func ParseClientFlags() *ClientConfig {
	var (
		server = flag.String("server", "localhost:50051", "Candles service address")
		pairs  = flag.String("pairs", "BTC-USDT", "Comma-separated pairs to subscribe")
		format = flag.String("format", "json", "Output format (json/table)")
	)
	flag.Parse()

	return &ClientConfig{
		ServerAddress: *server,
		Pairs:         strings.Split(*pairs, ","),
		Format:        *format,
	}
}
