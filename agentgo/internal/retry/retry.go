// Package retry provides exponential-backoff retry for transient API errors.
package retry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"strings"
	"time"
)

// Do calls fn up to maxRetries+1 times with exponential backoff.
// fn should return nil on success. If fn returns a retryable error, Do waits
// and retries. Non-retryable errors are returned immediately.
// Context cancellation is respected between retries.
func Do(ctx context.Context, maxRetries int, fn func() error) error {
	const maxBackoff = 30 * time.Second
	backoff := 1 * time.Second
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Apply jitter: backoff/2 + random(0, backoff/2).
			jitter := backoff/2 + time.Duration(rand.Int64N(int64(backoff/2+1)))
			select {
			case <-time.After(jitter):
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled: %w (last error: %v)", ctx.Err(), lastErr)
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if !isRetryable(lastErr) {
			return lastErr
		}
	}

	return fmt.Errorf("retries exhausted after %d attempts: %w", maxRetries+1, lastErr)
}

// isRetryable checks if an error qualifies for automatic retry.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Structured HTTP status errors take priority over substring matching.
	var httpErr *RetryableHTTPError
	if errors.As(err, &httpErr) {
		return IsRetryableHTTPStatus(httpErr.Code)
	}

	msg := err.Error()

	// HTTP status codes from provider wrappers.
	if strings.Contains(msg, "429") || strings.Contains(msg, "503") || strings.Contains(msg, "500") {
		return true
	}
	if strings.Contains(msg, "rate limit") || strings.Contains(msg, "rate_limit") {
		return true
	}
	if strings.Contains(msg, "service unavailable") || strings.Contains(msg, "server error") {
		return true
	}
	if strings.Contains(msg, "too many requests") {
		return true
	}

	// EOF on connection dial is a transient network issue.
	if errors.Is(err, io.EOF) {
		return true
	}

	// Network errors: ECONNRESET, temporary DNS failures, etc.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}

	return false
}

// RetryableHTTPError wraps an HTTP status code that should trigger a retry.
type RetryableHTTPError struct {
	Code int
}

func (e *RetryableHTTPError) Error() string {
	return fmt.Sprintf("retryable HTTP status %d", e.Code)
}

// IsRetryableHTTPStatus returns true for status codes that warrant a retry.
func IsRetryableHTTPStatus(code int) bool {
	if code >= 500 && code <= 599 {
		return true
	}
	if code == 429 || code == 408 {
		return true
	}
	return false
}
