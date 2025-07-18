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

# Build the client service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o client ./client

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/client .

# Default command
CMD ["./client", "--server=candles:50051", "--pairs=BTC-USDT", "--format=json"] 