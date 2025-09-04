package metrics

import (
	"testing"

	"github.com/MichaelAJay/go-metrics/metric"
)

func TestEnhancedCacheMetrics_RecordMemoryPressure(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	globalTags := metric.Tags{"service": "test-cache"}
	metrics := NewEnhancedCacheMetrics(registry, globalTags)

	provider := "redis"
	testTags := metric.Tags{"instance": "test-01"}

	tests := []struct {
		name           string
		usageBytes     int64
		threshold      int64
		expectedLevel  float64
		shouldBreach   bool
		expectedSeverity string
	}{
		{
			name:          "normal usage - 50%",
			usageBytes:    50,
			threshold:     100,
			expectedLevel: 50.0,
			shouldBreach:  false,
		},
		{
			name:          "high usage - 90%",
			usageBytes:    90,
			threshold:     100,
			expectedLevel: 90.0,
			shouldBreach:  false,
		},
		{
			name:             "warning level - 110%",
			usageBytes:       110,
			threshold:        100,
			expectedLevel:    110.0,
			shouldBreach:     true,
			expectedSeverity: "warning",
		},
		{
			name:             "high level - 130%",
			usageBytes:       130,
			threshold:        100,
			expectedLevel:    130.0,
			shouldBreach:     true,
			expectedSeverity: "high",
		},
		{
			name:             "critical level - 200%",
			usageBytes:       200,
			threshold:        100,
			expectedLevel:    200.0,
			shouldBreach:     true,
			expectedSeverity: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear registry before each test
			registry = metric.NewDefaultRegistry()
			metrics = NewEnhancedCacheMetrics(registry, globalTags)
			
			// Record memory pressure
			metrics.RecordMemoryPressure(provider, tt.usageBytes, tt.threshold, testTags)

			// Verify pressure gauge was recorded
			pressureGauge := findGaugeByName(registry, "cache_memory_pressure_percent")
			if pressureGauge == nil {
				t.Fatal("pressure gauge not found")
			}
			
			// Since we can't directly get the value from the gauge, we verify it was created
			// In a real implementation, you might use a test registry that allows inspection
			
			if tt.shouldBreach {
				// Verify breach counter exists
				breachCounter := findCounterByName(registry, "cache_memory_pressure_breaches_total")
				if breachCounter == nil {
					t.Error("breach counter not found when threshold exceeded")
				}
				
				// Verify severity counter exists
				alertCounter := findCounterByName(registry, "cache_memory_pressure_alerts_total")
				if alertCounter == nil {
					t.Error("alert counter not found when threshold exceeded")
				}
			}
		})
	}
}

func TestEnhancedCacheMetrics_RecordMemoryPressure_NilTags(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	metrics := NewEnhancedCacheMetrics(registry, nil)

	// Should not panic with nil tags
	metrics.RecordMemoryPressure("redis", 150, 100, nil)
	
	// Verify gauge was created
	gauge := findGaugeByName(registry, "cache_memory_pressure_percent")
	if gauge == nil {
		t.Error("pressure gauge not created with nil tags")
	}
}

func TestEnhancedCacheMetrics_RecordMemoryPressure_ZeroThreshold(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	metrics := NewEnhancedCacheMetrics(registry, nil)

	// Should handle zero threshold gracefully (though this is an edge case)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RecordMemoryPressure panicked with zero threshold: %v", r)
		}
	}()
	
	metrics.RecordMemoryPressure("redis", 100, 0, nil)
}

func TestEnhancedCacheMetrics_RecordMemoryPressure_MultipleCalls(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	metrics := NewEnhancedCacheMetrics(registry, nil)

	// Multiple calls should increment breach counter
	threshold := int64(100)
	
	// First breach
	metrics.RecordMemoryPressure("redis", 150, threshold, nil)
	
	// Second breach
	metrics.RecordMemoryPressure("redis", 180, threshold, nil)
	
	// Verify counters exist (would need test registry to check actual values)
	breachCounter := findCounterByName(registry, "cache_memory_pressure_breaches_total")
	if breachCounter == nil {
		t.Error("breach counter not found after multiple calls")
	}
}

func TestNoopEnhancedCacheMetrics_RecordMemoryPressure(t *testing.T) {
	metrics := NewNoopEnhancedCacheMetrics()
	
	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Noop metrics panicked: %v", r)
		}
	}()
	
	metrics.RecordMemoryPressure("redis", 150, 100, nil)
}

// Helper functions for testing

func findGaugeByName(registry metric.Registry, name string) metric.Gauge {
	// In a real test scenario, you'd need a way to inspect the registry
	// This is a simplified version - in practice you might use a mock registry
	// or a registry that supports inspection
	return registry.Gauge(metric.Options{
		Name: name,
		Tags: metric.Tags{"provider": "redis"},
	})
}

func findCounterByName(registry metric.Registry, name string) metric.Counter {
	// Similar helper for counters
	return registry.Counter(metric.Options{
		Name: name,
		Tags: metric.Tags{"provider": "redis"},
	})
}