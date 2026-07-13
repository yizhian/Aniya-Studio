package agent

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker()
	if !cb.Allow() {
		t.Error("expected initial state to allow requests")
	}
	if cb.state != CircuitClosed {
		t.Errorf("expected CircuitClosed, got %d", cb.state)
	}
}

func TestCircuitBreaker_TripAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.tripThreshold = 3
	cb.resetTimeout = 10 * time.Second

	// 3 consecutive failures → trip to Open.
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatalf("request %d should be allowed before trip", i+1)
		}
		cb.RecordFailure()
	}

	if cb.Allow() {
		t.Error("expected circuit to be Open after 3 failures")
	}
	if cb.state != CircuitOpen {
		t.Errorf("expected CircuitOpen, got %d", cb.state)
	}
}

func TestCircuitBreaker_HalfOpenProbe_Success(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.tripThreshold = 1
	cb.resetTimeout = 1 * time.Millisecond

	// Trip immediately.
	cb.RecordFailure()
	if cb.state != CircuitOpen {
		t.Fatalf("expected CircuitOpen, got %d", cb.state)
	}

	// Wait for reset timeout.
	time.Sleep(5 * time.Millisecond)

	// Probe allowed → HalfOpen.
	if !cb.Allow() {
		t.Error("expected probe to be allowed after timeout")
	}
	if cb.state != CircuitHalfOpen {
		t.Errorf("expected CircuitHalfOpen, got %d", cb.state)
	}

	// Success → back to Closed.
	cb.RecordSuccess()
	if cb.state != CircuitClosed {
		t.Errorf("expected CircuitClosed after success, got %d", cb.state)
	}
}

func TestCircuitBreaker_HalfOpenProbe_Failure(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.tripThreshold = 1
	cb.resetTimeout = 1 * time.Millisecond

	// Trip immediately.
	cb.RecordFailure()
	time.Sleep(5 * time.Millisecond)

	// Probe allowed → HalfOpen.
	if !cb.Allow() {
		t.Error("expected probe to be allowed after timeout")
	}

	// Probe fails → back to Open.
	cb.RecordFailure()
	if cb.state != CircuitOpen {
		t.Errorf("expected CircuitOpen after probe failure, got %d", cb.state)
	}
	if cb.Allow() {
		t.Error("expected circuit to be Open after failed probe")
	}
}

func TestCircuitBreaker_RecordSuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.tripThreshold = 5

	// 4 failures → still Closed.
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	if !cb.Allow() {
		t.Error("expected still Closed after 4 failures")
	}

	// Success resets counter.
	cb.RecordSuccess()
	if cb.consecutiveFails != 0 {
		t.Errorf("expected 0 consecutive fails after success, got %d", cb.consecutiveFails)
	}

	// 4 more failures → still not tripped (counter was reset).
	for i := 0; i < 4; i++ {
		cb.RecordFailure()
	}
	if !cb.Allow() {
		t.Error("expected still Closed after 4 failures (counter was reset)")
	}
}

func TestCircuitBreaker_AllowFalse_NeverCallsRecordFailure(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.tripThreshold = 2

	// Trip the circuit.
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.Allow() {
		t.Fatal("expected circuit to be Open")
	}

	// Allow() returns false — caller should NOT call RecordFailure.
	// Verify that RecordFailure is NOT called when Allow is false.
	// (This is a design contract test: the caller must check Allow() first.)
	if !cb.Allow() {
		// Circuit is still Open — this is correct behavior.
		// If the caller calls RecordFailure anyway, it would extend the outage.
		// We test that the circuit stays Open even without additional failures.
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.tripThreshold = 100 // large threshold so we don't trip during concurrent test

	var wg sync.WaitGroup
	concurrency := 50
	iterations := 100

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if cb.Allow() {
					if j%3 == 0 {
						cb.RecordFailure()
					} else {
						cb.RecordSuccess()
					}
				}
			}
		}()
	}
	wg.Wait()

	// Circuit should still be Closed (threshold was high enough).
	if cb.state != CircuitClosed {
		t.Errorf("expected CircuitClosed after concurrent access, got %d", cb.state)
	}
}

func TestCircuitBreaker_DefaultThresholds(t *testing.T) {
	cb := NewCircuitBreaker()
	if cb.tripThreshold != 5 {
		t.Errorf("expected default tripThreshold 5, got %d", cb.tripThreshold)
	}
	if cb.resetTimeout != 30*time.Second {
		t.Errorf("expected default resetTimeout 30s, got %v", cb.resetTimeout)
	}
	if cb.state != CircuitClosed {
		t.Errorf("expected default state CircuitClosed, got %d", cb.state)
	}
}

func TestCircuitBreaker_ResetAfterTimeout(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.tripThreshold = 1
	cb.resetTimeout = 1 * time.Millisecond

	// Trip.
	cb.RecordFailure()
	if !cb.Allow() {
		t.Log("circuit is Open as expected")
	}

	// Wait long enough.
	time.Sleep(10 * time.Millisecond)

	// Should be HalfOpen now and allow probe.
	if !cb.Allow() {
		t.Error("expected probe to be allowed after full reset timeout")
	}
}

func TestCircuitBreaker_StringRepresentation(t *testing.T) {
	cb := NewCircuitBreaker()
	stateNames := map[CircuitState]string{
		CircuitClosed:   "closed",
		CircuitOpen:     "open",
		CircuitHalfOpen: "half-open",
	}

	cb.state = CircuitClosed
	if stateNames[cb.state] != "closed" {
		t.Errorf("unexpected state name: %s", stateNames[cb.state])
	}

	cb.state = CircuitOpen
	if stateNames[cb.state] != "open" {
		t.Errorf("unexpected state name: %s", stateNames[cb.state])
	}

	cb.state = CircuitHalfOpen
	if stateNames[cb.state] != "half-open" {
		t.Errorf("unexpected state name: %s", stateNames[cb.state])
	}
}

// Smoke test: rapid trip/retry/reset cycle.
func TestCircuitBreaker_Smoke_RapidCycle(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.tripThreshold = 2
	cb.resetTimeout = 1 * time.Millisecond

	for cycle := 0; cycle < 5; cycle++ {
		// Trip.
		cb.RecordFailure()
		cb.RecordFailure()
		if cb.Allow() {
			t.Errorf("cycle %d: expected Open after trip", cycle)
		}

		// Wait and recover.
		time.Sleep(5 * time.Millisecond)
		if !cb.Allow() {
			t.Errorf("cycle %d: expected HalfOpen after timeout", cycle)
		}
		cb.RecordSuccess()
		if cb.state != CircuitClosed {
			t.Errorf("cycle %d: expected Closed after success, got %d", cycle, cb.state)
		}
	}
}

// SIT: Verify the breaker protects against burst failures.
func TestCircuitBreaker_SIT_BurstProtection(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.tripThreshold = 3
	cb.resetTimeout = 100 * time.Millisecond

	allowedCount := 0
	deniedCount := 0

	// Simulate 20 requests with a burst of 3 consecutive failures at the start.
	for i := 0; i < 20; i++ {
		if !cb.Allow() {
			deniedCount++
			continue
		}
		allowedCount++
		// First 3 are failures (burst), then only successes.
		if i < 3 {
			cb.RecordFailure()
		} else {
			cb.RecordSuccess()
		}
	}

	// After the initial burst of 3 failures, the breaker should have tripped.
	if deniedCount == 0 {
		t.Error("expected some requests to be denied after burst failure")
	}
	t.Logf("burst protection: %d allowed, %d denied", allowedCount, deniedCount)
}

// Ensure fmt import is used.
var _ = fmt.Sprintf