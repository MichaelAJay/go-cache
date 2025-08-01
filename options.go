package cache

import (
	"time"

	"github.com/MichaelAJay/go-cache/interfaces"
	"github.com/MichaelAJay/go-logger"
	"github.com/MichaelAJay/go-metrics/metric"
)

// Re-export types from interfaces package for backward compatibility
type CacheOption = interfaces.CacheOption
type SecurityConfig = interfaces.SecurityConfig
type CacheHooks = interfaces.CacheHooks
type CleanupConfig = interfaces.CleanupConfig
type MetricsConfig = interfaces.MetricsConfig
type CacheOptions = interfaces.CacheOptions
type RedisOptions = interfaces.RedisOptions

// Re-export option functions from interfaces package for backward compatibility
var WithTTL = interfaces.WithTTL
var WithMaxEntries = interfaces.WithMaxEntries
var WithCleanupInterval = interfaces.WithCleanupInterval
var WithLogger = interfaces.WithLogger
var WithRedisOptions = interfaces.WithRedisOptions
var WithMaxSize = interfaces.WithMaxSize
var WithGoMetricsRegistry = interfaces.WithGoMetricsRegistry
var WithMetricsEnabled = interfaces.WithMetricsEnabled
var WithGlobalMetricsTags = interfaces.WithGlobalMetricsTags
var WithSecurityConfig = interfaces.WithSecurityConfig
var WithHooks = interfaces.WithHooks
var WithIndexes = interfaces.WithIndexes
var WithCleanupConfig = interfaces.WithCleanupConfig
var WithMetricsConfig = interfaces.WithMetricsConfig
var WithPrometheusMetrics = interfaces.WithPrometheusMetrics
var WithOpenTelemetryMetrics = interfaces.WithOpenTelemetryMetrics
var WithServiceTags = interfaces.WithServiceTags
var WithDetailedMetrics = interfaces.WithDetailedMetrics
var WithMetricsPrefix = interfaces.WithMetricsPrefix
