package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Handler provides HTTP endpoints for metrics
type Handler struct {
	metrics *Metrics
}

// NewHandler creates a new metrics handler
func NewHandler(metrics *Metrics) *Handler {
	return &Handler{
		metrics: metrics,
	}
}

// HandleJSON returns metrics in JSON format
func (h *Handler) HandleJSON(w http.ResponseWriter, r *http.Request) {
	snapshot := h.metrics.GetSnapshot()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
	}
}

// HandlePrometheus returns metrics in Prometheus format
func (h *Handler) HandlePrometheus(w http.ResponseWriter, r *http.Request) {
	snapshot := h.metrics.GetSnapshot()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)

	var sb strings.Builder

	// Gateway info
	sb.WriteString("# HELP gateway_info Gateway information\n")
	sb.WriteString("# TYPE gateway_info gauge\n")
	sb.WriteString("gateway_info{version=\"1.0.0\"} 1\n\n")

	// Uptime
	sb.WriteString("# HELP gateway_uptime_seconds Gateway uptime in seconds\n")
	sb.WriteString("# TYPE gateway_uptime_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("gateway_uptime_seconds %.2f\n\n", snapshot.UptimeSeconds))

	// Total requests
	sb.WriteString("# HELP gateway_requests_total Total number of requests\n")
	sb.WriteString("# TYPE gateway_requests_total counter\n")
	sb.WriteString(fmt.Sprintf("gateway_requests_total %d\n\n", snapshot.TotalRequests))

	// Successful requests
	sb.WriteString("# HELP gateway_requests_successful_total Total number of successful requests\n")
	sb.WriteString("# TYPE gateway_requests_successful_total counter\n")
	sb.WriteString(fmt.Sprintf("gateway_requests_successful_total %d\n\n", snapshot.SuccessfulRequests))

	// Failed requests
	sb.WriteString("# HELP gateway_requests_failed_total Total number of failed requests\n")
	sb.WriteString("# TYPE gateway_requests_failed_total counter\n")
	sb.WriteString(fmt.Sprintf("gateway_requests_failed_total %d\n\n", snapshot.FailedRequests))

	// Success rate
	sb.WriteString("# HELP gateway_success_rate Success rate percentage\n")
	sb.WriteString("# TYPE gateway_success_rate gauge\n")
	sb.WriteString(fmt.Sprintf("gateway_success_rate %.2f\n\n", snapshot.SuccessRate))

	// Average latency
	sb.WriteString("# HELP gateway_latency_avg_ms Average latency in milliseconds\n")
	sb.WriteString("# TYPE gateway_latency_avg_ms gauge\n")
	sb.WriteString(fmt.Sprintf("gateway_latency_avg_ms %.2f\n\n", snapshot.AvgLatencyMs))

	// Requests per second
	sb.WriteString("# HELP gateway_requests_per_second Requests per second\n")
	sb.WriteString("# TYPE gateway_requests_per_second gauge\n")
	sb.WriteString(fmt.Sprintf("gateway_requests_per_second %.2f\n\n", snapshot.RequestsPerSecond))

	// Cache metrics
	sb.WriteString("# HELP gateway_cache_hits_total Total cache hits\n")
	sb.WriteString("# TYPE gateway_cache_hits_total counter\n")
	sb.WriteString(fmt.Sprintf("gateway_cache_hits_total %d\n\n", snapshot.CacheHits))

	sb.WriteString("# HELP gateway_cache_misses_total Total cache misses\n")
	sb.WriteString("# TYPE gateway_cache_misses_total counter\n")
	sb.WriteString(fmt.Sprintf("gateway_cache_misses_total %d\n\n", snapshot.CacheMisses))

	sb.WriteString("# HELP gateway_cache_hit_rate Cache hit rate percentage\n")
	sb.WriteString("# TYPE gateway_cache_hit_rate gauge\n")
	sb.WriteString(fmt.Sprintf("gateway_cache_hit_rate %.2f\n\n", snapshot.CacheHitRate))

	// Circuit breaker trips
	sb.WriteString("# HELP gateway_circuit_breaker_trips_total Total circuit breaker trips\n")
	sb.WriteString("# TYPE gateway_circuit_breaker_trips_total counter\n")
	sb.WriteString(fmt.Sprintf("gateway_circuit_breaker_trips_total %d\n\n", snapshot.CircuitBreakerTrips))

	// Route metrics
	if len(snapshot.Routes) > 0 {
		sb.WriteString("# HELP gateway_route_requests_total Total requests per route\n")
		sb.WriteString("# TYPE gateway_route_requests_total counter\n")

		// Sort routes for consistent output
		routes := make([]string, 0, len(snapshot.Routes))
		for route := range snapshot.Routes {
			routes = append(routes, route)
		}
		sort.Strings(routes)

		for _, route := range routes {
			rm := snapshot.Routes[route]
			sb.WriteString(fmt.Sprintf("gateway_route_requests_total{route=\"%s\"} %d\n", route, rm.Requests))
		}
		sb.WriteString("\n")

		sb.WriteString("# HELP gateway_route_latency_avg_ms Average latency per route in milliseconds\n")
		sb.WriteString("# TYPE gateway_route_latency_avg_ms gauge\n")
		for _, route := range routes {
			rm := snapshot.Routes[route]
			sb.WriteString(fmt.Sprintf("gateway_route_latency_avg_ms{route=\"%s\"} %.2f\n", route, rm.AvgLatencyMs))
		}
		sb.WriteString("\n")
	}

	// Service metrics
	if len(snapshot.Services) > 0 {
		sb.WriteString("# HELP gateway_service_requests_total Total requests per service\n")
		sb.WriteString("# TYPE gateway_service_requests_total counter\n")

		// Sort services for consistent output
		services := make([]string, 0, len(snapshot.Services))
		for service := range snapshot.Services {
			services = append(services, service)
		}
		sort.Strings(services)

		for _, service := range services {
			sm := snapshot.Services[service]
			sb.WriteString(fmt.Sprintf("gateway_service_requests_total{service=\"%s\"} %d\n", service, sm.Requests))
		}
		sb.WriteString("\n")

		sb.WriteString("# HELP gateway_service_latency_avg_ms Average latency per service in milliseconds\n")
		sb.WriteString("# TYPE gateway_service_latency_avg_ms gauge\n")
		for _, service := range services {
			sm := snapshot.Services[service]
			sb.WriteString(fmt.Sprintf("gateway_service_latency_avg_ms{service=\"%s\"} %.2f\n", service, sm.AvgLatencyMs))
		}
		sb.WriteString("\n")

		sb.WriteString("# HELP gateway_service_failures_total Total failures per service\n")
		sb.WriteString("# TYPE gateway_service_failures_total counter\n")
		for _, service := range services {
			sm := snapshot.Services[service]
			sb.WriteString(fmt.Sprintf("gateway_service_failures_total{service=\"%s\"} %d\n", service, sm.Failures))
		}
	}

	w.Write([]byte(sb.String()))
}

// HandleSummary returns a human-readable summary
func (h *Handler) HandleSummary(w http.ResponseWriter, r *http.Request) {
	snapshot := h.metrics.GetSnapshot()

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)

	var sb strings.Builder

	sb.WriteString("=== Hub API Gateway Metrics ===\n\n")

	sb.WriteString("Overall Statistics:\n")
	sb.WriteString(fmt.Sprintf("  Uptime: %.0f seconds (%.1f minutes)\n", snapshot.UptimeSeconds, snapshot.UptimeSeconds/60))
	sb.WriteString(fmt.Sprintf("  Total Requests: %d\n", snapshot.TotalRequests))
	sb.WriteString(fmt.Sprintf("  Successful: %d (%.1f%%)\n", snapshot.SuccessfulRequests, snapshot.SuccessRate))
	sb.WriteString(fmt.Sprintf("  Failed: %d\n", snapshot.FailedRequests))
	sb.WriteString(fmt.Sprintf("  Avg Latency: %.2f ms\n", snapshot.AvgLatencyMs))
	sb.WriteString(fmt.Sprintf("  Requests/sec: %.2f\n", snapshot.RequestsPerSecond))
	sb.WriteString("\n")

	sb.WriteString("Cache Performance:\n")
	sb.WriteString(fmt.Sprintf("  Cache Hits: %d\n", snapshot.CacheHits))
	sb.WriteString(fmt.Sprintf("  Cache Misses: %d\n", snapshot.CacheMisses))
	sb.WriteString(fmt.Sprintf("  Hit Rate: %.1f%%\n", snapshot.CacheHitRate))
	sb.WriteString("\n")

	sb.WriteString("Reliability:\n")
	sb.WriteString(fmt.Sprintf("  Circuit Breaker Trips: %d\n", snapshot.CircuitBreakerTrips))
	sb.WriteString("\n")

	if len(snapshot.Routes) > 0 {
		sb.WriteString("Top Routes by Traffic:\n")

		// Sort routes by request count
		type routeStat struct {
			name    string
			metrics RouteSnapshot
		}
		routes := make([]routeStat, 0, len(snapshot.Routes))
		for name, metrics := range snapshot.Routes {
			routes = append(routes, routeStat{name, metrics})
		}
		sort.Slice(routes, func(i, j int) bool {
			return routes[i].metrics.Requests > routes[j].metrics.Requests
		})

		for i, rs := range routes {
			if i >= 10 {
				break // Show top 10
			}
			sb.WriteString(fmt.Sprintf("  %d. %s - %d requests (%.2f ms avg)\n",
				i+1, rs.name, rs.metrics.Requests, rs.metrics.AvgLatencyMs))
		}
		sb.WriteString("\n")
	}

	if len(snapshot.Services) > 0 {
		sb.WriteString("Backend Services:\n")

		// Sort services by name
		services := make([]string, 0, len(snapshot.Services))
		for service := range snapshot.Services {
			services = append(services, service)
		}
		sort.Strings(services)

		for _, service := range services {
			sm := snapshot.Services[service]
			sb.WriteString(fmt.Sprintf("  %s: %d requests, %d failures (%.2f ms avg)\n",
				service, sm.Requests, sm.Failures, sm.AvgLatencyMs))
		}
	}

	w.Write([]byte(sb.String()))
}
