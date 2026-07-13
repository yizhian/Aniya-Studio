package provider

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestSSEReader_ContextCancellation(t *testing.T) {
	// Simulate a body that blocks forever (never returns data).
	pr, pw := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())

	// Track when the read goroutine exits.
	done := make(chan struct{})
	go func() {
		defer close(done)
		reader := newSSEReader(ctx, pr)
		defer reader.close()
		// readLine blocks on scanner.Scan() until body closes.
		_, _, eof := reader.readLine()
		if !eof {
			t.Error("expected eof after context cancellation")
		}
	}()

	// Cancel after a short delay.
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Goroutine should exit within 200ms of cancellation.
	select {
	case <-done:
		// Success.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("goroutine did not exit after context cancellation")
	}

	pw.Close() // clean up
}

func TestSSEReader_NormalEOF(t *testing.T) {
	body := io.NopCloser(strings.NewReader("data: hello\n\n"))
	ctx := context.Background()
	reader := newSSEReader(ctx, body)
	defer reader.close()

	data, done, eof := reader.readLine()
	if eof {
		t.Fatal("unexpected EOF")
	}
	if done {
		t.Fatal("unexpected done")
	}
	if data != "hello" {
		t.Errorf("expected 'hello', got %q", data)
	}

	// Second read should return EOF.
	_, _, eof = reader.readLine()
	if !eof {
		t.Fatal("expected EOF after last line")
	}
}

func TestSSEReader_GoroutineExitsOnClose(t *testing.T) {
	body := io.NopCloser(strings.NewReader("data: hello\n\n"))
	ctx := context.Background() // never cancels

	done := make(chan struct{})
	go func() {
		defer close(done)
		reader := newSSEReader(ctx, body)
		defer reader.close()
		reader.readLine()
		reader.readLine() // EOF
	}()

	select {
	case <-done:
		// goroutine exited — done channel signalled correctly
	case <-time.After(time.Second):
		t.Fatal("goroutine did not exit after close() — still blocked on ctx.Done()")
	}
}

func TestSSEReader_DoneDetection(t *testing.T) {
	body := io.NopCloser(strings.NewReader("data: [DONE]\n\n"))
	ctx := context.Background()
	reader := newSSEReader(ctx, body)
	defer reader.close()

	_, done, eof := reader.readLine()
	if eof {
		t.Fatal("unexpected EOF")
	}
	if !done {
		t.Fatal("expected done for [DONE] token")
	}
}
