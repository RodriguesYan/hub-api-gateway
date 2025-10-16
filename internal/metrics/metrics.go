package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics collects gateway performance metrics
type Metrics struct {
	// Request counters
	totalRequests      atomic.Uint64
	successfulRequests atomic.Uint64
	failedRequests     atomic.Uint64

	// Request by route
	routeMetrics sync.Map // map[string]*RouteMetrics

	// Response time tracking
	totalLatency atomic.Uint64 // in milliseconds

	// Service-specific metrics
	serviceMetrics sync.Map // map[string]*ServiceMetrics

	// Circuit breaker metrics
	circuitBreakerTrips atomic.Uint64

	// Cache metrics
	cacheHits   atomic.Uint64
	cacheMisses atomic.Uint64

	startTime time.Time
}

// RouteMetrics tracks metrics for a specific route
type RouteMetrics struct {
	requests      atomic.Uint64
	successes     atomic.Uint64
	failures      atomic.Uint64
	totalLatency  atomic.Uint64 // in milliseconds
	lastRequestAt atomic.Value  // time.Time
}

// ServiceMetrics tracks metrics for a specific backend service
type ServiceMetrics struct {
	requests     atomic.Uint64
	successes    atomic.Uint64
	failures     atomic.Uint64
	totalLatency atomic.Uint64 // in milliseconds
}

// NewMetrics creates a new metrics collector
func NewMetrics() *Metrics {
	return &Metrics{
		startTime: time.Now(),
	}
}

// RecordRequest records a request and its outcome
func (m *Metrics) RecordRequest(routeName, serviceName string, latency time.Duration, success bool) {
	// Update total counters
	m.totalRequests.Add(1)
	if success {
		m.successfulRequests.Add(1)
	} else {
		m.failedRequests.Add(1)
	}
	m.totalLatency.Add(uint64(latency.Milliseconds()))

	// Update route metrics
	rm := m.getOrCreateRouteMetrics(routeName)
	rm.requests.Add(1)
	if success {
		rm.successes.Add(1)
	} else {
		rm.failures.Add(1)
	}
	rm.totalLatency.Add(uint64(latency.Milliseconds()))
	rm.lastRequestAt.Store(time.Now())

	// Update service metrics
	if serviceName != "" {
		sm := m.getOrCreateServiceMetrics(serviceName)
		sm.requests.Add(1)
		if success {
			sm.successes.Add(1)
		} else {
			sm.failures.Add(1)
		}
		sm.totalLatency.Add(uint64(latency.Milliseconds()))
	}
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit() {
	m.cacheHits.Add(1)
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss() {
	m.cacheMisses.Add(1)
}

// RecordCircuitBreakerTrip records a circuit breaker trip
func (m *Metrics) RecordCircuitBreakerTrip() {
	m.circuitBreakerTrips.Add(1)
}

// getOrCreateRouteMetrics gets or creates route metrics
func (m *Metrics) getOrCreateRouteMetrics(routeName string) *RouteMetrics {
	if val, ok := m.routeMetrics.Load(routeName); ok {
		return val.(*RouteMetrics)
	}

	rm := &RouteMetrics{}
	m.routeMetrics.Store(routeName, rm)
	return rm
}

// getOrCreateServiceMetrics gets or creates service metrics
func (m *Metrics) getOrCreateServiceMetrics(serviceName string) *ServiceMetrics {
	if val, ok := m.serviceMetrics.Load(serviceName); ok {
		return val.(*ServiceMetrics)
	}

	sm := &ServiceMetrics{}
	m.serviceMetrics.Store(serviceName, sm)
	return sm
}

// GetSnapshot returns a snapshot of current metrics
func (m *Metrics) GetSnapshot() MetricsSnapshot {
	totalReqs := m.totalRequests.Load()
	successReqs := m.successfulRequests.Load()
	failedReqs := m.failedRequests.Load()
	totalLat := m.totalLatency.Load()

	var avgLatency float64
	if totalReqs > 0 {
		avgLatency = float64(totalLat) / float64(totalReqs)
	}

	var successRate float64
	if totalReqs > 0 {
		successRate = float64(successReqs) / float64(totalReqs) * 100
	}

	// Collect route metrics
	routes := make(map[string]RouteSnapshot)
	m.routeMetrics.Range(func(key, value interface{}) bool {
		routeName := key.(string)
		rm := value.(*RouteMetrics)

		reqs := rm.requests.Load()
		var avgLat float64
		if reqs > 0 {
			avgLat = float64(rm.totalLatency.Load()) / float64(reqs)
		}

		var lastReq time.Time
		if val := rm.lastRequestAt.Load(); val != nil {
			lastReq = val.(time.Time)
		}

		routes[routeName] = RouteSnapshot{
			Requests:      reqs,
			Successes:     rm.successes.Load(),
			Failures:      rm.failures.Load(),
			AvgLatencyMs:  avgLat,
			LastRequestAt: lastReq,
		}
		return true
	})

	// Collect service metrics
	services := make(map[string]ServiceSnapshot)
	m.serviceMetrics.Range(func(key, value interface{}) bool {
		serviceName := key.(string)
		sm := value.(*ServiceMetrics)

		reqs := sm.requests.Load()
		var avgLat float64
		if reqs > 0 {
			avgLat = float64(sm.totalLatency.Load()) / float64(reqs)
		}

		services[serviceName] = ServiceSnapshot{
			Requests:     reqs,
			Successes:    sm.successes.Load(),
			Failures:     sm.failures.Load(),
			AvgLatencyMs: avgLat,
		}
		return true
	})

	// Calculate requests per second
	uptime := time.Since(m.startTime).Seconds()
	var reqsPerSec float64
	if uptime > 0 {
		reqsPerSec = float64(totalReqs) / uptime
	}

	// Calculate cache hit rate
	totalCacheReqs := m.cacheHits.Load() + m.cacheMisses.Load()
	var cacheHitRate float64
	if totalCacheReqs > 0 {
		cacheHitRate = float64(m.cacheHits.Load()) / float64(totalCacheReqs) * 100
	}

	return MetricsSnapshot{
		TotalRequests:       totalReqs,
		SuccessfulRequests:  successReqs,
		FailedRequests:      failedReqs,
		SuccessRate:         successRate,
		AvgLatencyMs:        avgLatency,
		RequestsPerSecond:   reqsPerSec,
		CacheHits:           m.cacheHits.Load(),
		CacheMisses:         m.cacheMisses.Load(),
		CacheHitRate:        cacheHitRate,
		CircuitBreakerTrips: m.circuitBreakerTrips.Load(),
		UptimeSeconds:       uptime,
		Routes:              routes,
		Services:            services,
	}
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	TotalRequests       uint64
	SuccessfulRequests  uint64
	FailedRequests      uint64
	SuccessRate         float64
	AvgLatencyMs        float64
	RequestsPerSecond   float64
	CacheHits           uint64
	CacheMisses         uint64
	CacheHitRate        float64
	CircuitBreakerTrips uint64
	UptimeSeconds       float64
	Routes              map[string]RouteSnapshot
	Services            map[string]ServiceSnapshot
}

// RouteSnapshot represents metrics for a specific route
type RouteSnapshot struct {
	Requests      uint64
	Successes     uint64
	Failures      uint64
	AvgLatencyMs  float64
	LastRequestAt time.Time
}

// ServiceSnapshot represents metrics for a specific service
type ServiceSnapshot struct {
	Requests     uint64
	Successes    uint64
	Failures     uint64
	AvgLatencyMs float64
}

// Reset resets all metrics
func (m *Metrics) Reset() {
	m.totalRequests.Store(0)
	m.successfulRequests.Store(0)
	m.failedRequests.Store(0)
	m.totalLatency.Store(0)
	m.cacheHits.Store(0)
	m.cacheMisses.Store(0)
	m.circuitBreakerTrips.Store(0)
	m.routeMetrics = sync.Map{}
	m.serviceMetrics = sync.Map{}
	m.startTime = time.Now()
}
