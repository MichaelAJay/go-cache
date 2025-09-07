package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegrationWithRealBenchmarkData tests the analyzer with actual benchmark output
func TestIntegrationWithRealBenchmarkData(t *testing.T) {
	// Create temporary files for testing
	tempDir, err := ioutil.TempDir("", "allocation-analyzer-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create baseline benchmark data (simulating the actual format from allocation_baseline_10_runs.txt)
	baselineData := `=== Run 1 ===
goos: darwin
goarch: arm64
pkg: github.com/MichaelAJay/go-cache
cpu: Apple M1 Pro
BenchmarkRedisCache_Has_Allocations-10                        	    7041	    166128 ns/op	    1256 B/op	      28 allocs/op
BenchmarkRedisCache_Has_Allocations_Miss-10                   	    8151	    171283 ns/op	    1304 B/op	      29 allocs/op
BenchmarkRedisCache_Get_Allocations-10                        	    6301	    196245 ns/op	    5889 B/op	      58 allocs/op
BenchmarkRedisCache_Get_Allocations_Miss-10                   	    6859	    171679 ns/op	    2312 B/op	      43 allocs/op
BenchmarkRedisCache_Set_Allocations-10                        	    5710	    212429 ns/op	    5492 B/op	      51 allocs/op
BenchmarkRedisCache_Delete_Allocations-10                     	    5984	    173717 ns/op	    2083 B/op	      51 allocs/op
PASS
ok  	github.com/MichaelAJay/go-cache	18.823s
=== Run 2 ===
goos: darwin
goarch: arm64
pkg: github.com/MichaelAJay/go-cache
cpu: Apple M1 Pro
BenchmarkRedisCache_Has_Allocations-10                        	    7065	    157968 ns/op	    1257 B/op	      28 allocs/op
BenchmarkRedisCache_Has_Allocations_Miss-10                   	    7362	    165363 ns/op	    1304 B/op	      29 allocs/op
PASS
ok  	github.com/MichaelAJay/go-cache	18.438s`

	// Create current benchmark data with some regressions
	currentData := `goos: darwin
goarch: arm64
pkg: github.com/MichaelAJay/go-cache
cpu: Apple M1 Pro
BenchmarkRedisCache_Has_Allocations-10                        	    7041	    166128 ns/op	    1256 B/op	      35 allocs/op
BenchmarkRedisCache_Has_Allocations_Miss-10                   	    8151	    171283 ns/op	    1304 B/op	      29 allocs/op
BenchmarkRedisCache_Get_Allocations-10                        	    6301	    196245 ns/op	    5889 B/op	      75 allocs/op
BenchmarkRedisCache_Get_Allocations_Miss-10                   	    6859	    171679 ns/op	    2312 B/op	      43 allocs/op
BenchmarkRedisCache_Set_Allocations-10                        	    5710	    212429 ns/op	    5492 B/op	      51 allocs/op
BenchmarkRedisCache_Delete_Allocations-10                     	    5984	    173717 ns/op	    2083 B/op	      51 allocs/op
PASS
ok  	github.com/MichaelAJay/go-cache	18.823s`

	baselineFile := filepath.Join(tempDir, "baseline.txt")
	currentFile := filepath.Join(tempDir, "current.txt")

	err = ioutil.WriteFile(baselineFile, []byte(baselineData), 0644)
	if err != nil {
		t.Fatalf("Failed to write baseline file: %v", err)
	}

	err = ioutil.WriteFile(currentFile, []byte(currentData), 0644)
	if err != nil {
		t.Fatalf("Failed to write current file: %v", err)
	}

	// Test the analyzer
	analyzer := NewAllocationAnalyzer(baselineFile, currentFile, 10.0)
	
	report, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze() failed: %v", err)
	}

	// Verify baseline parsing
	expectedBaseline := 6 // 6 unique benchmarks in baseline
	if len(report.BaselineResults) != expectedBaseline {
		t.Errorf("Expected %d baseline results, got %d", expectedBaseline, len(report.BaselineResults))
	}

	// Verify current parsing  
	expectedCurrent := 6 // 6 benchmarks in current
	if len(report.CurrentResults) != expectedCurrent {
		t.Errorf("Expected %d current results, got %d", expectedCurrent, len(report.CurrentResults))
	}

	// Verify regressions detected
	// Expected regressions:
	// - BenchmarkRedisCache_Has_Allocations: 28 -> 35 (25% increase)
	// - BenchmarkRedisCache_Get_Allocations: 58 -> 75 (29.3% increase)
	expectedRegressions := 2
	if len(report.Regressions) != expectedRegressions {
		t.Errorf("Expected %d regressions, got %d", expectedRegressions, len(report.Regressions))
	}

	// Verify specific regressions
	hasRegressionFound := false
	getRegressionFound := false
	
	for _, regression := range report.Regressions {
		switch regression.BenchmarkName {
		case "BenchmarkRedisCache_Has_Allocations":
			hasRegressionFound = true
			if regression.BaselineAllocs != 28 {
				t.Errorf("Has regression baseline allocs = %d, expected 28", regression.BaselineAllocs)
			}
			if regression.CurrentAllocs != 35 {
				t.Errorf("Has regression current allocs = %d, expected 35", regression.CurrentAllocs)
			}
			expectedPct := 25.0
			if regression.PercentIncrease < expectedPct-1 || regression.PercentIncrease > expectedPct+1 {
				t.Errorf("Has regression percent = %.1f, expected ~%.1f", regression.PercentIncrease, expectedPct)
			}
			if regression.Severity != "MODERATE" {
				t.Errorf("Has regression severity = %s, expected MODERATE", regression.Severity)
			}
			
		case "BenchmarkRedisCache_Get_Allocations":
			getRegressionFound = true
			if regression.BaselineAllocs != 58 {
				t.Errorf("Get regression baseline allocs = %d, expected 58", regression.BaselineAllocs)
			}
			if regression.CurrentAllocs != 75 {
				t.Errorf("Get regression current allocs = %d, expected 75", regression.CurrentAllocs)
			}
			expectedPct := 29.3
			if regression.PercentIncrease < expectedPct-1 || regression.PercentIncrease > expectedPct+1 {
				t.Errorf("Get regression percent = %.1f, expected ~%.1f", regression.PercentIncrease, expectedPct)
			}
			if regression.Severity != "MODERATE" {
				t.Errorf("Get regression severity = %s, expected MODERATE", regression.Severity)
			}
		}
	}

	if !hasRegressionFound {
		t.Error("Expected Has regression not found")
	}
	if !getRegressionFound {
		t.Error("Expected Get regression not found")
	}

	// Verify summary
	if report.Summary.TotalBenchmarks != 6 {
		t.Errorf("Summary total benchmarks = %d, expected 6", report.Summary.TotalBenchmarks)
	}
	if report.Summary.TotalRegressions != 2 {
		t.Errorf("Summary total regressions = %d, expected 2", report.Summary.TotalRegressions)
	}
	if report.Summary.ModerateRegressions != 2 {
		t.Errorf("Summary moderate regressions = %d, expected 2", report.Summary.ModerateRegressions)
	}

	// Verify worst regression
	if report.Summary.WorstRegression == nil {
		t.Error("Expected worst regression, but got nil")
	} else if report.Summary.WorstRegression.BenchmarkName != "BenchmarkRedisCache_Get_Allocations" {
		t.Errorf("Worst regression = %s, expected BenchmarkRedisCache_Get_Allocations", 
			report.Summary.WorstRegression.BenchmarkName)
	}
}

// TestIntegrationReportGeneration tests report generation with real data
func TestIntegrationReportGeneration(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "allocation-analyzer-report-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Simple test data
	baselineData := `BenchmarkRedisCache_Has_Allocations-10                        	    7041	    166128 ns/op	    1256 B/op	      10 allocs/op
BenchmarkRedisCache_HighAlloc-10                              	    5000	    200000 ns/op	    3000 B/op	      25 allocs/op`

	currentData := `BenchmarkRedisCache_Has_Allocations-10                        	    7041	    166128 ns/op	    1256 B/op	      15 allocs/op
BenchmarkRedisCache_HighAlloc-10                              	    5000	    200000 ns/op	    3000 B/op	      30 allocs/op`

	baselineFile := filepath.Join(tempDir, "baseline.txt")
	currentFile := filepath.Join(tempDir, "current.txt")

	err = ioutil.WriteFile(baselineFile, []byte(baselineData), 0644)
	if err != nil {
		t.Fatalf("Failed to write baseline file: %v", err)
	}

	err = ioutil.WriteFile(currentFile, []byte(currentData), 0644)
	if err != nil {
		t.Fatalf("Failed to write current file: %v", err)
	}

	analyzer := NewAllocationAnalyzer(baselineFile, currentFile, 10.0)
	report, err := analyzer.Analyze()
	if err != nil {
		t.Fatalf("Analyze() failed: %v", err)
	}

	// Generate report
	reportText := analyzer.GenerateReport(report)

	// Verify report structure
	expectedSections := []string{
		"=== ALLOCATION ANALYSIS REPORT ===",
		"SUMMARY:",
		"Total Benchmarks: 2",
		"Total Regressions: 2",
		"ALLOCATION REGRESSIONS:",
		"ALLOCATION HOTSPOTS",
		"BenchmarkRedisCache_Has_Allocations",
		"BenchmarkRedisCache_HighAlloc",
	}

	for _, section := range expectedSections {
		if !strings.Contains(reportText, section) {
			t.Errorf("Report missing section: %s", section)
		}
	}

	// Verify hotspots section shows high-allocation benchmarks
	if !strings.Contains(reportText, "BenchmarkRedisCache_HighAlloc") {
		t.Error("Report should include high allocation benchmark in hotspots")
	}

	// Verify regression details
	if !strings.Contains(reportText, "50.0%") { // Has: 10->15 = 50% increase
		t.Error("Report should show 50% regression for Has benchmark")
	}
	if !strings.Contains(reportText, "20.0%") { // HighAlloc: 25->30 = 20% increase  
		t.Error("Report should show 20% regression for HighAlloc benchmark")
	}
}

// TestIntegrationErrorHandling tests error scenarios with integration setup
func TestIntegrationErrorHandling(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "allocation-analyzer-error-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	nonexistentFile := filepath.Join(tempDir, "nonexistent.txt")
	validFile := filepath.Join(tempDir, "valid.txt")
	emptyFile := filepath.Join(tempDir, "empty.txt")

	// Create a valid file
	validData := `BenchmarkRedisCache_Has_Allocations-10                        	    7041	    166128 ns/op	    1256 B/op	      28 allocs/op`
	err = ioutil.WriteFile(validFile, []byte(validData), 0644)
	if err != nil {
		t.Fatalf("Failed to write valid file: %v", err)
	}

	// Create an empty file
	err = ioutil.WriteFile(emptyFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to write empty file: %v", err)
	}

	tests := []struct {
		name         string
		baselineFile string
		currentFile  string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "nonexistent baseline file",
			baselineFile: nonexistentFile,
			currentFile:  validFile,
			wantErr:      true,
			errContains:  "failed to open baseline file",
		},
		{
			name:         "nonexistent current file",
			baselineFile: validFile,
			currentFile:  nonexistentFile,
			wantErr:      true,
			errContains:  "failed to open current file",
		},
		{
			name:         "empty baseline file",
			baselineFile: emptyFile,
			currentFile:  validFile,
			wantErr:      true,
			errContains:  "no baseline benchmarks found",
		},
		{
			name:         "empty current file",
			baselineFile: validFile,
			currentFile:  emptyFile,
			wantErr:      true,
			errContains:  "no current benchmarks found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAllocationAnalyzer(tt.baselineFile, tt.currentFile, 10.0)
			_, err := analyzer.Analyze()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}