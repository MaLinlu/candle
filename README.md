# Real-Time Candlestick (OHLC) Service

A real-time candlestick service that connects to multiple cryptocurrency exchanges (Binance, Bybit, OKX), aggregates trade data from perpetual contracts, builds OHLC candles, and streams them to clients via gRPC.

## 🚀 Features

- **Multi-Exchange Integration**: Connects to Binance, Bybit, and OKX simultaneously for perpetual contracts
- **Real-time Data**: WebSocket connections for live trade data
- **Trade Consolidation**: Aggregates trades across exchanges using volume-weighted pricing
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

### Logging

Services use structured JSON logging with configurable levels. Logs are stored in the `logs/` directory:

```bash
# View server logs
tail -f logs/server_test.log

# View client logs  
tail -f logs/client_test.log

# View Docker logs
docker-compose logs -f candles
docker-compose logs -f client
```

### Log Directory Structure

```
logs/
├── server_test.log     # Test server logs
├── client_test.log     # Test client logs
└── ...                 # Other application logs
```

The `logs/` directory is gitignored to prevent committing log files.
# Set log level
export LOG_LEVEL=debug
```

## 🧪 Testing

```bash
# Run all tests
make test

# Run with coverage
go test -cover ./...

```

### 📋 Future Enhancements
- **Production Features**: Monitoring, metrics, circuit breakers
- **Rate Limit Protection**: Automatic detection and handling of exchange rate limits
- **Historical Candle**: Ability to persist and backfill candles from historical data
- **Alerting and Notification**: Real-time alerts for connection loss, abnormal trade activity, or system errors
- **Diverse Asset Types and More Pairs**: Extend support to spot, options, and additional trading pairs beyond the initial set.

## 📄 License

This project is licensed under the MIT License.


## 📞 Support

For questions or issues, please open a GitHub issue. 