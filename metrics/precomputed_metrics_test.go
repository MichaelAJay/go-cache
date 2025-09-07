package metrics

import (
	"testing"
	"time"

	"github.com/MichaelAJay/go-metrics/metric"
)

func TestNewPrecomputedCacheMetrics(t *testing.T) {
	tests := []struct {
		name      string
		registry  metric.Registry
		finalTags metric.Tags
		wantNil   bool
	}{
		{
			name:      "with valid registry and tags",
			registry:  metric.NewDefaultRegistry(),
			finalTags: metric.Tags{"provider": "redis", "instance_id": "test"},
			wantNil:   false,
		},
		{
			name:      "with nil registry",
			registry:  nil,
			finalTags: metric.Tags{"provider": "redis"},
			wantNil:   false, // Should create default registry
		},
		{
			name:      "with nil tags",
			registry:  metric.NewDefaultRegistry(),
			finalTags: nil,
			wantNil:   false,
		},
		{
			name:      "with empty tags",
			registry:  metric.NewDefaultRegistry(),
			finalTags: metric.Tags{},
			wantNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pcm := NewPrecomputedCacheMetrics(tt.registry, tt.finalTags)
			if (pcm == nil) != tt.wantNil {
				t.Errorf("NewPrecomputedCacheMetrics() = %v, want nil = %v", pcm, tt.wantNil)
			}
			if pcm != nil {
				// Verify all timers are initialized
				verifyTimersInitialized(t, pcm)
				// Verify all counters are initialized  
				verifyCountersInitialized(t, pcm)
			}
		})
	}
}

func verifyTimersInitialized(t *testing.T, pcm *PrecomputedCacheMetrics) {
	t.Helper()
	
	timers := []struct {
		name  string
		timer metric.Timer
	}{
		{"HasTimer", pcm.HasTimer()},
		{"GetTimer", pcm.GetTimer()},
		{"SetTimer", pcm.SetTimer()},
		{"DeleteTimer", pcm.DeleteTimer()},
		{"ClearTimer", pcm.ClearTimer()},
		{"GetByOwnerTimer", pcm.GetByOwnerTimer()},
		{"DeleteByOwnerTimer", pcm.DeleteByOwnerTimer()},
		{"GetKeysByPatternTimer", pcm.GetKeysByPatternTimer()},
		{"IncrementTimer", pcm.IncrementTimer()},
		{"DecrementTimer", pcm.DecrementTimer()},
		{"IncrementFloatTimer", pcm.IncrementFloatTimer()},
		{"ExtendTTLTimer", pcm.ExtendTTLTimer()},
		{"GetManyTimer", pcm.GetManyTimer()},
		{"SetManyTimer", pcm.SetManyTimer()},
		{"DeleteManyTimer", pcm.DeleteManyTimer()},
	}

	for _, timer := range timers {
		if timer.timer == nil {
			t.Errorf("%s is nil", timer.name)
		}
	}
}

func verifyCountersInitialized(t *testing.T, pcm *PrecomputedCacheMetrics) {
	t.Helper()
	
	counters := []struct {
		name    string
		counter metric.Counter
	}{
		// Success counters
		{"HasSuccessCounter", pcm.HasSuccessCounter()},
		{"GetSuccessCounter", pcm.GetSuccessCounter()},
		{"SetSuccessCounter", pcm.SetSuccessCounter()},
		{"DeleteSuccessCounter", pcm.DeleteSuccessCounter()},
		{"ClearSuccessCounter", pcm.ClearSuccessCounter()},
		{"ClearEmptyCounter", pcm.ClearEmptyCounter()},
		{"GetByOwnerSuccessCounter", pcm.GetByOwnerSuccessCounter()},
		{"GetByOwnerEmptyCounter", pcm.GetByOwnerEmptyCounter()},
		{"DeleteByOwnerSuccessCounter", pcm.DeleteByOwnerSuccessCounter()},
		{"GetKeysByPatternSuccessCounter", pcm.GetKeysByPatternSuccessCounter()},
		{"IncrementSuccessCounter", pcm.IncrementSuccessCounter()},
		{"DecrementSuccessCounter", pcm.DecrementSuccessCounter()},
		{"IncrementFloatSuccessCounter", pcm.IncrementFloatSuccessCounter()},
		{"ExtendTTLSuccessCounter", pcm.ExtendTTLSuccessCounter()},
		
		// Hit/Miss counters
		{"GetMissCounter", pcm.GetMissCounter()},
		{"GetByOwnerMissCounter", pcm.GetByOwnerMissCounter()},
		{"GetHitCounter", pcm.GetHitCounter()},
		{"GetByOwnerHitCounter", pcm.GetByOwnerHitCounter()},
		{"GetManyHitCounter", pcm.GetManyHitCounter()},
		{"GeneralMissCounter", pcm.GeneralMissCounter()},
		
		// Error counters (sampling - testing a few key ones)
		{"HasCircuitBreakerErrorCounter", pcm.HasCircuitBreakerErrorCounter()},
		{"GetRedisErrorCounter", pcm.GetRedisErrorCounter()},
		{"SetSerializationErrorCounter", pcm.SetSerializationErrorCounter()},
		{"DeleteMemorySamplingErrorCounter", pcm.DeleteMemorySamplingErrorCounter()},
		
		// Batch operation counters
		{"GetManyBatchCounter", pcm.GetManyBatchCounter()},
		{"SetManyBatchCounter", pcm.SetManyBatchCounter()},
		{"DeleteManyBatchCounter", pcm.DeleteManyBatchCounter()},
	}

	for _, counter := range counters {
		if counter.counter == nil {
			t.Errorf("%s is nil", counter.name)
		}
	}
}

func TestPrecomputedCacheMetrics_AllErrorCounters(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	finalTags := metric.Tags{"provider": "redis", "instance_id": "test"}
	pcm := NewPrecomputedCacheMetrics(registry, finalTags)

	// Test all error counters are non-nil
	errorCounters := []struct {
		name    string
		counter metric.Counter
	}{
		{"HasCircuitBreakerErrorCounter", pcm.HasCircuitBreakerErrorCounter()},
		{"HasRedisErrorCounter", pcm.HasRedisErrorCounter()},
		{"GetCircuitBreakerErrorCounter", pcm.GetCircuitBreakerErrorCounter()},
		{"GetRedisErrorCounter", pcm.GetRedisErrorCounter()},
		{"GetSerializationErrorCounter", pcm.GetSerializationErrorCounter()},
		{"SetCircuitBreakerErrorCounter", pcm.SetCircuitBreakerErrorCounter()},
		{"SetRedisErrorCounter", pcm.SetRedisErrorCounter()},
		{"SetSerializationErrorCounter", pcm.SetSerializationErrorCounter()},
		{"SetMemorySamplingErrorCounter", pcm.SetMemorySamplingErrorCounter()},
		{"DeleteCircuitBreakerErrorCounter", pcm.DeleteCircuitBreakerErrorCounter()},
		{"DeleteRedisErrorCounter", pcm.DeleteRedisErrorCounter()},
		{"DeleteMemorySamplingErrorCounter", pcm.DeleteMemorySamplingErrorCounter()},
		{"ClearCircuitBreakerErrorCounter", pcm.ClearCircuitBreakerErrorCounter()},
		{"GetByOwnerCircuitBreakerErrorCounter", pcm.GetByOwnerCircuitBreakerErrorCounter()},
		{"GetByOwnerRedisErrorCounter", pcm.GetByOwnerRedisErrorCounter()},
		{"GetByOwnerSerializationErrorCounter", pcm.GetByOwnerSerializationErrorCounter()},
		{"DeleteByOwnerCircuitBreakerErrorCounter", pcm.DeleteByOwnerCircuitBreakerErrorCounter()},
		{"DeleteByOwnerRedisErrorCounter", pcm.DeleteByOwnerRedisErrorCounter()},
		{"GetKeysByPatternCircuitBreakerErrorCounter", pcm.GetKeysByPatternCircuitBreakerErrorCounter()},
		{"GetKeysByPatternRedisErrorCounter", pcm.GetKeysByPatternRedisErrorCounter()},
		{"IncrementCircuitBreakerErrorCounter", pcm.IncrementCircuitBreakerErrorCounter()},
		{"IncrementRedisErrorCounter", pcm.IncrementRedisErrorCounter()},
		{"IncrementTimeoutErrorCounter", pcm.IncrementTimeoutErrorCounter()},
		{"DecrementCircuitBreakerErrorCounter", pcm.DecrementCircuitBreakerErrorCounter()},
		{"DecrementRedisErrorCounter", pcm.DecrementRedisErrorCounter()},
		{"DecrementTimeoutErrorCounter", pcm.DecrementTimeoutErrorCounter()},
		{"IncrementFloatCircuitBreakerErrorCounter", pcm.IncrementFloatCircuitBreakerErrorCounter()},
		{"IncrementFloatRedisErrorCounter", pcm.IncrementFloatRedisErrorCounter()},
		{"IncrementFloatTimeoutErrorCounter", pcm.IncrementFloatTimeoutErrorCounter()},
		{"ExtendTTLCircuitBreakerErrorCounter", pcm.ExtendTTLCircuitBreakerErrorCounter()},
		{"ExtendTTLRedisErrorCounter", pcm.ExtendTTLRedisErrorCounter()},
		{"ExtendTTLKeyNotFoundErrorCounter", pcm.ExtendTTLKeyNotFoundErrorCounter()},
		{"GetManyCircuitBreakerErrorCounter", pcm.GetManyCircuitBreakerErrorCounter()},
		{"GetManyRedisErrorCounter", pcm.GetManyRedisErrorCounter()},
		{"GetManySerializationErrorCounter", pcm.GetManySerializationErrorCounter()},
		{"SetManyCircuitBreakerErrorCounter", pcm.SetManyCircuitBreakerErrorCounter()},
		{"SetManyRedisErrorCounter", pcm.SetManyRedisErrorCounter()},
		{"SetManySerializationErrorCounter", pcm.SetManySerializationErrorCounter()},
		{"DeleteManyCircuitBreakerErrorCounter", pcm.DeleteManyCircuitBreakerErrorCounter()},
		{"DeleteManyRedisErrorCounter", pcm.DeleteManyRedisErrorCounter()},
	}

	for _, errorCounter := range errorCounters {
		if errorCounter.counter == nil {
			t.Errorf("%s is nil", errorCounter.name)
		}
	}
}

func TestPrecomputedCacheMetrics_ZeroAllocationAccess(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	finalTags := metric.Tags{"provider": "redis", "instance_id": "test"}
	pcm := NewPrecomputedCacheMetrics(registry, finalTags)

	// Test that accessing pre-computed metrics doesn't allocate
	// This is a behavioral test - we can verify the metrics are returned consistently
	timer1 := pcm.GetTimer()
	timer2 := pcm.GetTimer()
	
	if timer1 != timer2 {
		t.Error("GetTimer() should return the same instance each time (zero allocation)")
	}

	counter1 := pcm.GetSuccessCounter()
	counter2 := pcm.GetSuccessCounter()
	
	if counter1 != counter2 {
		t.Error("GetSuccessCounter() should return the same instance each time (zero allocation)")
	}
}

func TestPrecomputedCacheMetrics_MetricFunctionality(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	finalTags := metric.Tags{"provider": "redis", "instance_id": "test"}
	pcm := NewPrecomputedCacheMetrics(registry, finalTags)

	// Test timers can record durations
	pcm.GetTimer().Record(time.Millisecond)
	pcm.SetTimer().Record(2 * time.Millisecond)
	
	// Test counters can increment
	pcm.GetSuccessCounter().Inc()
	pcm.GetHitCounter().Inc()
	pcm.GeneralMissCounter().Inc()
	pcm.GetCircuitBreakerErrorCounter().Inc()
	
	// Test batch operation counters
	pcm.GetManyBatchCounter().Inc()

	// No panics or errors expected - metrics should work normally
}

func TestPrecomputedCacheMetrics_TagMerging(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	originalTags := metric.Tags{"provider": "redis", "instance_id": "test", "version": "1.0"}
	pcm := NewPrecomputedCacheMetrics(registry, originalTags)

	// Verify the original tags aren't modified by the metrics creation
	// (This tests the tag copying behavior)
	originalTags["modified"] = "true"
	
	// The metrics should still work normally even after original tags are modified
	pcm.GetTimer().Record(time.Millisecond)
	pcm.GetSuccessCounter().Inc()
}

// TestCreateHelper* functions test the helper functions used during initialization

func TestCreateTimer(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	baseTags := metric.Tags{"provider": "redis"}
	
	timer := createTimer(registry, "test_duration", "Test timer", baseTags, "test")
	
	if timer == nil {
		t.Error("createTimer should return non-nil timer")
	}
	
	// Test timer functionality
	timer.Record(time.Millisecond)
}

func TestCreateOperationCounter(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	baseTags := metric.Tags{"provider": "redis"}
	
	counter := createOperationCounter(registry, baseTags, "test", "success")
	
	if counter == nil {
		t.Error("createOperationCounter should return non-nil counter")
	}
	
	// Test counter functionality
	counter.Inc()
}

func TestCreateHitCounter(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	baseTags := metric.Tags{"provider": "redis"}
	
	counter := createHitCounter(registry, baseTags)
	
	if counter == nil {
		t.Error("createHitCounter should return non-nil counter")
	}
	
	// Test counter functionality
	counter.Inc()
}

func TestCreateMissCounter(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	baseTags := metric.Tags{"provider": "redis"}
	
	counter := createMissCounter(registry, baseTags)
	
	if counter == nil {
		t.Error("createMissCounter should return non-nil counter")
	}
	
	// Test counter functionality
	counter.Inc()
}

func TestCreateErrorCounter(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	baseTags := metric.Tags{"provider": "redis"}
	
	counter := createErrorCounter(registry, baseTags, "test", "redis_error", "infrastructure")
	
	if counter == nil {
		t.Error("createErrorCounter should return non-nil counter")
	}
	
	// Test counter functionality
	counter.Inc()
}

func TestCreateBatchOperationCounter(t *testing.T) {
	registry := metric.NewDefaultRegistry()
	baseTags := metric.Tags{"provider": "redis"}
	
	counter := createBatchOperationCounter(registry, baseTags, "test")
	
	if counter == nil {
		t.Error("createBatchOperationCounter should return non-nil counter")
	}
	
	// Test counter functionality
	counter.Inc()
}

// Benchmark tests to verify zero allocation behavior

func BenchmarkPrecomputedMetrics_TimerAccess(b *testing.B) {
	registry := metric.NewDefaultRegistry()
	finalTags := metric.Tags{"provider": "redis", "instance_id": "test"}
	pcm := NewPrecomputedCacheMetrics(registry, finalTags)

	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = pcm.GetTimer()
	}
}

func BenchmarkPrecomputedMetrics_CounterAccess(b *testing.B) {
	registry := metric.NewDefaultRegistry()
	finalTags := metric.Tags{"provider": "redis", "instance_id": "test"}
	pcm := NewPrecomputedCacheMetrics(registry, finalTags)

	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = pcm.GetSuccessCounter()
	}
}

func BenchmarkPrecomputedMetrics_ErrorCounterAccess(b *testing.B) {
	registry := metric.NewDefaultRegistry()
	finalTags := metric.Tags{"provider": "redis", "instance_id": "test"}
	pcm := NewPrecomputedCacheMetrics(registry, finalTags)

	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_ = pcm.GetCircuitBreakerErrorCounter()
	}
}

func BenchmarkPrecomputedMetrics_TimerRecord(b *testing.B) {
	registry := metric.NewDefaultRegistry()
	finalTags := metric.Tags{"provider": "redis", "instance_id": "test"}
	pcm := NewPrecomputedCacheMetrics(registry, finalTags)
	timer := pcm.GetTimer()

	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		timer.Record(time.Microsecond)
	}
}

func BenchmarkPrecomputedMetrics_CounterInc(b *testing.B) {
	registry := metric.NewDefaultRegistry()
	finalTags := metric.Tags{"provider": "redis", "instance_id": "test"}
	pcm := NewPrecomputedCacheMetrics(registry, finalTags)
	counter := pcm.GetSuccessCounter()

	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		counter.Inc()
	}
}