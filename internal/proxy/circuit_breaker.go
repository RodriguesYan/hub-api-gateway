package proxy

import (
	"errors"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation
	StateOpen                         // Failing, reject requests
	StateHalfOpen                     // Testing if service recovered
)

// String returns the string representation of the circuit state
func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name             string
	maxFailures      uint32        // Max failures before opening
	resetTimeout     time.Duration // Time to wait before attempting recovery
	halfOpenRequests uint32        // Number of test requests in half-open state

	mu              sync.RWMutex
	state           CircuitState
	failures        uint32
	lastFailureTime time.Time
	successCount    uint32
}

// CircuitBreakerConfig holds configuration for circuit breaker
type CircuitBreakerConfig struct {
	MaxFailures      uint32        // Default: 5
	ResetTimeout     time.Duration // Default: 30s
	HalfOpenRequests uint32        // Default: 3
}

var (
	// ErrCircuitOpen is returned when circuit breaker is open
	ErrCircuitOpen = errors.New("circuit breaker is open")
	// ErrTooManyRequests is returned when too many requests in half-open state
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	if config.MaxFailures == 0 {
		config.MaxFailures = 5
	}
	if config.ResetTimeout == 0 {
		config.ResetTimeout = 30 * time.Second
	}
	if config.HalfOpenRequests == 0 {
		config.HalfOpenRequests = 3
	}

	return &CircuitBreaker{
		name:             name,
		maxFailures:      config.MaxFailures,
		resetTimeout:     config.ResetTimeout,
		halfOpenRequests: config.HalfOpenRequests,
		state:            StateClosed,
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	err := fn()
	cb.afterRequest(err)
	return err
}

// beforeRequest checks if request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Normal operation - allow request
		return nil

	case StateOpen:
		// Check if enough time has passed to try recovery
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
			return nil
		}
		return ErrCircuitOpen

	case StateHalfOpen:
		// Allow limited requests to test service
		if cb.successCount >= cb.halfOpenRequests {
			return ErrTooManyRequests
		}
		return nil

	default:
		return nil
	}
}

// afterRequest records the result of a request
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onFailure handles a failed request
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.maxFailures {
			cb.state = StateOpen
		}

	case StateHalfOpen:
		// Failed during testing - back to open
		cb.state = StateOpen
		cb.successCount = 0
	}
}

// onSuccess handles a successful request
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failures = 0

	case StateHalfOpen:
		// Count successes to determine if we can close
		cb.successCount++
		if cb.successCount >= cb.halfOpenRequests {
			cb.state = StateClosed
			cb.failures = 0
		}
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailures returns the current failure count
func (cb *CircuitBreaker) GetFailures() uint32 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.failures = 0
	cb.successCount = 0
}

// GetStats returns circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":             cb.name,
		"state":            cb.state.String(),
		"failures":         cb.failures,
		"max_failures":     cb.maxFailures,
		"last_failure":     cb.lastFailureTime,
		"success_count":    cb.successCount,
		"reset_timeout_ms": cb.resetTimeout.Milliseconds(),
	}
}
