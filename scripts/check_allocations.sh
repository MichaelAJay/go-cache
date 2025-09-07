#!/bin/bash

# check_allocations.sh - Automated allocation regression detection
# 
# This script runs allocation benchmarks and compares them against baseline
# to detect allocation regressions exceeding the specified threshold.
#
# Usage:
#   ./scripts/check_allocations.sh [options]
#
# Options:
#   -b, --baseline FILE    Baseline benchmark file (default: allocation_baseline_10_runs.txt)
#   -t, --threshold PERCENT Regression threshold percentage (default: 10.0)
#   -o, --output FILE      Output file for current benchmarks (default: current_allocation_results.txt)
#   -h, --help             Show this help message
#
# Exit codes:
#   0 - No allocation regressions detected
#   1 - Allocation regressions detected or error occurred

set -euo pipefail

# Default configuration
BASELINE_FILE="allocation_baseline_10_runs.txt"
THRESHOLD="10.0"
OUTPUT_FILE="current_allocation_results.txt"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
ANALYZER_DIR="$PROJECT_ROOT/tools/allocation-analyzer"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

show_help() {
    cat << EOF
check_allocations.sh - Automated allocation regression detection

This script runs allocation benchmarks and compares them against baseline
to detect allocation regressions exceeding the specified threshold.

USAGE:
    $0 [OPTIONS]

OPTIONS:
    -b, --baseline FILE     Baseline benchmark file (default: $BASELINE_FILE)
    -t, --threshold PERCENT Regression threshold percentage (default: $THRESHOLD)
    -o, --output FILE       Output file for current benchmarks (default: $OUTPUT_FILE)
    -h, --help              Show this help message

EXAMPLES:
    # Check allocations with default settings
    $0
    
    # Use custom baseline file and threshold
    $0 --baseline my_baseline.txt --threshold 15.0
    
    # Save current results to specific file
    $0 --output my_results.txt

EXIT CODES:
    0 - No allocation regressions detected
    1 - Allocation regressions detected or error occurred

ENVIRONMENT VARIABLES:
    GOCACHE_TEST_MODE       Test mode for cache integration (containers, compose)
    ALLOCATION_CHECK_SKIP   Skip allocation checks if set to 'true'
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--baseline)
            BASELINE_FILE="$2"
            shift 2
            ;;
        -t|--threshold)
            THRESHOLD="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Check if allocation checks should be skipped
if [[ "${ALLOCATION_CHECK_SKIP:-}" == "true" ]]; then
    log_warn "Allocation checks skipped (ALLOCATION_CHECK_SKIP=true)"
    exit 0
fi

# Validate inputs
if [[ ! -f "$BASELINE_FILE" ]]; then
    log_error "Baseline file not found: $BASELINE_FILE"
    log_info "Run allocation benchmarks first to create baseline"
    exit 1
fi

if ! command -v go &> /dev/null; then
    log_error "Go compiler not found in PATH"
    exit 1
fi

# Build allocation analyzer if needed
log_info "Building allocation analyzer..."
cd "$ANALYZER_DIR"
if ! go build -o allocation-analyzer .; then
    log_error "Failed to build allocation analyzer"
    exit 1
fi
cd "$PROJECT_ROOT"

# Run current allocation benchmarks
log_info "Running current allocation benchmarks..."
BENCHMARK_PATTERN="BenchmarkRedisCache_.*_Allocations"

# Set up test environment
export GOCACHE_TEST_MODE="${GOCACHE_TEST_MODE:-containers}"

# Run benchmarks with timeout
if ! timeout 300s go test -bench="$BENCHMARK_PATTERN" -benchmem -count=1 -timeout=240s > "$OUTPUT_FILE" 2>&1; then
    log_error "Benchmark execution failed or timed out"
    if [[ -f "$OUTPUT_FILE" ]]; then
        log_error "Benchmark output:"
        cat "$OUTPUT_FILE" >&2
    fi
    exit 1
fi

# Verify benchmark results were generated
if [[ ! -s "$OUTPUT_FILE" ]]; then
    log_error "No benchmark results generated in $OUTPUT_FILE"
    exit 1
fi

# Check if any benchmarks were actually run
if ! grep -q "BenchmarkRedisCache_.*allocs/op" "$OUTPUT_FILE"; then
    log_error "No allocation benchmarks found in results"
    log_error "Benchmark output:"
    cat "$OUTPUT_FILE" >&2
    exit 1
fi

log_success "Allocation benchmarks completed"

# Run allocation analysis
log_info "Analyzing allocation regressions..."
ANALYZER_PATH="$ANALYZER_DIR/allocation-analyzer"

if [[ ! -x "$ANALYZER_PATH" ]]; then
    log_error "Allocation analyzer executable not found: $ANALYZER_PATH"
    exit 1
fi

# Run the analyzer and capture both stdout and exit code
set +e
ANALYSIS_OUTPUT=$("$ANALYZER_PATH" "$BASELINE_FILE" "$OUTPUT_FILE" "$THRESHOLD" 2>&1)
ANALYZER_EXIT_CODE=$?
set -e

# Display the analysis results
echo "$ANALYSIS_OUTPUT"

# Handle analyzer results
if [[ $ANALYZER_EXIT_CODE -eq 0 ]]; then
    log_success "No allocation regressions detected (threshold: ${THRESHOLD}%)"
    
    # Clean up temporary file
    rm -f "$OUTPUT_FILE"
    
    exit 0
else
    log_error "Allocation regressions detected!"
    
    # Keep the results file for debugging
    log_info "Current benchmark results saved to: $OUTPUT_FILE"
    log_info "Baseline results: $BASELINE_FILE"
    log_info ""
    log_info "To update baseline (if regressions are intentional):"
    log_info "  cp $OUTPUT_FILE $BASELINE_FILE"
    log_info ""
    log_info "To investigate specific regressions:"
    log_info "  go test -bench=BenchmarkName -benchmem -count=10"
    
    exit 1
fi