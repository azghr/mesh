package telemetry

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP request metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"service", "method", "endpoint", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method", "endpoint"},
	)

	httpRequestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of HTTP requests being processed",
		},
		[]string{"service"},
	)

	// gRPC request metrics
	grpcRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests",
		},
		[]string{"service", "method", "status"},
	)

	grpcRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "gRPC request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)

	// Database metrics
	dbQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		},
		[]string{"service", "operation", "status"},
	)

	dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "operation"},
	)

	dbConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Current number of active database connections",
		},
		[]string{"service"},
	)

	// External API metrics
	externalAPICallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "external_api_calls_total",
			Help: "Total number of external API calls",
		},
		[]string{"service", "provider", "endpoint", "status"},
	)

	externalAPIDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "external_api_duration_seconds",
			Help:    "External API call latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "provider", "endpoint"},
	)

	// Cache metrics
	cacheHitsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"service", "cache"},
	)

	cacheMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"service", "cache"},
	)

	// Business logic metrics
	aiRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_requests_total",
			Help: "Total number of AI requests",
		},
		[]string{"service", "model", "status"},
	)

	aiRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_request_duration_seconds",
			Help:    "AI request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "model"},
	)

	tokenSyncsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "token_syncs_total",
			Help: "Total number of token sync operations",
		},
		[]string{"service", "status"},
	)

	tokenSyncDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "token_sync_duration_seconds",
			Help:    "Token sync operation duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service"},
	)
)

// HTTP Metrics

// RecordHTTPRequest records an HTTP request
func RecordHTTPRequest(service, method, endpoint string, statusCode int, duration time.Duration) {
	status := getStatusLabel(statusCode)
	httpRequestsTotal.WithLabelValues(service, method, endpoint, status).Inc()
	httpRequestDuration.WithLabelValues(service, method, endpoint).Observe(duration.Seconds())
}

// IncrementHTTPRequestsInFlight increments the in-flight HTTP requests gauge
func IncrementHTTPRequestsInFlight(service string) {
	httpRequestsInFlight.WithLabelValues(service).Inc()
}

// DecrementHTTPRequestsInFlight decrements the in-flight HTTP requests gauge
func DecrementHTTPRequestsInFlight(service string) {
	httpRequestsInFlight.WithLabelValues(service).Dec()
}

// gRPC Metrics

// RecordGRPCRequest records a gRPC request
func RecordGRPCRequest(service, method, status string, duration time.Duration) {
	grpcRequestsTotal.WithLabelValues(service, method, status).Inc()
	grpcRequestDuration.WithLabelValues(service, method).Observe(duration.Seconds())
}

// Database Metrics

// RecordDBQuery records a database query
func RecordDBQuery(service, operation string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}
	dbQueriesTotal.WithLabelValues(service, operation, status).Inc()
	dbQueryDuration.WithLabelValues(service, operation).Observe(duration.Seconds())
}

// SetDBConnections sets the number of active database connections
func SetDBConnections(service string, count float64) {
	dbConnectionsActive.WithLabelValues(service).Set(count)
}

// External API Metrics

// RecordExternalAPICall records an external API call
func RecordExternalAPICall(service, provider, endpoint string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}
	externalAPICallsTotal.WithLabelValues(service, provider, endpoint, status).Inc()
	externalAPIDuration.WithLabelValues(service, provider, endpoint).Observe(duration.Seconds())
}

// Cache Metrics

// RecordCacheHit records a cache hit
func RecordCacheHit(service, cache string) {
	cacheHitsTotal.WithLabelValues(service, cache).Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss(service, cache string) {
	cacheMissesTotal.WithLabelValues(service, cache).Inc()
}

// AI Metrics

// RecordAIRequest records an AI request
func RecordAIRequest(service, model string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}
	aiRequestsTotal.WithLabelValues(service, model, status).Inc()
	aiRequestDuration.WithLabelValues(service, model).Observe(duration.Seconds())
}

// Token Sync Metrics

// RecordTokenSync records a token sync operation
func RecordTokenSync(service string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}
	tokenSyncsTotal.WithLabelValues(service, status).Inc()
	tokenSyncDuration.WithLabelValues(service).Observe(duration.Seconds())
}

// Handler returns the Prometheus metrics HTTP handler
func Handler() http.Handler {
	return promhttp.Handler()
}

// Helper function to convert HTTP status code to label
func getStatusLabel(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500 && statusCode < 600:
		return "5xx"
	default:
		return "unknown"
	}
}
