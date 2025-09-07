package metrics

import (
	"github.com/MichaelAJay/go-metrics/metric"
)

// PrecomputedCacheMetrics provides zero-allocation metrics for cache operations
// All metrics are pre-computed with final tags baked in to eliminate runtime allocations
type PrecomputedCacheMetrics struct {
	// Core operation timers (pre-computed with final tags)
	hasTimer                metric.Timer
	getTimer                metric.Timer
	setTimer                metric.Timer
	deleteTimer             metric.Timer
	clearTimer              metric.Timer
	getByOwnerTimer         metric.Timer
	deleteByOwnerTimer      metric.Timer
	getKeysByPatternTimer   metric.Timer
	incrementTimer          metric.Timer
	decrementTimer          metric.Timer
	incrementFloatTimer     metric.Timer
	extendTTLTimer          metric.Timer
	
	// Batch operation timers
	getManyTimer    metric.Timer
	setManyTimer    metric.Timer
	deleteManyTimer metric.Timer
	
	// Success/status counters
	hasSuccessCounter                metric.Counter
	getSuccessCounter                metric.Counter
	setSuccessCounter                metric.Counter
	deleteSuccessCounter             metric.Counter
	clearSuccessCounter              metric.Counter
	clearEmptyCounter                metric.Counter
	getByOwnerSuccessCounter         metric.Counter
	getByOwnerEmptyCounter           metric.Counter
	deleteByOwnerSuccessCounter      metric.Counter
	getKeysByPatternSuccessCounter   metric.Counter
	incrementSuccessCounter          metric.Counter
	decrementSuccessCounter          metric.Counter
	incrementFloatSuccessCounter     metric.Counter
	extendTTLSuccessCounter          metric.Counter
	
	// Miss counters
	getMissCounter                   metric.Counter
	getByOwnerMissCounter           metric.Counter
	
	// Hit counters  
	getHitCounter                    metric.Counter
	getByOwnerHitCounter            metric.Counter
	getManyHitCounter               metric.Counter
	
	// Miss counter (general)
	generalMissCounter               metric.Counter
	
	// Error counters by operation and error type
	hasCircuitBreakerErrorCounter           metric.Counter
	hasRedisErrorCounter                    metric.Counter
	getCircuitBreakerErrorCounter           metric.Counter
	getRedisErrorCounter                    metric.Counter
	getSerializationErrorCounter            metric.Counter
	setCircuitBreakerErrorCounter           metric.Counter
	setRedisErrorCounter                    metric.Counter
	setSerializationErrorCounter            metric.Counter
	setMemorySamplingErrorCounter           metric.Counter
	deleteCircuitBreakerErrorCounter        metric.Counter
	deleteRedisErrorCounter                 metric.Counter
	deleteMemorySamplingErrorCounter        metric.Counter
	clearCircuitBreakerErrorCounter         metric.Counter
	getByOwnerCircuitBreakerErrorCounter    metric.Counter
	getByOwnerRedisErrorCounter             metric.Counter
	getByOwnerSerializationErrorCounter     metric.Counter
	deleteByOwnerCircuitBreakerErrorCounter metric.Counter
	deleteByOwnerRedisErrorCounter          metric.Counter
	getKeysByPatternCircuitBreakerErrorCounter metric.Counter
	getKeysByPatternRedisErrorCounter       metric.Counter
	incrementCircuitBreakerErrorCounter     metric.Counter
	incrementRedisErrorCounter              metric.Counter
	incrementTimeoutErrorCounter            metric.Counter
	decrementCircuitBreakerErrorCounter     metric.Counter
	decrementRedisErrorCounter              metric.Counter
	decrementTimeoutErrorCounter            metric.Counter
	incrementFloatCircuitBreakerErrorCounter metric.Counter
	incrementFloatRedisErrorCounter         metric.Counter
	incrementFloatTimeoutErrorCounter       metric.Counter
	extendTTLCircuitBreakerErrorCounter     metric.Counter
	extendTTLRedisErrorCounter              metric.Counter
	extendTTLKeyNotFoundErrorCounter        metric.Counter
	
	// Batch operation error counters
	getManyCircuitBreakerErrorCounter      metric.Counter
	getManyRedisErrorCounter               metric.Counter
	getManySerializationErrorCounter       metric.Counter
	setManyCircuitBreakerErrorCounter      metric.Counter
	setManyRedisErrorCounter               metric.Counter
	setManySerializationErrorCounter       metric.Counter
	deleteManyCircuitBreakerErrorCounter   metric.Counter
	deleteManyRedisErrorCounter            metric.Counter
	
	// Batch operation counters 
	getManyBatchCounter    metric.Counter
	setManyBatchCounter    metric.Counter
	deleteManyBatchCounter metric.Counter
}

// NewPrecomputedCacheMetrics creates a new PrecomputedCacheMetrics instance
// with all metrics pre-initialized with the provided registry and final tags
func NewPrecomputedCacheMetrics(registry metric.Registry, finalTags metric.Tags) *PrecomputedCacheMetrics {
	if registry == nil {
		registry = metric.NewDefaultRegistry()
	}
	
	// Create a copy of tags to avoid external modifications
	tags := make(metric.Tags)
	if finalTags != nil {
		for k, v := range finalTags {
			tags[k] = v
		}
	}
	
	pcm := &PrecomputedCacheMetrics{}
	
	// Initialize operation timers
	pcm.hasTimer = createTimer(registry, "cache_operation_duration", "Duration of has operations", tags, "has")
	pcm.getTimer = createTimer(registry, "cache_operation_duration", "Duration of get operations", tags, "get")
	pcm.setTimer = createTimer(registry, "cache_operation_duration", "Duration of set operations", tags, "set")
	pcm.deleteTimer = createTimer(registry, "cache_operation_duration", "Duration of delete operations", tags, "delete")
	pcm.clearTimer = createTimer(registry, "cache_operation_duration", "Duration of clear operations", tags, "clear")
	pcm.getByOwnerTimer = createTimer(registry, "cache_operation_duration", "Duration of getbyowner operations", tags, "getbyowner")
	pcm.deleteByOwnerTimer = createTimer(registry, "cache_operation_duration", "Duration of deletebyowner operations", tags, "deletebyowner")
	pcm.getKeysByPatternTimer = createTimer(registry, "cache_operation_duration", "Duration of getkeysbypattern operations", tags, "getkeysbypattern")
	pcm.incrementTimer = createTimer(registry, "cache_operation_duration", "Duration of increment operations", tags, "increment")
	pcm.decrementTimer = createTimer(registry, "cache_operation_duration", "Duration of decrement operations", tags, "decrement")
	pcm.incrementFloatTimer = createTimer(registry, "cache_operation_duration", "Duration of increment_float operations", tags, "increment_float")
	pcm.extendTTLTimer = createTimer(registry, "cache_operation_duration", "Duration of extend_ttl operations", tags, "extend_ttl")
	
	// Initialize batch operation timers
	pcm.getManyTimer = createTimer(registry, "cache_operation_duration", "Duration of getmany operations", tags, "getmany")
	pcm.setManyTimer = createTimer(registry, "cache_operation_duration", "Duration of setmany operations", tags, "setmany")
	pcm.deleteManyTimer = createTimer(registry, "cache_operation_duration", "Duration of deletemany operations", tags, "deletemany")
	
	// Initialize success counters  
	pcm.hasSuccessCounter = createOperationCounter(registry, tags, "has", "success")
	pcm.getSuccessCounter = createOperationCounter(registry, tags, "get", "success")
	pcm.setSuccessCounter = createOperationCounter(registry, tags, "set", "success")
	pcm.deleteSuccessCounter = createOperationCounter(registry, tags, "delete", "success")
	pcm.clearSuccessCounter = createOperationCounter(registry, tags, "clear", "success")
	pcm.clearEmptyCounter = createOperationCounter(registry, tags, "clear", "empty")
	pcm.getByOwnerSuccessCounter = createOperationCounter(registry, tags, "getbyowner", "success")
	pcm.getByOwnerEmptyCounter = createOperationCounter(registry, tags, "getbyowner", "empty")
	pcm.deleteByOwnerSuccessCounter = createOperationCounter(registry, tags, "deletebyowner", "success")
	pcm.getKeysByPatternSuccessCounter = createOperationCounter(registry, tags, "getkeysbypattern", "success")
	pcm.incrementSuccessCounter = createOperationCounter(registry, tags, "increment", "success")
	pcm.decrementSuccessCounter = createOperationCounter(registry, tags, "decrement", "success")
	pcm.incrementFloatSuccessCounter = createOperationCounter(registry, tags, "increment_float", "success")
	pcm.extendTTLSuccessCounter = createOperationCounter(registry, tags, "extend_ttl", "success")
	
	// Initialize miss counters
	pcm.getMissCounter = createOperationCounter(registry, tags, "get", "miss")
	pcm.getByOwnerMissCounter = createOperationCounter(registry, tags, "getbyowner", "empty")
	
	// Initialize hit counters
	pcm.getHitCounter = createHitCounter(registry, tags)
	pcm.getByOwnerHitCounter = createHitCounter(registry, tags)
	pcm.getManyHitCounter = createHitCounter(registry, tags)
	
	// Initialize general miss counter
	pcm.generalMissCounter = createMissCounter(registry, tags)
	
	// Initialize error counters
	pcm.hasCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "has", "circuit_breaker", "availability")
	pcm.hasRedisErrorCounter = createErrorCounter(registry, tags, "has", "redis_error", "infrastructure")
	pcm.getCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "get", "circuit_breaker", "availability")
	pcm.getRedisErrorCounter = createErrorCounter(registry, tags, "get", "redis_error", "infrastructure")
	pcm.getSerializationErrorCounter = createErrorCounter(registry, tags, "get", "serialization_error", "data")
	pcm.setCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "set", "circuit_breaker", "availability")
	pcm.setRedisErrorCounter = createErrorCounter(registry, tags, "set", "redis_error", "infrastructure")
	pcm.setSerializationErrorCounter = createErrorCounter(registry, tags, "set", "serialization_error", "data")
	pcm.setMemorySamplingErrorCounter = createErrorCounter(registry, tags, "set", "memory_sampling_error", "infrastructure")
	pcm.deleteCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "delete", "circuit_breaker", "availability")
	pcm.deleteRedisErrorCounter = createErrorCounter(registry, tags, "delete", "redis_error", "infrastructure")
	pcm.deleteMemorySamplingErrorCounter = createErrorCounter(registry, tags, "delete", "memory_sampling_error", "infrastructure")
	pcm.clearCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "clear", "circuit_breaker", "availability")
	pcm.getByOwnerCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "getbyowner", "circuit_breaker", "availability")
	pcm.getByOwnerRedisErrorCounter = createErrorCounter(registry, tags, "getbyowner", "redis_error", "infrastructure")
	pcm.getByOwnerSerializationErrorCounter = createErrorCounter(registry, tags, "getbyowner", "serialization_error", "data")
	pcm.deleteByOwnerCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "deletebyowner", "circuit_breaker", "availability")
	pcm.deleteByOwnerRedisErrorCounter = createErrorCounter(registry, tags, "deletebyowner", "redis_error", "infrastructure")
	pcm.getKeysByPatternCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "getkeysbypattern", "circuit_breaker", "availability")
	pcm.getKeysByPatternRedisErrorCounter = createErrorCounter(registry, tags, "getkeysbypattern", "redis_error", "infrastructure")
	pcm.incrementCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "increment", "circuit_breaker", "availability")
	pcm.incrementRedisErrorCounter = createErrorCounter(registry, tags, "increment", "redis_error", "infrastructure")
	pcm.incrementTimeoutErrorCounter = createErrorCounter(registry, tags, "increment", "timeout", "infrastructure")
	pcm.decrementCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "decrement", "circuit_breaker", "availability")
	pcm.decrementRedisErrorCounter = createErrorCounter(registry, tags, "decrement", "redis_error", "infrastructure")
	pcm.decrementTimeoutErrorCounter = createErrorCounter(registry, tags, "decrement", "timeout", "infrastructure")
	pcm.incrementFloatCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "increment_float", "circuit_breaker", "availability")
	pcm.incrementFloatRedisErrorCounter = createErrorCounter(registry, tags, "increment_float", "redis_error", "infrastructure")
	pcm.incrementFloatTimeoutErrorCounter = createErrorCounter(registry, tags, "increment_float", "timeout", "infrastructure")
	pcm.extendTTLCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "extend_ttl", "circuit_breaker", "availability")
	pcm.extendTTLRedisErrorCounter = createErrorCounter(registry, tags, "extend_ttl", "redis_error", "infrastructure")
	pcm.extendTTLKeyNotFoundErrorCounter = createErrorCounter(registry, tags, "extend_ttl", "key_not_found", "data")
	
	// Initialize batch operation error counters
	pcm.getManyCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "getmany", "circuit_breaker", "availability")
	pcm.getManyRedisErrorCounter = createErrorCounter(registry, tags, "getmany", "redis_error", "infrastructure")
	pcm.getManySerializationErrorCounter = createErrorCounter(registry, tags, "getmany", "serialization_error", "data")
	pcm.setManyCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "setmany", "circuit_breaker", "availability")
	pcm.setManyRedisErrorCounter = createErrorCounter(registry, tags, "setmany", "redis_error", "infrastructure")
	pcm.setManySerializationErrorCounter = createErrorCounter(registry, tags, "setmany", "serialization_error", "data")
	pcm.deleteManyCircuitBreakerErrorCounter = createErrorCounter(registry, tags, "deletemany", "circuit_breaker", "availability")
	pcm.deleteManyRedisErrorCounter = createErrorCounter(registry, tags, "deletemany", "redis_error", "infrastructure")
	
	// Initialize batch operation counters
	pcm.getManyBatchCounter = createBatchOperationCounter(registry, tags, "getmany")
	pcm.setManyBatchCounter = createBatchOperationCounter(registry, tags, "setmany")
	pcm.deleteManyBatchCounter = createBatchOperationCounter(registry, tags, "deletemany")
	
	return pcm
}

// Helper functions to create metrics with pre-baked tags

func createTimer(registry metric.Registry, name, description string, baseTags metric.Tags, operation string) metric.Timer {
	tags := make(metric.Tags, len(baseTags)+1)
	for k, v := range baseTags {
		tags[k] = v
	}
	tags["operation"] = operation
	
	return registry.Timer(metric.Options{
		Name:        name,
		Description: description,
		Unit:        "nanoseconds",
		Tags:        tags,
	})
}

func createOperationCounter(registry metric.Registry, baseTags metric.Tags, operation, status string) metric.Counter {
	tags := make(metric.Tags, len(baseTags)+2)
	for k, v := range baseTags {
		tags[k] = v
	}
	tags["operation"] = operation
	tags["status"] = status
	
	return registry.Counter(metric.Options{
		Name:        "cache_operations_total",
		Description: "Total number of cache operations",
		Unit:        "count",
		Tags:        tags,
	})
}

func createHitCounter(registry metric.Registry, baseTags metric.Tags) metric.Counter {
	return registry.Counter(metric.Options{
		Name:        "cache_hits_total",
		Description: "Total number of cache hits",
		Unit:        "count",
		Tags:        baseTags,
	})
}

func createMissCounter(registry metric.Registry, baseTags metric.Tags) metric.Counter {
	return registry.Counter(metric.Options{
		Name:        "cache_misses_total",
		Description: "Total number of cache misses",
		Unit:        "count",
		Tags:        baseTags,
	})
}

func createErrorCounter(registry metric.Registry, baseTags metric.Tags, operation, errorType, errorCategory string) metric.Counter {
	tags := make(metric.Tags, len(baseTags)+3)
	for k, v := range baseTags {
		tags[k] = v
	}
	tags["operation"] = operation
	tags["error_type"] = errorType
	tags["error_category"] = errorCategory
	
	return registry.Counter(metric.Options{
		Name:        "cache_security_events_total",
		Description: "Total number of cache security events",
		Unit:        "count",
		Tags:        tags,
	})
}

func createBatchOperationCounter(registry metric.Registry, baseTags metric.Tags, operation string) metric.Counter {
	tags := make(metric.Tags, len(baseTags)+1)
	for k, v := range baseTags {
		tags[k] = v
	}
	tags["operation"] = "batch_" + operation
	tags["status"] = "completed"
	
	return registry.Counter(metric.Options{
		Name:        "cache_operations_total",
		Description: "Total number of cache operations",
		Unit:        "count",
		Tags:        tags,
	})
}

// Access methods provide zero-allocation metric access
// All metrics are pre-computed, so these simply return the cached instances

// Timer access methods
func (pcm *PrecomputedCacheMetrics) HasTimer() metric.Timer { return pcm.hasTimer }
func (pcm *PrecomputedCacheMetrics) GetTimer() metric.Timer { return pcm.getTimer }
func (pcm *PrecomputedCacheMetrics) SetTimer() metric.Timer { return pcm.setTimer }
func (pcm *PrecomputedCacheMetrics) DeleteTimer() metric.Timer { return pcm.deleteTimer }
func (pcm *PrecomputedCacheMetrics) ClearTimer() metric.Timer { return pcm.clearTimer }
func (pcm *PrecomputedCacheMetrics) GetByOwnerTimer() metric.Timer { return pcm.getByOwnerTimer }
func (pcm *PrecomputedCacheMetrics) DeleteByOwnerTimer() metric.Timer { return pcm.deleteByOwnerTimer }
func (pcm *PrecomputedCacheMetrics) GetKeysByPatternTimer() metric.Timer { return pcm.getKeysByPatternTimer }
func (pcm *PrecomputedCacheMetrics) IncrementTimer() metric.Timer { return pcm.incrementTimer }
func (pcm *PrecomputedCacheMetrics) DecrementTimer() metric.Timer { return pcm.decrementTimer }
func (pcm *PrecomputedCacheMetrics) IncrementFloatTimer() metric.Timer { return pcm.incrementFloatTimer }
func (pcm *PrecomputedCacheMetrics) ExtendTTLTimer() metric.Timer { return pcm.extendTTLTimer }

// Batch timer access methods
func (pcm *PrecomputedCacheMetrics) GetManyTimer() metric.Timer { return pcm.getManyTimer }
func (pcm *PrecomputedCacheMetrics) SetManyTimer() metric.Timer { return pcm.setManyTimer }
func (pcm *PrecomputedCacheMetrics) DeleteManyTimer() metric.Timer { return pcm.deleteManyTimer }

// Success counter access methods
func (pcm *PrecomputedCacheMetrics) HasSuccessCounter() metric.Counter { return pcm.hasSuccessCounter }
func (pcm *PrecomputedCacheMetrics) GetSuccessCounter() metric.Counter { return pcm.getSuccessCounter }
func (pcm *PrecomputedCacheMetrics) SetSuccessCounter() metric.Counter { return pcm.setSuccessCounter }
func (pcm *PrecomputedCacheMetrics) DeleteSuccessCounter() metric.Counter { return pcm.deleteSuccessCounter }
func (pcm *PrecomputedCacheMetrics) ClearSuccessCounter() metric.Counter { return pcm.clearSuccessCounter }
func (pcm *PrecomputedCacheMetrics) ClearEmptyCounter() metric.Counter { return pcm.clearEmptyCounter }
func (pcm *PrecomputedCacheMetrics) GetByOwnerSuccessCounter() metric.Counter { return pcm.getByOwnerSuccessCounter }
func (pcm *PrecomputedCacheMetrics) GetByOwnerEmptyCounter() metric.Counter { return pcm.getByOwnerEmptyCounter }
func (pcm *PrecomputedCacheMetrics) DeleteByOwnerSuccessCounter() metric.Counter { return pcm.deleteByOwnerSuccessCounter }
func (pcm *PrecomputedCacheMetrics) GetKeysByPatternSuccessCounter() metric.Counter { return pcm.getKeysByPatternSuccessCounter }
func (pcm *PrecomputedCacheMetrics) IncrementSuccessCounter() metric.Counter { return pcm.incrementSuccessCounter }
func (pcm *PrecomputedCacheMetrics) DecrementSuccessCounter() metric.Counter { return pcm.decrementSuccessCounter }
func (pcm *PrecomputedCacheMetrics) IncrementFloatSuccessCounter() metric.Counter { return pcm.incrementFloatSuccessCounter }
func (pcm *PrecomputedCacheMetrics) ExtendTTLSuccessCounter() metric.Counter { return pcm.extendTTLSuccessCounter }

// Miss counter access methods
func (pcm *PrecomputedCacheMetrics) GetMissCounter() metric.Counter { return pcm.getMissCounter }
func (pcm *PrecomputedCacheMetrics) GetByOwnerMissCounter() metric.Counter { return pcm.getByOwnerMissCounter }

// Hit counter access methods
func (pcm *PrecomputedCacheMetrics) GetHitCounter() metric.Counter { return pcm.getHitCounter }
func (pcm *PrecomputedCacheMetrics) GetByOwnerHitCounter() metric.Counter { return pcm.getByOwnerHitCounter }
func (pcm *PrecomputedCacheMetrics) GetManyHitCounter() metric.Counter { return pcm.getManyHitCounter }

// General miss counter
func (pcm *PrecomputedCacheMetrics) GeneralMissCounter() metric.Counter { return pcm.generalMissCounter }

// Error counter access methods
func (pcm *PrecomputedCacheMetrics) HasCircuitBreakerErrorCounter() metric.Counter { return pcm.hasCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) HasRedisErrorCounter() metric.Counter { return pcm.hasRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) GetCircuitBreakerErrorCounter() metric.Counter { return pcm.getCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) GetRedisErrorCounter() metric.Counter { return pcm.getRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) GetSerializationErrorCounter() metric.Counter { return pcm.getSerializationErrorCounter }
func (pcm *PrecomputedCacheMetrics) SetCircuitBreakerErrorCounter() metric.Counter { return pcm.setCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) SetRedisErrorCounter() metric.Counter { return pcm.setRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) SetSerializationErrorCounter() metric.Counter { return pcm.setSerializationErrorCounter }
func (pcm *PrecomputedCacheMetrics) SetMemorySamplingErrorCounter() metric.Counter { return pcm.setMemorySamplingErrorCounter }
func (pcm *PrecomputedCacheMetrics) DeleteCircuitBreakerErrorCounter() metric.Counter { return pcm.deleteCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) DeleteRedisErrorCounter() metric.Counter { return pcm.deleteRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) DeleteMemorySamplingErrorCounter() metric.Counter { return pcm.deleteMemorySamplingErrorCounter }
func (pcm *PrecomputedCacheMetrics) ClearCircuitBreakerErrorCounter() metric.Counter { return pcm.clearCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) GetByOwnerCircuitBreakerErrorCounter() metric.Counter { return pcm.getByOwnerCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) GetByOwnerRedisErrorCounter() metric.Counter { return pcm.getByOwnerRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) GetByOwnerSerializationErrorCounter() metric.Counter { return pcm.getByOwnerSerializationErrorCounter }
func (pcm *PrecomputedCacheMetrics) DeleteByOwnerCircuitBreakerErrorCounter() metric.Counter { return pcm.deleteByOwnerCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) DeleteByOwnerRedisErrorCounter() metric.Counter { return pcm.deleteByOwnerRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) GetKeysByPatternCircuitBreakerErrorCounter() metric.Counter { return pcm.getKeysByPatternCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) GetKeysByPatternRedisErrorCounter() metric.Counter { return pcm.getKeysByPatternRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) IncrementCircuitBreakerErrorCounter() metric.Counter { return pcm.incrementCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) IncrementRedisErrorCounter() metric.Counter { return pcm.incrementRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) IncrementTimeoutErrorCounter() metric.Counter { return pcm.incrementTimeoutErrorCounter }
func (pcm *PrecomputedCacheMetrics) DecrementCircuitBreakerErrorCounter() metric.Counter { return pcm.decrementCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) DecrementRedisErrorCounter() metric.Counter { return pcm.decrementRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) DecrementTimeoutErrorCounter() metric.Counter { return pcm.decrementTimeoutErrorCounter }
func (pcm *PrecomputedCacheMetrics) IncrementFloatCircuitBreakerErrorCounter() metric.Counter { return pcm.incrementFloatCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) IncrementFloatRedisErrorCounter() metric.Counter { return pcm.incrementFloatRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) IncrementFloatTimeoutErrorCounter() metric.Counter { return pcm.incrementFloatTimeoutErrorCounter }
func (pcm *PrecomputedCacheMetrics) ExtendTTLCircuitBreakerErrorCounter() metric.Counter { return pcm.extendTTLCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) ExtendTTLRedisErrorCounter() metric.Counter { return pcm.extendTTLRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) ExtendTTLKeyNotFoundErrorCounter() metric.Counter { return pcm.extendTTLKeyNotFoundErrorCounter }

// Batch operation error counter access methods
func (pcm *PrecomputedCacheMetrics) GetManyCircuitBreakerErrorCounter() metric.Counter { return pcm.getManyCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) GetManyRedisErrorCounter() metric.Counter { return pcm.getManyRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) GetManySerializationErrorCounter() metric.Counter { return pcm.getManySerializationErrorCounter }
func (pcm *PrecomputedCacheMetrics) SetManyCircuitBreakerErrorCounter() metric.Counter { return pcm.setManyCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) SetManyRedisErrorCounter() metric.Counter { return pcm.setManyRedisErrorCounter }
func (pcm *PrecomputedCacheMetrics) SetManySerializationErrorCounter() metric.Counter { return pcm.setManySerializationErrorCounter }
func (pcm *PrecomputedCacheMetrics) DeleteManyCircuitBreakerErrorCounter() metric.Counter { return pcm.deleteManyCircuitBreakerErrorCounter }
func (pcm *PrecomputedCacheMetrics) DeleteManyRedisErrorCounter() metric.Counter { return pcm.deleteManyRedisErrorCounter }

// Batch operation counter access methods  
func (pcm *PrecomputedCacheMetrics) GetManyBatchCounter() metric.Counter { return pcm.getManyBatchCounter }
func (pcm *PrecomputedCacheMetrics) SetManyBatchCounter() metric.Counter { return pcm.setManyBatchCounter }
func (pcm *PrecomputedCacheMetrics) DeleteManyBatchCounter() metric.Counter { return pcm.deleteManyBatchCounter }