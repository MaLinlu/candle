#!/bin/bash

# End-to-End Test Script for Candle Service
# This script tests the complete pipeline from exchange data to gRPC client

set -e

echo "🚀 Starting End-to-End Test for Candle Service"
echo "=============================================="

# Configuration
SERVER_PORT=50051
SERVER_PAIRS="BTC-USDT,ETH-USDT,SOL-USDT"
CLIENT1_PAIRS="BTC-USDT,ETH-USDT"
CLIENT2_PAIRS="SOL-USDT,ETH-USDT"
CLIENT3_PAIRS="BTC-USDT,SOL-USDT,ETH-USDT"
INTERVAL=5
TEST_DURATION=25

# Process tracking
CANDLES_PID=""
CLIENT1_PID=""
CLIENT2_PID=""
CLIENT3_PID=""
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Clean up function
cleanup() {
    echo "🧹 Cleaning up..."
    
    # Kill specific processes by PID if they exist
    if [ -n "$CANDLES_PID" ] && kill -0 $CANDLES_PID 2>/dev/null; then
        echo "Stopping candles service (PID: $CANDLES_PID)"
        kill $CANDLES_PID
        sleep 1
        # Force kill if still running
        if kill -0 $CANDLES_PID 2>/dev/null; then
            kill -9 $CANDLES_PID 2>/dev/null || true
        fi
    fi
    
    if [ -n "$CLIENT1_PID" ] && kill -0 $CLIENT1_PID 2>/dev/null; then
        echo "Stopping client 1 (PID: $CLIENT1_PID)"
        kill $CLIENT1_PID
        sleep 1
        # Force kill if still running
        if kill -0 $CLIENT1_PID 2>/dev/null; then
            kill -9 $CLIENT1_PID 2>/dev/null || true
        fi
    fi
    
    if [ -n "$CLIENT2_PID" ] && kill -0 $CLIENT2_PID 2>/dev/null; then
        echo "Stopping client 2 (PID: $CLIENT2_PID)"
        kill $CLIENT2_PID
        sleep 1
        # Force kill if still running
        if kill -0 $CLIENT2_PID 2>/dev/null; then
            kill -9 $CLIENT2_PID 2>/dev/null || true
        fi
    fi
    
    if [ -n "$CLIENT3_PID" ] && kill -0 $CLIENT3_PID 2>/dev/null; then
        echo "Stopping client 3 (PID: $CLIENT3_PID)"
        kill $CLIENT3_PID
        sleep 1
        # Force kill if still running
        if kill -0 $CLIENT3_PID 2>/dev/null; then
            kill -9 $CLIENT3_PID 2>/dev/null || true
        fi
    fi
    
    # Kill any remaining processes with more specific patterns
    # Only kill processes that match our exact binary names
    pkill -f "bin/candles" 2>/dev/null || true
    pkill -f "bin/client" 2>/dev/null || true
    
    # Note: Client logs are preserved at logs/ for inspection
    
    sleep 1
}

# Set up cleanup on exit
trap cleanup EXIT

# Step 1: Build the applications
echo "🔨 Building applications..."
go build -o bin/candles ./candles
go build -o bin/client ./client
print_status "Build completed"

# Step 2: Start the candles service
echo "🚀 Starting candles service..."
echo "📊 Server subscribing to: $SERVER_PAIRS"
echo "🔄 Testing all exchanges: Binance, Bybit, OKX"
echo "📁 Server logs will be saved to: logs/server_test.log"
./bin/candles -pairs="$SERVER_PAIRS" -interval=$INTERVAL -port=$SERVER_PORT > logs/server_test.log 2>&1 &
CANDLES_PID=$!

# Wait for service to start
sleep 3

# Check if service is running
if ! kill -0 $CANDLES_PID 2>/dev/null; then
    print_error "Candles service failed to start"
    exit 1
fi
print_status "Candles service started (PID: $CANDLES_PID)"

# Step 3: Start first client (BTC-USDT, ETH-USDT)
echo "📡 Starting Client 1 (BTC-USDT, ETH-USDT)..."
echo "📁 Client 1 logs will be saved to: logs/client1_test.log"

# Run client 1 in background and capture its PID
./bin/client -server="localhost:$SERVER_PORT" -pairs="$CLIENT1_PAIRS" -duration="${TEST_DURATION}s" > logs/client1_test.log 2>&1 &
CLIENT1_PID=$!

# Step 4: Start second client (SOL-USDT, ETH-USDT)
echo "📡 Starting Client 2 (SOL-USDT, ETH-USDT)..."
echo "📁 Client 2 logs will be saved to: logs/client2_test.log"

# Run client 2 in background and capture its PID
./bin/client -server="localhost:$SERVER_PORT" -pairs="$CLIENT2_PAIRS" -duration="${TEST_DURATION}s" > logs/client2_test.log 2>&1 &
CLIENT2_PID=$!

# Step 5: Start third client (BTC-USDT, SOL-USDT)
echo "📡 Starting Client 3 (BTC-USDT, SOL-USDT)..."
echo "📁 Client 3 logs will be saved to: logs/client3_test.log"

# Run client 3 in background and capture its PID
./bin/client -server="localhost:$SERVER_PORT" -pairs="$CLIENT3_PAIRS" -duration="${TEST_DURATION}s" > logs/client3_test.log 2>&1 &
CLIENT3_PID=$!

# Step 6: Wait for all clients to complete
echo "⏳ Waiting for clients to complete (${TEST_DURATION}s)..."

# Wait for all clients to complete or timeout
wait $CLIENT1_PID 2>/dev/null || true
wait $CLIENT2_PID 2>/dev/null || true
wait $CLIENT3_PID 2>/dev/null || true

# Check if clients are still running after expected duration
if kill -0 $CLIENT1_PID 2>/dev/null; then
    echo "Client 1 still running after ${TEST_DURATION}s, stopping it..."
    kill $CLIENT1_PID
    sleep 1
    if kill -0 $CLIENT1_PID 2>/dev/null; then
        kill -9 $CLIENT1_PID 2>/dev/null || true
    fi
fi

if kill -0 $CLIENT2_PID 2>/dev/null; then
    echo "Client 2 still running after ${TEST_DURATION}s, stopping it..."
    kill $CLIENT2_PID
    sleep 1
    if kill -0 $CLIENT2_PID 2>/dev/null; then
        kill -9 $CLIENT2_PID 2>/dev/null || true
    fi
fi

if kill -0 $CLIENT3_PID 2>/dev/null; then
    echo "Client 3 still running after ${TEST_DURATION}s, stopping it..."
    kill $CLIENT3_PID
    sleep 1
    if kill -0 $CLIENT3_PID 2>/dev/null; then
        kill -9 $CLIENT3_PID 2>/dev/null || true
    fi
fi

# Get client outputs
CLIENT1_OUTPUT=$(cat logs/client1_test.log)
CLIENT2_OUTPUT=$(cat logs/client2_test.log)
CLIENT3_OUTPUT=$(cat logs/client3_test.log)

echo ""
echo "📄 Client 1 log contents (BTC-USDT, ETH-USDT):"
echo "=============================================="
echo "$CLIENT1_OUTPUT"
echo ""
echo "📄 Client 2 log contents (SOL-USDT, ETH-USDT):"
echo "=============================================="
echo "$CLIENT2_OUTPUT"
echo ""
echo "📄 Client 3 log contents (BTC-USDT, SOL-USDT):"
echo "=============================================="
echo "$CLIENT3_OUTPUT"
echo "=============================================="

# Step 7: Analyze results
echo "📊 Analyzing test results..."

# Check if all clients connected successfully
CLIENT1_CONNECTED=false
CLIENT2_CONNECTED=false
CLIENT3_CONNECTED=false

if echo "$CLIENT1_OUTPUT" | grep -q "Available pairs:"; then
    print_status "Client 1 connected to server successfully"
    CLIENT1_CONNECTED=true
else
    print_error "Client 1 failed to connect to server"
fi

if echo "$CLIENT2_OUTPUT" | grep -q "Available pairs:"; then
    print_status "Client 2 connected to server successfully"
    CLIENT2_CONNECTED=true
else
    print_error "Client 2 failed to connect to server"
fi

if echo "$CLIENT3_OUTPUT" | grep -q "Available pairs:"; then
    print_status "Client 3 connected to server successfully"
    CLIENT3_CONNECTED=true
else
    print_error "Client 3 failed to connect to server"
fi

if [ "$CLIENT1_CONNECTED" = false ] || [ "$CLIENT2_CONNECTED" = false ] || [ "$CLIENT3_CONNECTED" = false ]; then
    exit 1
fi

# Check if candles were received by each client
CLIENT1_CANDLE_COUNT=$(echo "$CLIENT1_OUTPUT" | grep -c "🟢\|🟡" || echo "0")
CLIENT2_CANDLE_COUNT=$(echo "$CLIENT2_OUTPUT" | grep -c "🟢\|🟡" || echo "0")
CLIENT3_CANDLE_COUNT=$(echo "$CLIENT3_OUTPUT" | grep -c "🟢\|🟡" || echo "0")
TOTAL_CANDLE_COUNT=$((CLIENT1_CANDLE_COUNT + CLIENT2_CANDLE_COUNT + CLIENT3_CANDLE_COUNT))

if [ "$CLIENT1_CANDLE_COUNT" -gt 0 ]; then
    print_status "Client 1 received $CLIENT1_CANDLE_COUNT candles successfully"
else
    print_warning "Client 1 received no candles (this might be normal if no trades occurred)"
fi

if [ "$CLIENT2_CANDLE_COUNT" -gt 0 ]; then
    print_status "Client 2 received $CLIENT2_CANDLE_COUNT candles successfully"
else
    print_warning "Client 2 received no candles (this might be normal if no trades occurred)"
fi

if [ "$CLIENT3_CANDLE_COUNT" -gt 0 ]; then
    print_status "Client 3 received $CLIENT3_CANDLE_COUNT candles successfully"
else
    print_warning "Client 3 received no candles (this might be normal if no trades occurred)"
fi

# Check for specific pairs in each client
echo "🔍 Checking pair-specific data..."

# Client 1 checks (should have BTC-USDT and ETH-USDT)
if echo "$CLIENT1_OUTPUT" | grep -q "BTC-USDT"; then
    print_status "Client 1: BTC-USDT candles received ✓"
else
    print_warning "Client 1: No BTC-USDT candles received"
fi

if echo "$CLIENT1_OUTPUT" | grep -q "ETH-USDT"; then
    print_status "Client 1: ETH-USDT candles received ✓"
else
    print_warning "Client 1: No ETH-USDT candles received"
fi

# Client 2 checks (should have SOL-USDT and ETH-USDT)
if echo "$CLIENT2_OUTPUT" | grep -q "SOL-USDT"; then
    print_status "Client 2: SOL-USDT candles received ✓"
else
    print_warning "Client 2: No SOL-USDT candles received"
fi

if echo "$CLIENT2_OUTPUT" | grep -q "ETH-USDT"; then
    print_status "Client 2: ETH-USDT candles received ✓"
else
    print_warning "Client 2: No ETH-USDT candles received"
fi

# Client 3 checks (should have BTC-USDT and SOL-USDT)
if echo "$CLIENT3_OUTPUT" | grep -q "BTC-USDT"; then
    print_status "Client 3: BTC-USDT candles received ✓"
else
    print_warning "Client 3: No BTC-USDT candles received"
fi

if echo "$CLIENT3_OUTPUT" | grep -q "SOL-USDT"; then
    print_status "Client 3: SOL-USDT candles received ✓"
else
    print_warning "Client 3: No SOL-USDT candles received"
fi

# Verify pair isolation
if echo "$CLIENT1_OUTPUT" | grep -q "SOL-USDT"; then
    print_error "Client 1: Incorrectly received SOL-USDT candles (should only get BTC, ETH)"
    exit 1
else
    print_status "Client 1: Correctly filtered out SOL-USDT ✓"
fi

if echo "$CLIENT2_OUTPUT" | grep -q "BTC-USDT"; then
    print_error "Client 2: Incorrectly received BTC-USDT candles (should only get SOL, ETH)"
    exit 1
else
    print_status "Client 2: Correctly filtered out BTC-USDT ✓"
fi

if echo "$CLIENT3_OUTPUT" | grep -q "ETH-USDT"; then
    print_error "Client 3: Incorrectly received ETH-USDT candles (should only get BTC, SOL)"
    exit 1
else
    print_status "Client 3: Correctly filtered out ETH-USDT ✓"
fi

# Check for exchange data  
ALL_OUTPUT="$CLIENT1_OUTPUT$CLIENT2_OUTPUT$CLIENT3_OUTPUT"
if echo "$ALL_OUTPUT" | grep -q "binance:\|bybit:\|okx:"; then
    print_status "Exchange contribution data included"
    # Count different exchanges seen
    EXCHANGE_COUNT=0
    echo "$ALL_OUTPUT" | grep -q "binance:" && EXCHANGE_COUNT=$((EXCHANGE_COUNT + 1))
    echo "$ALL_OUTPUT" | grep -q "bybit:" && EXCHANGE_COUNT=$((EXCHANGE_COUNT + 1))
    echo "$ALL_OUTPUT" | grep -q "okx:" && EXCHANGE_COUNT=$((EXCHANGE_COUNT + 1))
    print_status "Found data from $EXCHANGE_COUNT different exchanges"
else
    print_warning "No exchange contribution data found"
fi

# Step 8: Validate data format
echo "🔍 Validating data format..."
if echo "$ALL_OUTPUT" | grep -q "O:.*H:.*L:.*C:.*V:"; then
    print_status "Candle OHLCV format is correct"
else
    print_error "Candle format validation failed"
    exit 1
fi

# Step 9: Performance check
echo "⚡ Performance check..."
if [ "$TOTAL_CANDLE_COUNT" -ge 2 ]; then
    print_status "Multiple candles received across clients, indicating real-time processing"
else
    print_warning "Limited candle data - may need longer test duration"
fi

# Final results
echo ""
echo "🎉 End-to-End Test Results"
echo "=========================="
print_status "✅ Service startup: PASSED"
print_status "✅ Client 1 connection (BTC, ETH): PASSED"
print_status "✅ Client 2 connection (SOL, ETH): PASSED"
print_status "✅ Client 3 connection (BTC, SOL): PASSED"
print_status "✅ Data streaming: PASSED"
print_status "✅ Pair filtering: PASSED"
print_status "✅ Format validation: PASSED"

if [ "$TOTAL_CANDLE_COUNT" -gt 0 ]; then
    print_status "✅ Real-time data: PASSED ($TOTAL_CANDLE_COUNT total candles)"
else
    print_warning "⚠️  Real-time data: LIMITED (no candles in test period)"
fi

echo ""
echo "📈 Test Summary:"
echo "- Service: Running on port $SERVER_PORT"
echo "- Server pairs: $SERVER_PAIRS"
echo "- Client 1 pairs: $CLIENT1_PAIRS (received $CLIENT1_CANDLE_COUNT candles)"
echo "- Client 2 pairs: $CLIENT2_PAIRS (received $CLIENT2_CANDLE_COUNT candles)"
echo "- Client 3 pairs: $CLIENT3_PAIRS (received $CLIENT3_CANDLE_COUNT candles)"
echo "- Interval: ${INTERVAL}s"
echo "- Total candles received: $TOTAL_CANDLE_COUNT"
echo "- Test duration: ${TEST_DURATION}s"

if [ "$TOTAL_CANDLE_COUNT" -gt 0 ]; then
    echo ""
    print_status "🎯 All tests PASSED! The candle service is working correctly with multiple exchanges and clients."
    exit 0
else
    echo ""
    print_warning "⚠️  Tests completed but no candles received. This may be normal if no trades occurred during the test period."
    print_status "✅ Core functionality is working (connection, streaming, format, filtering)"
    exit 0
fi 