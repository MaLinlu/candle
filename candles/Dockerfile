# Build stage
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the candles service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o candles ./candles

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Create log directory
RUN mkdir -p /app/logs && chown -R appuser:appgroup /app

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/candles .

# Change ownership to non-root user
RUN chown appuser:appgroup candles

# Switch to non-root user
USER appuser

# Expose gRPC port
EXPOSE 50051

# Run the candles service
CMD ["./candles"] 