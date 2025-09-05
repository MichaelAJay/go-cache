#!/bin/bash

# Benchmark runner script for go-cache
# Usage: ./scripts/run_benchmarks.sh [output-file] [category]
# 
# Requires Docker services to be running (integration tests)
# Environment variables:
#   BENCHTIME=1s      - Duration per benchmark (default: 3s)
#   COUNT=5           - Number of runs per benchmark (default: 5) 
#   LATENCY_MODE=enabled - Enable network latency simulation
#   LATENCY_MS=100    - Latency in milliseconds

set -e

BENCHMARK_DIR="benchmarks"
OUTPUT_FILE="${1:-benchmark_$(date +%Y%m%d_%H%M%S).txt}"
CATEGORY="${2:-all}"
BENCHTIME="${BENCHTIME:-3s}"
COUNT="${COUNT:-5}"
LATENCY_MODE="${LATENCY_MODE:-}"
LATENCY_MS="${LATENCY_MS:-}"

# Ensure benchmarks directory exists
mkdir -p "$BENCHMARK_DIR"

# Function to run benchmarks for a specific category
run_category_benchmarks() {
    local category=$1
    local output_file=$2
    
    echo "Running $category benchmarks..."
    
    # Set environment variables for latency testing
    local test_env=""
    if [ -n "$LATENCY_MODE" ]; then
        test_env="GOCACHE_TEST_MODE=containers GOCACHE_TEST_LATENCY=enabled"
        if [ -n "$LATENCY_MS" ]; then
            test_env="$test_env GOCACHE_TEST_REDIS_LATENCY_MS=$LATENCY_MS"
        fi
    fi
    
    case $category in
        "core")
            if [ -n "$test_env" ]; then
                env $test_env go test -tags=integration -bench="BenchmarkRedisCache_(Get|Set|Delete|Has|Clear|GetOrSet|Update|SetIf)" \
                    -run=^$ -benchtime=$BENCHTIME -count=$COUNT -benchmem -v >> "$output_file" 2>&1
            else
                go test -tags=integration -bench="BenchmarkRedisCache_(Get|Set|Delete|Has|Clear|GetOrSet|Update|SetIf)" \
                    -run=^$ -benchtime=$BENCHTIME -count=$COUNT -benchmem -v >> "$output_file" 2>&1
            fi
            ;;
        "batch")
            if [ -n "$test_env" ]; then
                env $test_env go test -tags=integration -bench="BenchmarkRedisCache_(GetMany|SetMany|DeleteMany|.*_Concurrent)" \
                    -run=^$ -benchtime=$BENCHTIME -count=$COUNT -benchmem >> "$output_file"
            else
                go test -tags=integration -bench="BenchmarkRedisCache_(GetMany|SetMany|DeleteMany|.*_Concurrent)" \
                    -run=^$ -benchtime=$BENCHTIME -count=$COUNT -benchmem >> "$output_file"
            fi
            ;;
        "features")
            if [ -n "$test_env" ]; then
                env $test_env go test -tags=integration -bench="BenchmarkRedisCache_(.*Indexing|.*Serialization|.*Script)" \
                    -run=^$ -benchtime=$BENCHTIME -count=$COUNT -benchmem >> "$output_file"
            else
                go test -tags=integration -bench="BenchmarkRedisCache_(.*Indexing|.*Serialization|.*Script)" \
                    -run=^$ -benchtime=$BENCHTIME -count=$COUNT -benchmem >> "$output_file"
            fi
            ;;
        "system")
            if [ -n "$test_env" ]; then
                env $test_env go test -tags=integration -bench="BenchmarkRedisCache_(CircuitBreaker|Memory|GC|Connection)" \
                    -run=^$ -benchtime=$BENCHTIME -count=$COUNT -benchmem >> "$output_file"
            else
                go test -tags=integration -bench="BenchmarkRedisCache_(CircuitBreaker|Memory|GC|Connection)" \
                    -run=^$ -benchtime=$BENCHTIME -count=$COUNT -benchmem >> "$output_file"
            fi
            ;;
        "all")
            if [ -n "$test_env" ]; then
                env $test_env go test -tags=integration -bench="BenchmarkRedisCache_" \
                    -run=^$ -benchtime=$BENCHTIME -count=$COUNT -benchmem >> "$output_file"
            else
                go test -tags=integration -bench="BenchmarkRedisCache_" \
                    -run=^$ -benchtime=$BENCHTIME -count=$COUNT -benchmem >> "$output_file"
            fi
            ;;
        *)
            echo "Unknown category: $category"
            echo "Available categories: core, batch, features, system, all"
            exit 1
            ;;
    esac
}

# Main execution
OUTPUT_PATH="$BENCHMARK_DIR/$OUTPUT_FILE"

echo "Go Cache Benchmark Runner"
echo "========================="
echo "Output file: $OUTPUT_PATH"
echo "Category: $CATEGORY"
echo "Benchtime: $BENCHTIME"
echo "Count: $COUNT"

# Display latency configuration if enabled
if [ -n "$LATENCY_MODE" ]; then
    echo "Latency mode: ENABLED"
    if [ -n "$LATENCY_MS" ]; then
        echo "Latency: ${LATENCY_MS}ms"
    else
        echo "Latency: default (configure with LATENCY_MS)"
    fi
else
    echo "Latency mode: disabled"
fi
echo ""

# Add header to output file
{
    echo "# Go Cache Benchmarks - $(date)"
    echo "# Category: $CATEGORY"
    echo "# Benchtime: $BENCHTIME, Count: $COUNT"
    if [ -n "$LATENCY_MODE" ]; then
        echo "# Latency: ENABLED${LATENCY_MS:+ (${LATENCY_MS}ms)}"
    else
        echo "# Latency: disabled"
    fi
    echo "# System: $(go version), $(uname -s) $(uname -m)"
    echo ""
} > "$OUTPUT_PATH"

# Run benchmarks
run_category_benchmarks "$CATEGORY" "$OUTPUT_PATH"

echo ""
echo "Benchmarks completed! Results saved to: $OUTPUT_PATH"
echo ""

# Show summary
echo "Summary of benchmark results:"
echo "============================="
grep -E "^Benchmark" "$OUTPUT_PATH" | tail -10