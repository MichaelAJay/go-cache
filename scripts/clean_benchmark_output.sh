#!/bin/bash

# Clean benchmark output files by removing testcontainer logs
# Usage: ./scripts/clean_benchmark_output.sh [directory]
#
# This script processes all .txt files in the benchmarks directory (or specified directory)
# and creates cleaned versions by filtering out testcontainer logging noise while preserving
# actual benchmark results and Go test output.

set -e

BENCHMARK_DIR="${1:-benchmarks}"
BACKUP_SUFFIX=".with_logs"

if [ ! -d "$BENCHMARK_DIR" ]; then
    echo "Error: Directory $BENCHMARK_DIR does not exist"
    exit 1
fi

echo "🧹 Cleaning benchmark output files in $BENCHMARK_DIR..."
echo "This will filter out testcontainer logs while preserving benchmark results"
echo ""

# Count total files to process
total_files=$(find "$BENCHMARK_DIR" -name "*.txt" -not -name "*$BACKUP_SUFFIX" | wc -l)
if [ "$total_files" -eq 0 ]; then
    echo "No .txt files found in $BENCHMARK_DIR"
    exit 0
fi

echo "Found $total_files benchmark files to clean"
echo ""

processed=0
for file in "$BENCHMARK_DIR"/*.txt; do
    # Skip if file doesn't exist (in case no .txt files)
    [ ! -f "$file" ] && continue
    
    # Skip already backed up files
    if [[ "$file" == *"$BACKUP_SUFFIX" ]]; then
        continue
    fi
    
    processed=$((processed + 1))
    filename=$(basename "$file")
    backup_file="${file}$BACKUP_SUFFIX"
    
    echo "[$processed/$total_files] Processing: $filename"
    
    # Create backup of original file
    if [ ! -f "$backup_file" ]; then
        cp "$file" "$backup_file"
        echo "  ✅ Created backup: ${filename}$BACKUP_SUFFIX"
    else
        echo "  ⚠️  Backup already exists: ${filename}$BACKUP_SUFFIX"
    fi
    
    # Apply filtering to create clean version
    # Keep: benchmark results, Go test metadata, test outcomes
    # Filter out: testcontainer logs, docker operations, container lifecycle
    grep -E "(^Benchmark|^goos:|^goarch:|^pkg:|^cpu:|^PASS|^FAIL|^ok[[:space:]]|^#)" "$backup_file" > "$file" 2>/dev/null || {
        echo "  ❌ Warning: No benchmark content found in $filename"
        # Restore original if filtering produced empty result
        cp "$backup_file" "$file"
    }
    
    # Show file size comparison
    original_size=$(wc -l < "$backup_file")
    cleaned_size=$(wc -l < "$file")
    if [ "$cleaned_size" -gt 0 ]; then
        reduction=$((original_size - cleaned_size))
        echo "  📊 Reduced from $original_size to $cleaned_size lines (removed $reduction log lines)"
    fi
done

echo ""
echo "🎉 Benchmark output cleaning completed!"
echo ""
echo "Results:"
echo "  - Original files backed up with '$BACKUP_SUFFIX' suffix"
echo "  - Cleaned files contain only benchmark results and Go test output"
echo "  - Container logs and testcontainer noise removed"
echo ""
echo "To restore originals: find $BENCHMARK_DIR -name '*$BACKUP_SUFFIX' -exec bash -c 'mv \"\$1\" \"\${1%$BACKUP_SUFFIX}\"' _ {} \\;"