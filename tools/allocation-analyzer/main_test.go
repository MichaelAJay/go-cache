package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseBenchmarkOutput(t *testing.T) {
	analyzer := NewAllocationAnalyzer("", "", 10.0)

	tests := []struct {
		name     string
		input    string
		expected map[string]BenchmarkResult
		wantErr  bool
	}{
		{
			name: "valid single benchmark",
			input: `goos: darwin
goarch: arm64
pkg: github.com/MichaelAJay/go-cache
cpu: Apple M1 Pro
BenchmarkRedisCache_Has_Allocations-10                        	    7041	    166128 ns/op	    1256 B/op	      28 allocs/op
PASS
ok  	github.com/MichaelAJay/go-cache	18.823s`,
			expected: map[string]BenchmarkResult{
				"BenchmarkRedisCache_Has_Allocations": {
					Name:         "BenchmarkRedisCache_Has_Allocations",
					Iterations:   7041,
					NsPerOp:      166128,
					BytesPerOp:   1256,
					AllocsPerOp:  28,
				},
			},
			wantErr: false,
		},
		{
			name: "multiple benchmarks",
			input: `goos: darwin
goarch: arm64
pkg: github.com/MichaelAJay/go-cache
cpu: Apple M1 Pro
BenchmarkRedisCache_Has_Allocations-10                        	    7041	    166128 ns/op	    1256 B/op	      28 allocs/op
BenchmarkRedisCache_Get_Allocations-10                        	    6301	    196245 ns/op	    5889 B/op	      58 allocs/op
BenchmarkRedisCache_Set_Allocations-10                        	    5710	    212429 ns/op	    5492 B/op	      51 allocs/op
PASS
ok  	github.com/MichaelAJay/go-cache	18.823s`,
			expected: map[string]BenchmarkResult{
				"BenchmarkRedisCache_Has_Allocations": {
					Name:         "BenchmarkRedisCache_Has_Allocations",
					Iterations:   7041,
					NsPerOp:      166128,
					BytesPerOp:   1256,
					AllocsPerOp:  28,
				},
				"BenchmarkRedisCache_Get_Allocations": {
					Name:         "BenchmarkRedisCache_Get_Allocations",
					Iterations:   6301,
					NsPerOp:      196245,
					BytesPerOp:   5889,
					AllocsPerOp:  58,
				},
				"BenchmarkRedisCache_Set_Allocations": {
					Name:         "BenchmarkRedisCache_Set_Allocations",
					Iterations:   5710,
					NsPerOp:      212429,
					BytesPerOp:   5492,
					AllocsPerOp:  51,
				},
			},
			wantErr: false,
		},
		{
			name: "floating point ns/op",
			input: `BenchmarkRedisCache_Has_Allocations-10                        	    7041	    166128.5 ns/op	    1256 B/op	      28 allocs/op`,
			expected: map[string]BenchmarkResult{
				"BenchmarkRedisCache_Has_Allocations": {
					Name:         "BenchmarkRedisCache_Has_Allocations",
					Iterations:   7041,
					NsPerOp:      166128.5,
					BytesPerOp:   1256,
					AllocsPerOp:  28,
				},
			},
			wantErr: false,
		},
		{
			name:     "empty input",
			input:    "",
			expected: map[string]BenchmarkResult{},
			wantErr:  false,
		},
		{
			name: "no benchmark lines",
			input: `goos: darwin
goarch: arm64
pkg: github.com/MichaelAJay/go-cache
cpu: Apple M1 Pro
PASS
ok  	github.com/MichaelAJay/go-cache	18.823s`,
			expected: map[string]BenchmarkResult{},
			wantErr:  false,
		},
		{
			name:     "malformed benchmark line - ignored",
			input:    `BenchmarkRedisCache_Has_Allocations-10                        	    abc	    166128 ns/op	    1256 B/op	      28 allocs/op`,
			expected: map[string]BenchmarkResult{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := analyzer.ParseBenchmarkOutput(reader)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseBenchmarkOutput() expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseBenchmarkOutput() unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("ParseBenchmarkOutput() result length = %d, expected %d", len(result), len(tt.expected))
				return
			}

			for name, expectedBench := range tt.expected {
				actualBench, exists := result[name]
				if !exists {
					t.Errorf("ParseBenchmarkOutput() missing benchmark %s", name)
					continue
				}

				if actualBench.Name != expectedBench.Name {
					t.Errorf("ParseBenchmarkOutput() benchmark %s: Name = %s, expected %s", name, actualBench.Name, expectedBench.Name)
				}
				if actualBench.Iterations != expectedBench.Iterations {
					t.Errorf("ParseBenchmarkOutput() benchmark %s: Iterations = %d, expected %d", name, actualBench.Iterations, expectedBench.Iterations)
				}
				if actualBench.NsPerOp != expectedBench.NsPerOp {
					t.Errorf("ParseBenchmarkOutput() benchmark %s: NsPerOp = %f, expected %f", name, actualBench.NsPerOp, expectedBench.NsPerOp)
				}
				if actualBench.BytesPerOp != expectedBench.BytesPerOp {
					t.Errorf("ParseBenchmarkOutput() benchmark %s: BytesPerOp = %d, expected %d", name, actualBench.BytesPerOp, expectedBench.BytesPerOp)
				}
				if actualBench.AllocsPerOp != expectedBench.AllocsPerOp {
					t.Errorf("ParseBenchmarkOutput() benchmark %s: AllocsPerOp = %d, expected %d", name, actualBench.AllocsPerOp, expectedBench.AllocsPerOp)
				}
			}
		})
	}
}

func TestCompareBenchmarks(t *testing.T) {
	analyzer := NewAllocationAnalyzer("", "", 10.0)

	baseline := map[string]BenchmarkResult{
		"BenchmarkRedisCache_Has_Allocations": {
			Name:         "BenchmarkRedisCache_Has_Allocations",
			Iterations:   7000,
			NsPerOp:      160000,
			BytesPerOp:   1200,
			AllocsPerOp:  20,
		},
		"BenchmarkRedisCache_Get_Allocations": {
			Name:         "BenchmarkRedisCache_Get_Allocations",
			Iterations:   6000,
			NsPerOp:      180000,
			BytesPerOp:   5000,
			AllocsPerOp:  50,
		},
	}

	tests := []struct {
		name                string
		current             map[string]BenchmarkResult
		expectedRegressions int
		expectedWorstName   string
		expectedWorstPct    float64
	}{
		{
			name: "no regressions",
			current: map[string]BenchmarkResult{
				"BenchmarkRedisCache_Has_Allocations": {
					Name:        "BenchmarkRedisCache_Has_Allocations",
					AllocsPerOp: 20, // same as baseline
				},
				"BenchmarkRedisCache_Get_Allocations": {
					Name:        "BenchmarkRedisCache_Get_Allocations",
					AllocsPerOp: 45, // 10% reduction, not a regression
				},
			},
			expectedRegressions: 0,
		},
		{
			name: "minor regression",
			current: map[string]BenchmarkResult{
				"BenchmarkRedisCache_Has_Allocations": {
					Name:        "BenchmarkRedisCache_Has_Allocations",
					AllocsPerOp: 23, // 15% increase (20 -> 23)
				},
				"BenchmarkRedisCache_Get_Allocations": {
					Name:        "BenchmarkRedisCache_Get_Allocations",
					AllocsPerOp: 50, // no change
				},
			},
			expectedRegressions: 1,
			expectedWorstName:   "BenchmarkRedisCache_Has_Allocations",
			expectedWorstPct:    15.0,
		},
		{
			name: "major regression",
			current: map[string]BenchmarkResult{
				"BenchmarkRedisCache_Has_Allocations": {
					Name:        "BenchmarkRedisCache_Has_Allocations",
					AllocsPerOp: 20, // no change
				},
				"BenchmarkRedisCache_Get_Allocations": {
					Name:        "BenchmarkRedisCache_Get_Allocations",
					AllocsPerOp: 100, // 100% increase (50 -> 100)
				},
			},
			expectedRegressions: 1,
			expectedWorstName:   "BenchmarkRedisCache_Get_Allocations",
			expectedWorstPct:    100.0,
		},
		{
			name: "multiple regressions",
			current: map[string]BenchmarkResult{
				"BenchmarkRedisCache_Has_Allocations": {
					Name:        "BenchmarkRedisCache_Has_Allocations",
					AllocsPerOp: 24, // 20% increase
				},
				"BenchmarkRedisCache_Get_Allocations": {
					Name:        "BenchmarkRedisCache_Get_Allocations",
					AllocsPerOp: 75, // 50% increase
				},
			},
			expectedRegressions: 2,
			expectedWorstName:   "BenchmarkRedisCache_Get_Allocations",
			expectedWorstPct:    50.0,
		},
		{
			name: "new benchmark (not in baseline)",
			current: map[string]BenchmarkResult{
				"BenchmarkRedisCache_Has_Allocations": {
					Name:        "BenchmarkRedisCache_Has_Allocations",
					AllocsPerOp: 20, // no change
				},
				"BenchmarkRedisCache_New_Allocations": {
					Name:        "BenchmarkRedisCache_New_Allocations",
					AllocsPerOp: 100, // new benchmark, should be ignored
				},
			},
			expectedRegressions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := analyzer.CompareBenchmarks(baseline, tt.current)

			if len(report.Regressions) != tt.expectedRegressions {
				t.Errorf("CompareBenchmarks() regressions = %d, expected %d", len(report.Regressions), tt.expectedRegressions)
			}

			if tt.expectedRegressions > 0 {
				if report.Summary.WorstRegression == nil {
					t.Errorf("CompareBenchmarks() expected worst regression, but got nil")
				} else {
					if report.Summary.WorstRegression.BenchmarkName != tt.expectedWorstName {
						t.Errorf("CompareBenchmarks() worst regression name = %s, expected %s", 
							report.Summary.WorstRegression.BenchmarkName, tt.expectedWorstName)
					}
					if report.Summary.WorstRegression.PercentIncrease != tt.expectedWorstPct {
						t.Errorf("CompareBenchmarks() worst regression pct = %.1f, expected %.1f", 
							report.Summary.WorstRegression.PercentIncrease, tt.expectedWorstPct)
					}
				}
			} else {
				if report.Summary.WorstRegression != nil {
					t.Errorf("CompareBenchmarks() expected no worst regression, but got %s", 
						report.Summary.WorstRegression.BenchmarkName)
				}
			}
		})
	}
}

func TestCategorizeSeverity(t *testing.T) {
	analyzer := NewAllocationAnalyzer("", "", 10.0)

	tests := []struct {
		percentIncrease float64
		expected        string
	}{
		{5.0, "MINOR"},
		{10.0, "MINOR"},
		{15.0, "MINOR"},
		{24.9, "MINOR"},
		{25.0, "MODERATE"},
		{30.0, "MODERATE"},
		{49.9, "MODERATE"},
		{50.0, "MAJOR"},
		{75.0, "MAJOR"},
		{100.0, "MAJOR"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.1f%%", tt.percentIncrease), func(t *testing.T) {
			result := analyzer.categorizeSeverity(tt.percentIncrease)
			if result != tt.expected {
				t.Errorf("categorizeSeverity(%.1f) = %s, expected %s", tt.percentIncrease, result, tt.expected)
			}
		})
	}
}

func TestGenerateReport(t *testing.T) {
	analyzer := NewAllocationAnalyzer("", "", 10.0)

	baseline := map[string]BenchmarkResult{
		"BenchmarkRedisCache_Has_Allocations": {
			Name:        "BenchmarkRedisCache_Has_Allocations",
			AllocsPerOp: 20,
		},
	}

	current := map[string]BenchmarkResult{
		"BenchmarkRedisCache_Has_Allocations": {
			Name:         "BenchmarkRedisCache_Has_Allocations",
			AllocsPerOp:  30,
			BytesPerOp:   1500,
		},
		"BenchmarkRedisCache_HighAlloc": {
			Name:         "BenchmarkRedisCache_HighAlloc",
			AllocsPerOp:  25,
			BytesPerOp:   2000,
		},
	}

	report := analyzer.CompareBenchmarks(baseline, current)
	reportText := analyzer.GenerateReport(report)

	// Verify report contains expected sections
	expectedSections := []string{
		"=== ALLOCATION ANALYSIS REPORT ===",
		"SUMMARY:",
		"Total Benchmarks:",
		"Total Regressions:",
		"ALLOCATION REGRESSIONS:",
		"ALLOCATION HOTSPOTS",
	}

	for _, section := range expectedSections {
		if !strings.Contains(reportText, section) {
			t.Errorf("GenerateReport() missing section: %s", section)
		}
	}

	// Verify regression is reported
	if !strings.Contains(reportText, "BenchmarkRedisCache_Has_Allocations") {
		t.Errorf("GenerateReport() should contain regression benchmark name")
	}

	// Verify hotspots section includes high allocation benchmarks
	if !strings.Contains(reportText, "BenchmarkRedisCache_HighAlloc") {
		t.Errorf("GenerateReport() should contain high allocation benchmark in hotspots")
	}
}

func TestNewAllocationAnalyzer(t *testing.T) {
	baselineFile := "baseline.txt"
	currentFile := "current.txt"
	threshold := 15.0

	analyzer := NewAllocationAnalyzer(baselineFile, currentFile, threshold)

	if analyzer.baselineFile != baselineFile {
		t.Errorf("NewAllocationAnalyzer() baselineFile = %s, expected %s", analyzer.baselineFile, baselineFile)
	}
	if analyzer.currentFile != currentFile {
		t.Errorf("NewAllocationAnalyzer() currentFile = %s, expected %s", analyzer.currentFile, currentFile)
	}
	if analyzer.threshold != threshold {
		t.Errorf("NewAllocationAnalyzer() threshold = %f, expected %f", analyzer.threshold, threshold)
	}
}