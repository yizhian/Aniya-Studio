package retry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 3, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestDo_RetryOnRateLimit(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 3, func() error {
		calls++
		if calls < 3 {
			return fmt.Errorf("429 rate limit exceeded")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDo_RetryOnServerError(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 2, func() error {
		calls++
		if calls < 2 {
			return fmt.Errorf("503 service unavailable")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestDo_RetryOn500(t *testing.T) {
	calls := 0
	Do(context.Background(), 1, func() error {
		calls++
		if calls == 1 {
			return fmt.Errorf("500 internal server error")
		}
		return nil
	})
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestDo_Exhausted(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 2, func() error {
		calls++
		return fmt.Errorf("429 rate limit exceeded")
	})
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	if calls != 3 { // maxRetries=2 means 3 attempts
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestDo_NonRetryable(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 3, func() error {
		calls++
		return errors.New("invalid api key")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry), got %d", calls)
	}
}

func TestDo_NonRetryableAuth(t *testing.T) {
	calls := 0
	Do(context.Background(), 3, func() error {
		calls++
		return fmt.Errorf("401 unauthorized")
	})
	if calls != 1 {
		t.Errorf("expected 1 call for 401, got %d", calls)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	errCh := make(chan error, 1)
	go func() {
		errCh <- Do(ctx, 5, func() error {
			calls++
			return fmt.Errorf("503 service unavailable")
		})
	}()

	// Wait for first call and backoff to start.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from cancelled context")
		}
		if !errors.Is(err, context.Canceled) && err.Error() == "" {
			t.Fatal("expected non-empty error")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for cancellation")
	}
}

func TestDo_Backoff(t *testing.T) {
	// Verify retry count and eventual success. We don't assert on exact timing
	// because jitter makes the backoff non-deterministic.
	calls := 0
	err := Do(context.Background(), 2, func() error {
		calls++
		if calls < 3 {
			return fmt.Errorf("503 service unavailable")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryableHTTPError_Error(t *testing.T) {
	err := &RetryableHTTPError{Code: 429}
	if err.Error() != "retryable HTTP status 429" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	err2 := &RetryableHTTPError{Code: 503}
	if err2.Error() != "retryable HTTP status 503" {
		t.Errorf("unexpected error message: %s", err2.Error())
	}
}

func TestIsRetryableHTTPStatus(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{500, true}, {502, true}, {503, true}, {504, true}, {599, true},
		{429, true}, {408, true},
		{400, false}, {401, false}, {403, false}, {404, false},
		{200, false}, {301, false}, {499, false}, {600, false},
	}
	for _, tc := range tests {
		if IsRetryableHTTPStatus(tc.code) != tc.expected {
			t.Errorf("IsRetryableHTTPStatus(%d) = %v, want %v", tc.code, !tc.expected, tc.expected)
		}
	}
}

func TestDo_RetryOnHTTPErrorType(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 2, func() error {
		calls++
		if calls < 2 {
			return &RetryableHTTPError{Code: 502}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls for RetryableHTTPError, got %d", calls)
	}
}

func TestDo_NonRetryableHTTPError(t *testing.T) {
	calls := 0
	err := Do(context.Background(), 3, func() error {
		calls++
		return &RetryableHTTPError{Code: 404}
	})
	if err == nil {
		t.Fatal("expected error for non-retryable HTTP status")
	}
	if calls != 1 {
		t.Errorf("expected 1 call for 404, got %d", calls)
	}
}

func TestAsNetErr_Temporary(t *testing.T) {
	// Create a temporary net.Error.
	err := &testNetError{temporary: true}
	if !isRetryable(err) {
		t.Error("temporary net.Error should be retryable")
	}
}

func TestAsNetErr_Timeout(t *testing.T) {
	err := &testNetError{timeout: true}
	if !isRetryable(err) {
		t.Error("timeout net.Error should be retryable")
	}
}

func TestAsNetErr_NotTemporary(t *testing.T) {
	err := &testNetError{temporary: false, timeout: false}
	if isRetryable(err) {
		t.Error("non-temporary net.Error should NOT be retryable")
	}
}

func TestIsRetryable_EOF(t *testing.T) {
	if !isRetryable(io.EOF) {
		t.Error("io.EOF should be retryable")
	}
}

type testNetError struct {
	temporary bool
	timeout   bool
}

func (e *testNetError) Error() string   { return "test network error" }
func (e *testNetError) Temporary() bool { return e.temporary }
func (e *testNetError) Timeout() bool   { return e.timeout }

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err     error
		retry   bool
	}{
		{nil, false},
		{fmt.Errorf("429 rate limit"), true},
		{fmt.Errorf("503 service unavailable"), true},
		{fmt.Errorf("500 internal server error"), true},
		{fmt.Errorf("rate limit exceeded"), true},
		{fmt.Errorf("rate_limit error"), true},
		{fmt.Errorf("too many requests"), true},
		{fmt.Errorf("server error"), true},
		{fmt.Errorf("401 unauthorized"), false},
		{fmt.Errorf("invalid request"), false},
		{errors.New("something broke"), false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			got := isRetryable(tt.err)
			if got != tt.retry {
				t.Errorf("isRetryable(%v) = %v, want %v", tt.err, got, tt.retry)
			}
		})
	}
}
