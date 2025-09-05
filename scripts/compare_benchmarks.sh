#!/bin/bash

# Benchmark comparison script for go-cache
# Usage: ./scripts/compare_benchmarks.sh <old_results.txt> <new_results.txt>

set -e

if [ $# -ne 2 ]; then
    echo "Usage: $0 <old_results.txt> <new_results.txt>"
    echo ""
    echo "Compare two benchmark result files to identify performance changes."
    echo ""
    echo "Example:"
    echo "  $0 benchmarks/baseline.txt benchmarks/current.txt"
    exit 1
fi

OLD_FILE="$1"
NEW_FILE="$2"

# Check if files exist
if [ ! -f "$OLD_FILE" ]; then
    echo "Error: Old results file '$OLD_FILE' not found"
    exit 1
fi

if [ ! -f "$NEW_FILE" ]; then
    echo "Error: New results file '$NEW_FILE' not found"
    exit 1
fi

# Check if benchstat is available
if ! command -v benchstat >/dev/null 2>&1; then
    echo "Installing benchstat tool..."
    go install golang.org/x/perf/cmd/benchstat@latest
fi

echo "Benchmark Comparison Report"
echo "=========================="
echo "Old results: $OLD_FILE"
echo "New results: $NEW_FILE"
echo "Generated: $(date)"
echo ""

# Run benchstat comparison
benchstat "$OLD_FILE" "$NEW_FILE"

echo ""
echo "Comparison complete!"
echo ""
echo "Interpretation guide:"
echo "- '+' means the new version is faster (good)"
echo "- '-' means the new version is slower (may need attention)"
echo "- '~' means no significant difference"
echo "- p-values < 0.05 indicate statistically significant changes"