# Real-Time Candlestick (OHLC) Service

A real-time candlestick service that connects to multiple cryptocurrency exchanges (Binance, Bybit, OKX), aggregates trade data from perpetual contracts, builds OHLC candles, and streams them to clients via gRPC.

## 🚀 Features

- **Multi-Exchange Integration**: Connects to Binance, Bybit, and OKX simultaneously for perpetual contracts
- **Real-time Data**: WebSocket connections for live trade data
- **Trade Consolidation**: Aggregates trades across exchanges
- **Configurable Intervals**: Build candles for any time interval (default: 5 seconds)
- **gRPC Streaming**: High-performance real-time candle streaming
- **Retry with Backoff and Storm Control**: Robust reconnection logic with exponential backoff and storm protection for exchange connections
- **Volume and Decomposition by Exchange**: Candle data includes per-exchange volume and trade breakdowns for transparency and analytics
- **Docker Support**: Full containerization with Docker Compose
- **Support for Diverse Asset Types and Additional Pairs**: The architecture is designed to be extensible for spot, futures, and other asset types, as well as for adding more trading pairs in the future.

## 📝 Assumptions

- **Perpetual Contracts Only**: The service assumes all integrated exchanges and pairs are perpetual futures (perps).
- **Top 3 Volume Perps CEXs**: Binance, Bybit, and OKX are chosen as the top 3 by perpetual contract trading volume.
- **First Incomplete Interval Ignored**: The first interval after service start is ignored if incomplete; only fully-formed candles are emitted.
- **No Candle if No Trade**: If there are no trades for a pair in an interval, no candle is emitted for that interval.

## 🏛️ System Architecture Overview

The service follows a multi-layer architecture for real-time candlestick generation:

```
┌─────────────────────────────────────────────────────────────┐
│                    Client Applications                       │
└──────────────────────┬──────────────────────────────────────┘
                       │ gRPC Streaming
┌─────────────────────────────────────────────────────────────┐
│                   Candles Service                           │
│  ┌─────────────┐    ┌─────────────┐                        │
│  │ gRPC Server │    │ OHLC Builder│                        │
│  │             │◄───┤             │ ◄───                   |
│  └─────────────┘    └─────────────┘      |                  │
└──────────────────────────────────────────┬──────────────────┘
                                           │ Trade Channel
┌─────────────────────────────────────────────────────────────┐
│                Exchange Connectors                          │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐      │
│  │   Binance   │    │    Bybit    │    │     OKX     │      │
│  │  WebSocket  │    │  WebSocket  │    │  WebSocket  │      │
│  └─────────────┘    └─────────────┘    └─────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

### Trade Flow

1. **Exchange Connection**: WebSocket connections to Binance, Bybit, and OKX perpetual contracts
2. **Trade Normalization**: Raw trade data is converted to standardized format with pair mapping
3. **Trade Aggregation**: All trades are routed through a central trade channel
4. **OHLC Building**: Trades are grouped by time intervals and converted to OHLC candles
5. **Candle Completion**: Candles are marked complete at interval boundaries and streamed to clients
6. **gRPC Streaming**: Completed candles are delivered to subscribed clients in real-time

## 📊 Output Example

```json
{
  "pair": "BTC-USDT",
  "start_time": "2024-01-15T10:05:00Z",
  "end_time": "2024-01-15T10:05:05Z",
  "open": 42350.50,
  "high": 42375.20,
  "low": 42340.10,
  "close": 42368.75,
  "volume": 15.247689,
  "trade_count": 127,
  "complete": true,
  "contributions": [
    {
      "exchange": "binance",
      "volume": 8.123456,
      "trade_count": 68
    },
    {
      "exchange": "bybit", 
      "volume": 4.567890,
      "trade_count": 35
    },
    {
      "exchange": "okx",
      "volume": 2.556343,
      "trade_count": 24
    }
  ]
}
```

## 📋 Prerequisites

- **Go 1.21+**
- **Protocol Buffers compiler (protoc)**
- **Docker & Docker Compose** (for containerized deployment)

### Install Protocol Buffers

```bash
# macOS
brew install protobuf

# Ubuntu/Debian
apt install -y protobuf-compiler

# Install Go plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

## 🛠️ Quick Start

### 1. Clone and Setup

```bash
git clone https://github.com/linluma/candle.git
cd candle
make setup
```

### 2. Run with Docker Compose (Recommended)

```bash
# Build and start all services
make docker-up

# Or manually
docker-compose up --build
```

### 3. Run Locally

```bash
# Terminal 1: Start candles service
make run-candles

# Terminal 2: Start client
make run-client
```

## 🔧 Configuration

### Candles Service Options

```bash
./bin/candles [options]

Options:
  --interval int        Candle interval in seconds (default: 5)
  --pairs string        Comma-separated trading pairs (default: "BTC-USDT,ETH-USDT,SOL-USDT")
  --port int           gRPC server port (default: 50051)
```

### Client Service Options

```bash
./bin/client [options]

Options:
  -server string       Candles service address (default: "localhost:50051")
  -pairs string        Comma-separated pairs to subscribe (default: "BTC-USDT,ETH-USDT")
  -duration duration   Duration to run the demo (default: 30s)
```

## 🏗️ Development

### Build Commands

```bash
# Generate protobuf code
make proto

# Build all services
make build

# Run tests
make test

# Clean build artifacts
make clean
```

### Project Structure

```
candle/
├── candles/                 # Candles service
│   ├── main.go             # Service entrypoint with direct orchestration
│   ├── exchanges/          # Exchange connectors with self-contained symbol conversion
│   ├── ohlc/              # OHLC candle builder with direct trade processing
│   ├── server/            # gRPC server
│   └── Dockerfile
├── client/                 # Client service
│   ├── main.go            # Client entrypoint
│   └── Dockerfile
├── proto/                 # Shared protobuf definitions
├── shared/               # Common utilities and models
├── logs/                 # Application logs (gitignored)
├── bin/                  # Built binaries (gitignored)
└── docker-compose.yml    # Service orchestration
```


### gRPC Service Definition

```protobuf
service CandleService {
  rpc Subscribe(CandleRequest) returns (stream CandleResponse);
  rpc GetAvailablePairs(Empty) returns (PairsResponse);
}
```

### Health Checks

Both services include health checks:
- **Candles**: gRPC port connectivity on 50051
- **Client**: Depends on candles service health


## 🧪 Testing

```bash
# Run all tests
make test

# Run with coverage
go test -cover ./...

```
## ⚠️ Current Limitations

- **No Persistence**: Candles are not stored; only real-time streaming is supported
- **No Historical Replays**: Cannot backfill or replay historical candle data
- **No Horizontal Scaling**: Single-instance deployment without Redis/Kafka buffering
- **No Built-in Monitoring**: No metrics, health dashboards, or observability tools
- **Memory-Only State**: All incomplete candles are stored in memory and lost on restart
- **No Rate Limit Handling**: Exchange rate limits may cause connection drops
- **Out-of-Order Trades**: Late-arriving trades may affect candle accuracy (close price)


## 🚀 Future Enhancements

- **Monitoring & Observability**: Implement Prometheus metrics, Grafana dashboards, and distributed tracing to enhance system visibility and performance monitoring.
- **Scalability**: Introduce horizontal scaling with Redis/Kafka-based message buffering to support multi-instance deployments and improve load distribution.
- **Data Persistency & Replay**: Integrate a database solution like PostgreSQL with TimescaleDB or ClickHouse for candle persistence, enabling historical data queries and replay capabilities.
- **Edge Case Handling**: Develop robust mechanisms to handle missing, out-of-order, and empty candles, ensuring data accuracy and reliability in all scenarios.


## 📄 License

This project is licensed under the MIT License.


## 📞 Support

For questions or issues, please open a GitHub issue. 