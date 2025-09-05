#!/bin/bash

# Latency benchmark runner script for go-cache with Toxiproxy
# Usage: ./scripts/run_latency_benchmarks.sh [latency_ms] [category] [output_prefix]
#
# Automatically starts Docker services and configures network latency
# Example: ./scripts/run_latency_benchmarks.sh 100 core test_100ms
# 
# Environment variables:
#   BENCHTIME=1s - Duration per benchmark (default: 3s)
#   COUNT=3      - Number of runs per benchmark (default: 3)

set -e

LATENCY_MS="${1:-100}"
CATEGORY="${2:-core}"
OUTPUT_PREFIX="${3:-latency_${LATENCY_MS}ms}"
BENCHTIME="${BENCHTIME:-3s}"
COUNT="${COUNT:-3}"

echo "Go Cache Latency Benchmark Runner"
echo "================================="
echo "This script will:"
echo "1. Start docker-compose services (Redis + Toxiproxy)"
echo "2. Configure ${LATENCY_MS}ms latency"
echo "3. Run benchmarks with network latency simulation"
echo "4. Compare results with baseline (if available)"
echo ""

# Check if docker-compose services are already running
if ! docker compose ps | grep -q "Up"; then
    echo "🐳 Starting docker-compose services..."
    docker compose up -d
    echo "⏳ Waiting for services to be ready..."
    sleep 10
    echo "✅ Services started"
else
    echo "✅ Docker-compose services already running"
fi

# Configure latency using the Go script directly  
echo "⚡ Configuring ${LATENCY_MS}ms latency..."
cd scripts

# Check if proxy already exists and has the right latency
current_latency=$(curl -s http://localhost:8474/proxies/redis_proxy/toxics | jq -r '.[] | select(.name=="redis_proxy_latency") | .attributes.latency // empty' 2>/dev/null)
if [ "$current_latency" = "$LATENCY_MS" ]; then
    echo "✅ Toxiproxy already configured with ${LATENCY_MS}ms latency"
else
    echo "🔧 Updating Toxiproxy latency configuration..."
    if ! GOCACHE_TEST_REDIS_LATENCY_MS=$LATENCY_MS go run setup-toxiproxy.go 2>/dev/null; then
        echo "⚠️  Toxiproxy setup had issues (proxy may already be configured). Continuing with existing setup..."
    fi
fi
cd ..

# Run benchmarks with latency
echo "🐌 Running benchmarks with ${LATENCY_MS}ms latency..."
OUTPUT_FILE="${OUTPUT_PREFIX}_$(date +%Y%m%d_%H%M%S).txt"

LATENCY_MODE=enabled LATENCY_MS=$LATENCY_MS \
    BENCHTIME=$BENCHTIME COUNT=$COUNT \
    "$(dirname "$0")/run_benchmarks.sh" "$OUTPUT_FILE" "$CATEGORY"

# Try to find a baseline for comparison
BASELINE_FILE=""
if [ -f "benchmarks/baseline.txt" ]; then
    BASELINE_FILE="benchmarks/baseline.txt"
elif [ -f "benchmarks/baseline_${CATEGORY}.txt" ]; then
    BASELINE_FILE="benchmarks/baseline_${CATEGORY}.txt"
fi

if [ -n "$BASELINE_FILE" ]; then
    echo ""
    echo "📊 Comparing with baseline: $BASELINE_FILE"
    "$(dirname "$0")/compare_benchmarks.sh" "$BASELINE_FILE" "benchmarks/$OUTPUT_FILE"
else
    echo ""
    echo "ℹ️  No baseline found for comparison. To create one, run:"
    echo "   ./scripts/run_benchmarks.sh baseline_${CATEGORY}.txt $CATEGORY"
fi

echo ""
echo "🎉 Latency benchmark completed!"
echo "📁 Results saved to: benchmarks/$OUTPUT_FILE"