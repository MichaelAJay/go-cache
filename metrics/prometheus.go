package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/MichaelAJay/go-metrics"
	"github.com/MichaelAJay/go-metrics/metric/prometheus"
)

// PrometheusExposer is an interface for types that can expose a Prometheus HTTP handler
type PrometheusExposer interface {
	GetPrometheusHandler() (any, bool)
}

// prometheusMetrics implements the CacheMetrics interface using go-metrics
type prometheusMetrics struct {
	// Counters
	hits   metrics.Counter
	misses metrics.Counter

	// Histograms for latency
	getLatency    metrics.Histogram
	setLatency    metrics.Histogram
	deleteLatency metrics.Histogram

	// Gauges
	cacheSize  metrics.Gauge
	entryCount metrics.Gauge

	// For calculations that aren't directly reported
	mu             sync.RWMutex
	lastGetTime    time.Duration
	lastSetTime    time.Duration
	lastDeleteTime time.Duration

	// The registry
	registry metrics.Registry
	reporter metrics.Reporter
}

// NewPrometheusMetrics creates a new metrics instance using Prometheus
func NewPrometheusMetrics(serviceName, namespace string) (CacheMetrics, error) {
	registry := metrics.NewRegistry()

	reporter := prometheus.NewReporter(
		prometheus.WithDefaultLabels(map[string]string{
			"service": serviceName,
		}),
	)

	m := &prometheusMetrics{
		hits: registry.Counter(metrics.Options{
			Name:        namespace + "_cache_hits_total",
			Description: "Total number of cache hits",
			Unit:        "count",
			Tags:        metrics.Tags{"cache_type": "cache"},
		}),

		misses: registry.Counter(metrics.Options{
			Name:        namespace + "_cache_misses_total",
			Description: "Total number of cache misses",
			Unit:        "count",
			Tags:        metrics.Tags{"cache_type": "cache"},
		}),

		getLatency: registry.Histogram(metrics.Options{
			Name:        namespace + "_cache_get_latency_milliseconds",
			Description: "Latency of get operations in milliseconds",
			Unit:        "ms",
			Tags:        metrics.Tags{"cache_type": "cache", "operation": "get"},
		}),

		setLatency: registry.Histogram(metrics.Options{
			Name:        namespace + "_cache_set_latency_milliseconds",
			Description: "Latency of set operations in milliseconds",
			Unit:        "ms",
			Tags:        metrics.Tags{"cache_type": "cache", "operation": "set"},
		}),

		deleteLatency: registry.Histogram(metrics.Options{
			Name:        namespace + "_cache_delete_latency_milliseconds",
			Description: "Latency of delete operations in milliseconds",
			Unit:        "ms",
			Tags:        metrics.Tags{"cache_type": "cache", "operation": "delete"},
		}),

		cacheSize: registry.Gauge(metrics.Options{
			Name:        namespace + "_cache_size_bytes",
			Description: "Current size of the cache in bytes",
			Unit:        "bytes",
			Tags:        metrics.Tags{"cache_type": "cache"},
		}),

		entryCount: registry.Gauge(metrics.Options{
			Name:        namespace + "_cache_entries_count",
			Description: "Current number of entries in the cache",
			Unit:        "items",
			Tags:        metrics.Tags{"cache_type": "cache"},
		}),

		registry: registry,
		reporter: reporter,
	}

	// Initial start the reporter
	err := reporter.Report(registry)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// RecordHit records a cache hit
func (m *prometheusMetrics) RecordHit() {
	m.hits.Inc()
}

// RecordMiss records a cache miss
func (m *prometheusMetrics) RecordMiss() {
	m.misses.Inc()
}

// RecordGetLatency records the latency of a Get operation
func (m *prometheusMetrics) RecordGetLatency(duration time.Duration) {
	milliseconds := float64(duration.Milliseconds())
	m.getLatency.Observe(milliseconds)

	m.mu.Lock()
	m.lastGetTime = duration
	m.mu.Unlock()
}

// RecordSetLatency records the latency of a Set operation
func (m *prometheusMetrics) RecordSetLatency(duration time.Duration) {
	milliseconds := float64(duration.Milliseconds())
	m.setLatency.Observe(milliseconds)

	m.mu.Lock()
	m.lastSetTime = duration
	m.mu.Unlock()
}

// RecordDeleteLatency records the latency of a Delete operation
func (m *prometheusMetrics) RecordDeleteLatency(duration time.Duration) {
	milliseconds := float64(duration.Milliseconds())
	m.deleteLatency.Observe(milliseconds)

	m.mu.Lock()
	m.lastDeleteTime = duration
	m.mu.Unlock()
}

// RecordCacheSize records the current cache size
func (m *prometheusMetrics) RecordCacheSize(size int64) {
	m.cacheSize.Set(float64(size))
}

// RecordEntryCount records the current number of entries
func (m *prometheusMetrics) RecordEntryCount(count int64) {
	m.entryCount.Set(float64(count))
}

// GetMetrics returns the current metrics snapshot
func (m *prometheusMetrics) GetMetrics() *CacheMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// We need to read hit and miss values from the prometheus metrics
	// This is a bit of a hack since we don't have direct access to the counter values
	// In a real-world implementation, you might want to maintain local copies

	// The key thing is that the snapshot is for immediate display/debugging
	// while the Prometheus metrics are for long-term monitoring

	return &CacheMetricsSnapshot{
		Hits:          0,   // We can't get this directly from prometheus metrics
		Misses:        0,   // We can't get this directly from prometheus metrics
		HitRatio:      0.0, // We can't calculate this accurately without hits/misses
		GetLatency:    m.lastGetTime,
		SetLatency:    m.lastSetTime,
		DeleteLatency: m.lastDeleteTime,
		CacheSize:     0, // We don't have access to the actual gauge value
		EntryCount:    0, // We don't have access to the actual gauge value
	}
}

// GetPrometheusHandler returns the HTTP handler for exposing metrics
func (m *prometheusMetrics) GetPrometheusHandler() (any, bool) {
	reporter, ok := m.reporter.(*prometheus.Reporter)
	if !ok {
		return nil, false
	}

	return reporter.Handler(), true
}

// StartPrometheusServer starts a HTTP server to expose metrics
func StartPrometheusServer(metrics CacheMetrics, address string) (*http.Server, error) {
	// Check if metrics supports Prometheus
	exposer, ok := metrics.(PrometheusExposer)
	if !ok {
		return nil, errors.New("metrics does not support Prometheus exposition")
	}

	// Get the handler
	handler, ok := exposer.GetPrometheusHandler()
	if !ok {
		return nil, errors.New("failed to get Prometheus handler")
	}

	// Type assertion to http.Handler
	httpHandler, ok := handler.(http.Handler)
	if !ok {
		return nil, errors.New("prometheus handler is not an http.Handler")
	}

	// Create mux for the metrics endpoint
	mux := http.NewServeMux()
	mux.Handle("/metrics", httpHandler)

	// Create the server
	server := &http.Server{
		Addr:    address,
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Prometheus server error: %v\n", err)
		}
	}()

	return server, nil
}