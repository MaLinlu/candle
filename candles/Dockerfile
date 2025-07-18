FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install protobuf compiler
RUN apk add --no-cache protobuf-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate protobuf code
RUN cd proto && go generate

# Build the candles service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o candles ./candles

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/candles .

# Expose gRPC port
EXPOSE 50051

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50051 || exit 1

# Default command
CMD ["./candles", "--interval=5", "--pairs=BTC-USDT,ETH-USDT,SOL-USDT", "--port=50051"] 