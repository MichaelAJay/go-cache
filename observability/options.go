package observability

import (
	"log/slog"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Options struct {
	// Logging
	Logger   *slog.Logger
	LogLevel slog.Level
	Redactor func(key string, val any) any

	// Metrics/Tracing (OTel)
	Meter           metric.Meter
	Tracer          trace.Tracer
	MetricNamespace string
	EnableExemplars bool

	// Cardinality & sampling
	MaxLabelValues  map[string]int
	SampleDebugLogs bool
	// TraceSampler todo

	// Feature flags
	EmitVerboseMetrics bool
}

// DefaultOptions returns production-ready defaults
func DefaultOptions(serviecName string) Options {
	return Options{
		Logger:             slog.Default(),
		LogLevel:           slog.LevelInfo,
		MetricNamespace:    serviecName,
		EnableExemplars:    true,
		SampleDebugLogs:    false,
		EmitVerboseMetrics: false,
	}
}

// WithLogger returns a copy with custom logger
func (o Options) WithLogger(logger *slog.Logger) Options {
	o.Logger = logger
	return o
}

// todo - remaining fields
