# Real-Time Candlestick (OHLC) Service

A real-time candlestick service that connects to multiple cryptocurrency exchanges (Binance, Coinbase, OKX), aggregates trade data, builds OHLC candles, and streams them to clients via gRPC.

## 🚀 Features

- **Multi-Exchange Integration**: Connects to Binance, Coinbase, and OKX simultaneously
- **Real-time Data**: WebSocket connections for live trade data
- **Trade Consolidation**: Aggregates trades across exchanges using volume-weighted pricing
- **Configurable Intervals**: Build candles for any time interval (default: 5 seconds)
- **gRPC Streaming**: High-performance real-time candle streaming
- **Docker Support**: Full containerization with Docker Compose

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
  --emit-incomplete     Emit live in-progress candles (default: false)
  --port int           gRPC server port (default: 50051)
```

### Client Service Options

```bash
./bin/client [options]

Options:
  --server string      Candles service address (default: "localhost:50051")
  --pairs string       Comma-separated pairs to subscribe (default: "BTC-USDT")
  --format string      Output format: json|table (default: "json")
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
│   ├── main.go             # Service entrypoint
│   ├── exchanges/          # Exchange connectors
│   ├── aggregator/         # Trade consolidation
│   ├── ohlc/              # OHLC candle builder
│   ├── server/            # gRPC server
│   └── Dockerfile
├── client/                 # Client service
│   ├── main.go            # Client entrypoint
│   ├── subscriber/        # gRPC client
│   └── Dockerfile
├── proto/                 # Shared protobuf definitions
├── shared/               # Common utilities
└── docker-compose.yml    # Service orchestration
```

## 📡 API Usage

### gRPC Service Definition

```protobuf
service CandleService {
  rpc Subscribe(CandleRequest) returns (stream CandleResponse);
  rpc GetAvailablePairs(Empty) returns (PairsResponse);
}
```

### Subscribe to Candles

```go
// Connect to service
conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
client := proto.NewCandleServiceClient(conn)

// Subscribe to BTC and ETH
stream, err := client.Subscribe(context.Background(), &proto.CandleRequest{
    Pairs: []string{"BTC-USDT", "ETH-USDT"},
    IncludeIncomplete: false,
})

// Receive candles
for {
    candle, err := stream.Recv()
    if err != nil {
        break
    }
    fmt.Printf("Received candle: %+v\n", candle)
}
```

## 🐳 Docker Deployment

### Production Configuration

```yaml
# docker-compose.prod.yml
version: '3.8'
services:
  candles:
    image: candle/candles:latest
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '1.0'
          memory: 512M
    environment:
      - LOG_LEVEL=warn
```

### Health Checks

Both services include health checks:
- **Candles**: gRPC port connectivity on 50051
- **Client**: Depends on candles service health

## 📊 Monitoring

### Available Metrics

- Trade ingestion rate per exchange
- Candle generation latency
- Active gRPC connections
- Exchange connection status
- Error rates and types

### Logging

Services use structured JSON logging with configurable levels:

```bash
# Set log level
export LOG_LEVEL=debug
```

## 🧪 Testing

```bash
# Run all tests
make test

# Run with coverage
go test -cover ./...

# Integration tests
go test -tags=integration ./...
```

## 🔍 Troubleshooting

### Common Issues

1. **Protocol Buffer compilation fails**
   ```bash
   # Ensure protoc is installed and in PATH
   protoc --version
   
   # Reinstall Go plugins
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```

2. **Exchange connection issues**
   ```bash
   # Check network connectivity
   # Verify exchange API status
   # Review logs for authentication errors
   ```

3. **gRPC connection refused**
   ```bash
   # Ensure candles service is running
   # Check port availability: netstat -ln | grep 50051
   # Verify firewall settings
   ```

## 🚧 Current Status

This is a scaffold implementation. The following components need implementation:

- [ ] Exchange WebSocket connections
- [ ] Trade consolidation algorithm  
- [ ] OHLC candle building logic
- [ ] gRPC server methods
- [ ] Client subscription handling
- [ ] Error handling and reconnection
- [ ] Metrics and monitoring
- [ ] Comprehensive testing

## 📄 License

This project is licensed under the MIT License.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📞 Support

For questions or issues, please open a GitHub issue. 