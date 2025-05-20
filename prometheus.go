package cache

import (
	"errors"
	"fmt"
	"net/http"
)

// PrometheusExposer is an interface for types that can expose a Prometheus HTTP handler
type PrometheusExposer interface {
	GetPrometheusHandler() (any, bool)
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
