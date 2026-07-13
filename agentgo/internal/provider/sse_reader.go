package provider

import (
	"bufio"
	"context"
	"io"
	"strings"
)

// sseReader encapsulates common SSE stream reading logic shared by
// OpenAI and Anthropic providers: scanner initialization, line parsing,
// [DONE] detection, and error handling.
type sseReader struct {
	scanner *bufio.Scanner
	body    io.ReadCloser
	done    chan struct{}
}

func newSSEReader(ctx context.Context, body io.ReadCloser) *sseReader {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 524288), 524288)

	sr := &sseReader{scanner: scanner, body: body, done: make(chan struct{})}

	// When the context is cancelled, close the body so the blocking
	// scanner.Scan() call returns EOF instead of leaking the goroutine.
	// When the stream completes normally (close is called), the done
	// channel is closed so this goroutine exits cleanly.
	go func() {
		select {
		case <-ctx.Done():
			body.Close()
		case <-sr.done:
		}
	}()

	return sr
}

// readLine reads the next SSE data line. It skips empty lines, event: lines,
// and lines without the "data:" prefix. Returns (data, isDone, isEOF).
func (r *sseReader) readLine() (data string, done bool, eof bool) {
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return "", true, false
		}
		return data, false, false
	}
	return "", false, true
}

// scannerErr returns the scanner error, if any.
func (r *sseReader) scannerErr() error {
	return r.scanner.Err()
}

// close signals the context goroutine to exit and closes the underlying body.
func (r *sseReader) close() {
	close(r.done)
	r.body.Close()
}
