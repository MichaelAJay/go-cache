package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// BenchmarkResult represents a parsed benchmark result
type BenchmarkResult struct {
	Name         string  // e.g., "BenchmarkRedisCache_Has_Allocations"
	Iterations   int     // number of iterations
	NsPerOp      float64 // nanoseconds per operation
	BytesPerOp   int     // bytes per operation
	AllocsPerOp  int     // allocations per operation
}

// AllocationReport contains the analysis results
type AllocationReport struct {
	BaselineResults map[string]BenchmarkResult
	CurrentResults  map[string]BenchmarkResult
	Regressions     []RegressionItem
	Summary         Summary
}

// RegressionItem represents a single allocation regression
type RegressionItem struct {
	BenchmarkName   string
	BaselineAllocs  int
	CurrentAllocs   int
	PercentIncrease float64
	Severity        string // "MAJOR", "MODERATE", "MINOR"
}

// Summary contains overall analysis metrics
type Summary struct {
	TotalBenchmarks    int
	TotalRegressions   int
	MajorRegressions   int
	ModerateRegressions int
	MinorRegressions   int
	WorstRegression    *RegressionItem
}

// AllocationAnalyzer handles benchmark parsing and comparison
type AllocationAnalyzer struct {
	baselineFile string
	currentFile  string
	threshold    float64 // regression threshold percentage
}

// NewAllocationAnalyzer creates a new analyzer instance
func NewAllocationAnalyzer(baselineFile, currentFile string, threshold float64) *AllocationAnalyzer {
	return &AllocationAnalyzer{
		baselineFile: baselineFile,
		currentFile:  currentFile,
		threshold:    threshold,
	}
}

// ParseBenchmarkOutput parses Go benchmark output and extracts allocation metrics
func (a *AllocationAnalyzer) ParseBenchmarkOutput(reader io.Reader) (map[string]BenchmarkResult, error) {
	results := make(map[string]BenchmarkResult)
	scanner := bufio.NewScanner(reader)
	
	// Regex to match benchmark lines
	// Example: BenchmarkRedisCache_Has_Allocations-10	7041	166128 ns/op	1256 B/op	28 allocs/op
	benchmarkRegex := regexp.MustCompile(`^(Benchmark\w+)-\d+\s+(\d+)\s+([\d.]+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op`)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		matches := benchmarkRegex.FindStringSubmatch(line)
		if len(matches) == 6 {
			iterations, err := strconv.Atoi(matches[2])
			if err != nil {
				return nil, fmt.Errorf("failed to parse iterations for %s: %w", matches[1], err)
			}
			
			nsPerOp, err := strconv.ParseFloat(matches[3], 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse ns/op for %s: %w", matches[1], err)
			}
			
			bytesPerOp, err := strconv.Atoi(matches[4])
			if err != nil {
				return nil, fmt.Errorf("failed to parse B/op for %s: %w", matches[1], err)
			}
			
			allocsPerOp, err := strconv.Atoi(matches[5])
			if err != nil {
				return nil, fmt.Errorf("failed to parse allocs/op for %s: %w", matches[1], err)
			}
			
			results[matches[1]] = BenchmarkResult{
				Name:         matches[1],
				Iterations:   iterations,
				NsPerOp:      nsPerOp,
				BytesPerOp:   bytesPerOp,
				AllocsPerOp:  allocsPerOp,
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading benchmark output: %w", err)
	}
	
	return results, nil
}

// CompareBenchmarks analyzes current results against baseline and identifies regressions
func (a *AllocationAnalyzer) CompareBenchmarks(baseline, current map[string]BenchmarkResult) *AllocationReport {
	report := &AllocationReport{
		BaselineResults: baseline,
		CurrentResults:  current,
		Regressions:     make([]RegressionItem, 0),
	}
	
	var worstRegression *RegressionItem
	
	for benchmarkName, currentResult := range current {
		baselineResult, exists := baseline[benchmarkName]
		if !exists {
			// New benchmark - skip regression analysis
			continue
		}
		
		if baselineResult.AllocsPerOp == 0 {
			// Avoid division by zero
			continue
		}
		
		percentIncrease := float64(currentResult.AllocsPerOp-baselineResult.AllocsPerOp) / float64(baselineResult.AllocsPerOp) * 100
		
		if percentIncrease > a.threshold {
			severity := a.categorizeSeverity(percentIncrease)
			regression := RegressionItem{
				BenchmarkName:   benchmarkName,
				BaselineAllocs:  baselineResult.AllocsPerOp,
				CurrentAllocs:   currentResult.AllocsPerOp,
				PercentIncrease: percentIncrease,
				Severity:        severity,
			}
			
			report.Regressions = append(report.Regressions, regression)
			
			if worstRegression == nil || percentIncrease > worstRegression.PercentIncrease {
				worstRegression = &regression
			}
		}
	}
	
	// Generate summary
	report.Summary = a.generateSummary(current, report.Regressions, worstRegression)
	
	return report
}

// categorizeSeverity determines the severity level of a regression
func (a *AllocationAnalyzer) categorizeSeverity(percentIncrease float64) string {
	switch {
	case percentIncrease >= 50:
		return "MAJOR"
	case percentIncrease >= 25:
		return "MODERATE"
	default:
		return "MINOR"
	}
}

// generateSummary creates summary statistics for the report
func (a *AllocationAnalyzer) generateSummary(current map[string]BenchmarkResult, regressions []RegressionItem, worstRegression *RegressionItem) Summary {
	summary := Summary{
		TotalBenchmarks:  len(current),
		TotalRegressions: len(regressions),
		WorstRegression:  worstRegression,
	}
	
	for _, regression := range regressions {
		switch regression.Severity {
		case "MAJOR":
			summary.MajorRegressions++
		case "MODERATE":
			summary.ModerateRegressions++
		case "MINOR":
			summary.MinorRegressions++
		}
	}
	
	return summary
}

// GenerateReport creates a human-readable allocation report
func (a *AllocationAnalyzer) GenerateReport(report *AllocationReport) string {
	var sb strings.Builder
	
	sb.WriteString("=== ALLOCATION ANALYSIS REPORT ===\n\n")
	
	// Summary section
	sb.WriteString("SUMMARY:\n")
	sb.WriteString(fmt.Sprintf("  Total Benchmarks: %d\n", report.Summary.TotalBenchmarks))
	sb.WriteString(fmt.Sprintf("  Total Regressions: %d\n", report.Summary.TotalRegressions))
	sb.WriteString(fmt.Sprintf("  - Major (≥50%%): %d\n", report.Summary.MajorRegressions))
	sb.WriteString(fmt.Sprintf("  - Moderate (≥25%%): %d\n", report.Summary.ModerateRegressions))
	sb.WriteString(fmt.Sprintf("  - Minor (≥%.1f%%): %d\n", a.threshold, report.Summary.MinorRegressions))
	
	if report.Summary.WorstRegression != nil {
		sb.WriteString(fmt.Sprintf("  Worst Regression: %s (+%.1f%%)\n", 
			report.Summary.WorstRegression.BenchmarkName, 
			report.Summary.WorstRegression.PercentIncrease))
	}
	
	sb.WriteString("\n")
	
	// Detailed regressions
	if len(report.Regressions) > 0 {
		sb.WriteString("ALLOCATION REGRESSIONS:\n")
		sb.WriteString(fmt.Sprintf("%-50s %10s %10s %10s %10s\n", "Benchmark", "Baseline", "Current", "Change", "Severity"))
		sb.WriteString(strings.Repeat("-", 100) + "\n")
		
		for _, regression := range report.Regressions {
			sb.WriteString(fmt.Sprintf("%-50s %10d %10d %+9.1f%% %10s\n",
				regression.BenchmarkName,
				regression.BaselineAllocs,
				regression.CurrentAllocs,
				regression.PercentIncrease,
				regression.Severity))
		}
		sb.WriteString("\n")
	}
	
	// Allocation hotspots (benchmarks with high allocation counts)
	sb.WriteString("ALLOCATION HOTSPOTS (>20 allocs/op):\n")
	sb.WriteString(fmt.Sprintf("%-50s %10s %10s\n", "Benchmark", "Allocs/Op", "Bytes/Op"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	
	for name, result := range report.CurrentResults {
		if result.AllocsPerOp > 20 {
			sb.WriteString(fmt.Sprintf("%-50s %10d %10d\n",
				name,
				result.AllocsPerOp,
				result.BytesPerOp))
		}
	}
	
	return sb.String()
}

// Analyze performs the full allocation analysis workflow
func (a *AllocationAnalyzer) Analyze() (*AllocationReport, error) {
	// Parse baseline results
	baselineFile, err := os.Open(a.baselineFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open baseline file %s: %w", a.baselineFile, err)
	}
	defer baselineFile.Close()
	
	baselineResults, err := a.ParseBenchmarkOutput(baselineFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse baseline benchmarks: %w", err)
	}
	
	if len(baselineResults) == 0 {
		return nil, fmt.Errorf("no baseline benchmarks found in %s", a.baselineFile)
	}
	
	// Parse current results
	currentFile, err := os.Open(a.currentFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open current file %s: %w", a.currentFile, err)
	}
	defer currentFile.Close()
	
	currentResults, err := a.ParseBenchmarkOutput(currentFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse current benchmarks: %w", err)
	}
	
	if len(currentResults) == 0 {
		return nil, fmt.Errorf("no current benchmarks found in %s", a.currentFile)
	}
	
	// Compare and generate report
	report := a.CompareBenchmarks(baselineResults, currentResults)
	return report, nil
}

// main function provides CLI interface
func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <baseline_file> <current_file> [threshold_percentage]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s baseline.txt current.txt 10.0\n", os.Args[0])
		os.Exit(1)
	}
	
	baselineFile := os.Args[1]
	currentFile := os.Args[2]
	
	threshold := 10.0 // default threshold
	if len(os.Args) > 3 {
		var err error
		threshold, err = strconv.ParseFloat(os.Args[3], 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid threshold percentage: %v\n", err)
			os.Exit(1)
		}
	}
	
	analyzer := NewAllocationAnalyzer(baselineFile, currentFile, threshold)
	
	report, err := analyzer.Analyze()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	
	// Generate and output report
	reportText := analyzer.GenerateReport(report)
	fmt.Print(reportText)
	
	// Exit with error code if regressions found
	if len(report.Regressions) > 0 {
		fmt.Fprintf(os.Stderr, "\n❌ ALLOCATION REGRESSIONS DETECTED: %d regressions found\n", len(report.Regressions))
		os.Exit(1)
	} else {
		fmt.Fprintf(os.Stderr, "\n✅ NO ALLOCATION REGRESSIONS DETECTED\n")
	}
}