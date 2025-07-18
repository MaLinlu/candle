# Real-Time Candlestick (OHLC) Service Design

A real-time candlestick (OHLC) service that connects to multiple centralized exchanges (CEXs), aggregates trade data, builds candles per interval, and streams them to clients over gRPC.

## 📐 System Overview

```plaintext
+---------------+     +---------------+     +---------------+
|   Binance     |     |   Coinbase    |     |    okx     |
| (WebSocket)   |     | (WebSocket)   |     | (WebSocket)   |
+-------+-------+     +-------+-------+     +-------+-------+
        |                     |                     |
        \____________________/ ____________________/
                              ↓
                    +-------------------+
                    |  Candles Service  |
                    | - Trade Aggregator|
                    | - OHLC Builder    |
                    | - gRPC Server     |
                    +--------+----------+
                             |
                      +------+------+
                      |   Client    |
                      | (gRPC sub)  |
                      +-------------+
```

## 🎯 Key Requirements

- **3 CEX Integration**: Binance (required), Coinbase, OKX
- **Trading Pairs**: BTC-USDT, ETH-USDT, SOL-USDT
- **Configurable Intervals**: Default 5 seconds, configurable via CLI
- **Real-time Streaming**: gRPC-based candle streaming
- **Consolidation**: Aggregate trade data from all 3 exchanges

## 🏗️ Architecture Components

### 1. Candles Service

**Core Responsibilities:**
- Connect to 3 exchange WebSocket streams simultaneously
- Normalize trade data from different exchange formats
- Consolidate trades across exchanges for each trading pair
- Build OHLC candles per configurable interval
- Stream completed and optionally incomplete candles via gRPC

**Consolidation Strategy:**
- **Volume-Weighted Price**: Use trade volume to weight prices from different exchanges
- **Time Synchronization**: Align trades to UTC timestamps within interval windows
- **Price Consensus**: Calculate OHLC using all trades from all exchanges within the time window

### 2. Client Service

**Core Responsibilities:**
- Subscribe to specific trading pairs via gRPC
- Receive real-time candle updates
- Output formatted candle data to stdout
- Handle connection failures and reconnection

## 📁 Project Structure

```plaintext
candle/
├── candles/                  # Candles service
│   ├── main.go              # Service entrypoint
│   ├── exchanges/           # Exchange connectors
│   │   ├── exchange.go      # Common interface
│   │   ├── binance.go       # Binance WebSocket client
│   │   ├── coinbase.go      # Coinbase WebSocket client
│   │   └── okx.go           # OKX WebSocket client
│   ├── aggregator/          # Trade consolidation logic
│   │   └── consolidator.go  # Cross-exchange trade aggregation
│   ├── ohlc/               # OHLC builder logic
│   │   └── builder.go      # Candle building and intervals
│   ├── server/             # gRPC server implementation
│   │   └── grpc.go         # CandleService gRPC server
│   └── Dockerfile
│
├── client/                  # Client service
│   ├── main.go             # Client entrypoint
│   ├── subscriber/         # gRPC client logic
│   │   └── client.go       # CandleService gRPC client
│   └── Dockerfile
│
├── proto/                   # Shared protobuf definitions
│   ├── candle.proto        # Service and message definitions
│   └── generate.go         # Go protobuf generation
│
├── shared/                  # Common utilities
│   ├── models/             # Data models
│   └── config/             # Configuration handling
│
├── docker-compose.yml       # Multi-service orchestration
├── go.mod                  # Go module definition
├── go.sum                  # Go dependencies
├── Makefile                # Build and development commands
├── README.md               # Project documentation
└── design.md               # This design document
```

## 🔧 Configuration

### Candles Service CLI Flags

| Flag | Description | Default | Example |
|------|-------------|---------|---------|
| `--interval` | Candle interval in seconds | 5 | `--interval=10` |
| `--pairs` | Comma-separated trading pairs | "BTC-USDT,ETH-USDT,SOL-USDT" | `--pairs=BTC-USDT` |
| `--emit-incomplete` | Emit live in-progress candles | false | `--emit-incomplete` |
| `--port` | gRPC server port | 50051 | `--port=50052` |

### Client Service CLI Flags

| Flag | Description | Default | Example |
|------|-------------|---------|---------|
| `--server` | Candles service address | "localhost:50051" | `--server=candles:50051` |
| `--pairs` | Comma-separated pairs to subscribe | "BTC-USDT" | `--pairs=BTC-USDT,ETH-USDT` |
| `--format` | Output format (json/table) | "json" | `--format=table` |

## 🚀 gRPC API Design

### Service Definition (`proto/candle.proto`)

```protobuf
syntax = "proto3";

package candle;

option go_package = "github.com/your-org/candle/proto";

service CandleService {
  // Subscribe to real-time candle updates for specified pairs
  rpc Subscribe(CandleRequest) returns (stream CandleResponse);
  
  // Get available trading pairs
  rpc GetAvailablePairs(Empty) returns (PairsResponse);
}

message CandleRequest {
  repeated string pairs = 1;          // Trading pairs to subscribe to
  bool include_incomplete = 2;        // Include incomplete candles
}

message CandleResponse {
  string pair = 1;                    // Trading pair (e.g., "BTC-USDT")
  int64 start_time = 2;              // Candle start time (Unix timestamp)
  int64 end_time = 3;                // Candle end time (Unix timestamp)
  double open = 4;                   // Opening price
  double high = 5;                   // Highest price
  double low = 6;                    // Lowest price
  double close = 7;                  // Closing price
  double volume = 8;                 // Total volume
  int32 trade_count = 9;             // Number of trades
  bool complete = 10;                // Whether candle is complete
  repeated ExchangeContribution contributions = 11; // Per-exchange breakdown
}

message ExchangeContribution {
  string exchange = 1;               // Exchange name
  double volume = 2;                 // Volume from this exchange
  int32 trade_count = 3;            // Trade count from this exchange
}

message Empty {}

message PairsResponse {
  repeated string pairs = 1;         // Available trading pairs
}
```

## 🔄 Data Flow

### 1. Trade Ingestion
```plaintext
Exchange WebSockets → Trade Normalization → Trade Channel → Consolidator
```

### 2. Consolidation Process
```plaintext
Trade Channel → Time Window Bucketing → Volume-Weighted Aggregation → OHLC Builder
```

### 3. Candle Distribution
```plaintext
OHLC Builder → Candle Channel → gRPC Server → Client Subscribers
```

## 🐳 Docker Configuration

### Development Setup

```bash
# Build and run all services
docker-compose up --build

# Run only candles service
docker-compose up candles

# Scale client service
docker-compose up --scale client=3
```

### Production Considerations

- **Resource Limits**: Set appropriate CPU/memory limits
- **Health Checks**: Implement HTTP health endpoints
- **Logging**: Structured logging with appropriate log levels
- **Monitoring**: Prometheus metrics for trade throughput, latency, and errors

## ⚙️ Implementation Details

### Exchange Integration

Each exchange connector implements a common interface:

```go
type Exchange interface {
    Connect(pairs []string) error
    Subscribe(tradeCh chan<- Trade) error
    Disconnect() error
    Name() string
    IsConnected() bool
}
```

### Trade Consolidation Algorithm

1. **Time Window Alignment**: Group trades by interval start time
2. **Price Calculation**: Use volume-weighted average for OHLC calculations
3. **Volume Aggregation**: Sum volumes across all exchanges
4. **Trade Count**: Count total trades from all sources

### Error Handling

- **Connection Failures**: Automatic reconnection with exponential backoff
- **Data Validation**: Validate trade data before processing
- **Partial Failures**: Continue processing if one exchange fails
- **Circuit Breaker**: Temporarily disable problematic exchanges

## 📊 Monitoring & Observability

### Metrics

- Trade ingestion rate per exchange
- Candle generation latency
- Active gRPC connections
- Error rates and types
- Exchange connection status

### Logging

- Structured JSON logging
- Trade processing events
- Connection state changes
- Error conditions with context

## 🚧 Extensibility

### Adding New Exchanges

1. Implement the `Exchange` interface
2. Add exchange-specific configuration
3. Register in the exchange factory
4. Update Docker configuration

### Scaling Considerations

- **Horizontal Scaling**: Use message queues (Redis/Kafka) for multi-instance deployment
- **Database Integration**: Add persistent storage for historical data
- **Caching**: Implement Redis caching for recent candles
- **Load Balancing**: gRPC load balancing for multiple client connections

## 🔒 Security & Reliability

### API Security

- TLS encryption for gRPC connections
- Rate limiting per client
- Authentication tokens (future enhancement)

### Data Integrity

- Checksum validation for critical data paths
- Duplicate trade detection
- Time drift compensation

### Failure Recovery

- Graceful shutdown handling
- State persistence for critical intervals
- Client reconnection support

## 🧪 Testing Strategy

### Unit Tests
- Exchange connector mocking
- OHLC calculation validation
- Consolidation algorithm testing

### Integration Tests
- End-to-end WebSocket simulation
- gRPC client-server testing
- Docker container orchestration

### Performance Tests
- High-frequency trade simulation
- Memory leak detection
- Concurrent client load testing

## 📋 Development Workflow

### Local Development

```bash
# Generate protobuf code
make proto

# Run tests
make test

# Build services
make build

# Run locally
make run-local
```

### CI/CD Pipeline

1. **Code Quality**: Linting, formatting, security scanning
2. **Testing**: Unit, integration, and performance tests
3. **Building**: Multi-architecture Docker builds
4. **Deployment**: Automated staging deployment

## 🎯 Success Metrics

- **Latency**: Sub-100ms candle generation from trade to client
- **Throughput**: Handle 10,000+ trades per second across all exchanges
- **Reliability**: 99.9% uptime with automatic failover
- **Accuracy**: <0.01% price deviation from individual exchange candles 