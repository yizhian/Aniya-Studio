package agent

import (
	"sync"
	"time"
)

// CircuitState represents the current state of the circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker implements a per-conversation circuit breaker that prevents
// infinite retry loops during provider outages. Thread-safe via sync.Mutex.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            CircuitState
	consecutiveFails int
	lastFailTime     time.Time
	tripThreshold    int
	resetTimeout     time.Duration
}

// NewCircuitBreaker creates a breaker with default thresholds (5 failures, 30s reset).
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		state:         CircuitClosed,
		tripThreshold: 5,
		resetTimeout:  30 * time.Second,
	}
}

// Allow reports whether a request should be attempted.
// If the circuit is open but the reset timeout has elapsed, transitions to
// half-open and allows one probe.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailTime) >= cb.resetTimeout {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	case CircuitHalfOpen:
		return true // allow the probe
	default:
		return true
	}
}

// RecordSuccess resets the breaker to closed on success.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitClosed
	cb.consecutiveFails = 0
}

// RecordFailure increments the failure counter and may trip the breaker.
// Only HTTP-level failures call this; resource-class finish_reason events do NOT.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFails++
	cb.lastFailTime = time.Now()

	switch cb.state {
	case CircuitClosed:
		if cb.consecutiveFails >= cb.tripThreshold {
			cb.state = CircuitOpen
		}
	case CircuitHalfOpen:
		// Probe failed — reopen.
		cb.state = CircuitOpen
	}
}
// State returns the current circuit state (thread-safe).
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// ConsecutiveFails returns the current failure count (thread-safe).
func (cb *CircuitBreaker) ConsecutiveFails() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.consecutiveFails
}
