.PHONY: proto build test run-candles run-client clean docker-build docker-up

# Generate protobuf code
proto:
	cd proto && go generate

# Build all services
build: proto
	go build -o bin/candles ./candles
	go build -o bin/client ./client

# Run tests
test:
	go test -v ./...

# Run candles service locally
run-candles: build
	./bin/candles --interval=5 --pairs=BTC-USDT,ETH-USDT,SOL-USDT

# Run client service locally
run-client: build
	./bin/client -server=localhost:50051 -pairs=BTC-USDT

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f proto/*.pb.go

# Build Docker images
docker-build:
	docker compose build

# Run with Docker Compose
docker-up: docker-build
	docker compose up

# Install dependencies
deps:
	go mod tidy
	go mod download

# Development setup
setup: deps proto

# Run locally (both services)
run-local: build
	./bin/candles --interval=5 --pairs=BTC-USDT,ETH-USDT,SOL-USDT &
	sleep 2
	./bin/client -server=localhost:50051 -pairs=BTC-USDT 