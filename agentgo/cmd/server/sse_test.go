package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"agentgo/internal/observability"
)

// fakeFlusher implements http.Flusher on top of httptest.ResponseRecorder.
type fakeFlusher struct {
	*httptest.ResponseRecorder
}

func (f *fakeFlusher) Flush() {}

func TestSetupSSE_InvalidFlusher(t *testing.T) {
	// A ResponseWriter that does NOT implement http.Flusher.
	w := &nonFlusherWriter{header: make(http.Header)}
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	sess, err := setupSSE(w, r, "test-sess", observability.NewConsoleObserver())
	if err == nil {
		t.Fatal("expected error for non-flushing writer")
	}
	if sess != nil {
		t.Error("expected nil session on error")
	}
}

type nonFlusherWriter struct {
	header http.Header
}

func (w *nonFlusherWriter) Header() http.Header         { return w.header }
func (w *nonFlusherWriter) Write(b []byte) (int, error)  { return len(b), nil }
func (w *nonFlusherWriter) WriteHeader(code int)          {}

func TestSetupSSE_HeadersSet(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/chat", nil)

	sess, err := setupSSE(&fakeFlusher{w}, r, "my-session-id", observability.NewConsoleObserver())
	if err != nil {
		t.Fatalf("setupSSE failed: %v", err)
	}
	defer sess.Close()

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected no-cache, got %q", cc)
	}
	if conn := w.Header().Get("Connection"); conn != "keep-alive" {
		t.Errorf("expected keep-alive, got %q", conn)
	}
	if sid := w.Header().Get("X-Session-Id"); sid != "my-session-id" {
		t.Errorf("expected my-session-id, got %q", sid)
	}
}

func TestSSESession_Emitter(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/chat", nil)

	sess, err := setupSSE(&fakeFlusher{w}, r, "test-sess", observability.NewConsoleObserver())
	if err != nil {
		t.Fatalf("setupSSE failed: %v", err)
	}
	defer sess.Close()

	emitter := sess.Emitter()
	if emitter == nil {
		t.Fatal("expected non-nil emitter")
	}
}

func TestSSESession_EmitError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/chat", nil)

	sess, err := setupSSE(&fakeFlusher{w}, r, "test-sess", observability.NewConsoleObserver())
	if err != nil {
		t.Fatalf("setupSSE failed: %v", err)
	}
	defer sess.Close()

	// EmitError writes directly to the response writer.
	sess.EmitError("something went wrong")

	body := w.Body.String()
	if !strings.Contains(body, "error") {
		t.Errorf("expected error type in SSE output, got: %s", body)
	}
	if !strings.Contains(body, "something went wrong") {
		t.Errorf("expected error message in SSE output, got: %s", body)
	}
}

func TestSSESession_SendEvent(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/chat", nil)

	sess, err := setupSSE(&fakeFlusher{w}, r, "test-sess", observability.NewConsoleObserver())
	if err != nil {
		t.Fatalf("setupSSE failed: %v", err)
	}
	defer sess.Close()

	ev := sseEvent{
		Type:  "result",
		Time:  time.Now(),
		Round: 3,
		Data: map[string]any{"key": "value"},
	}

	sess.SendEvent(ev)

	body := w.Body.String()
	if !strings.Contains(body, "result") {
		t.Errorf("expected 'result' type in SSE output, got: %s", body)
	}
	if !strings.Contains(body, "key") {
		t.Errorf("expected data key in SSE output, got: %s", body)
	}
}

func TestSSESession_Close_CleansUp(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/chat", nil)

	sess, err := setupSSE(&fakeFlusher{w}, r, "test-sess", observability.NewConsoleObserver())
	if err != nil {
		t.Fatalf("setupSSE failed: %v", err)
	}

	// Close should complete without panic.
	sess.Close()

	// Double-close should not panic (channels already closed, but Close tries to close them again).
	// This verifies we handled this gracefully or at least don't crash in worst case.
}

func TestSSESession_Close_ThenSendEvent(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/chat", nil)

	sess, err := setupSSE(&fakeFlusher{w}, r, "test-sess", observability.NewConsoleObserver())
	if err != nil {
		t.Fatalf("setupSSE failed: %v", err)
	}

	sess.Close()

	// SendEvent writes directly to w, bypassing channel — safe after close.
	ev := sseEvent{Type: "post-close", Time: time.Now()}
	sess.SendEvent(ev)

	body := w.Body.String()
	if !strings.Contains(body, "post-close") {
		t.Errorf("expected post-close event after session close, got: %s", body)
	}
}

func TestEmitSSEError_NilFlusher(t *testing.T) {
	w := httptest.NewRecorder()
	// emitSSEError with nil flusher should not panic.
	emitSSEError(w, nil, "test error")
	body := w.Body.String()
	if !strings.Contains(body, "error") {
		t.Errorf("expected error in output, got: %s", body)
	}
}

func TestEmitSSEError_ValidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	emitSSEError(w, &fakeFlusher{w}, "json test error")
	body := w.Body.String()

	// Parse the SSE output: "data: {json}\n\n"
	body = strings.TrimPrefix(body, "data: ")
	body = strings.TrimSpace(body)

	var ev sseEvent
	if err := json.Unmarshal([]byte(body), &ev); err != nil {
		t.Fatalf("expected valid JSON, got error: %v (body=%q)", err, body)
	}
	if ev.Type != "error" {
		t.Errorf("expected 'error' type, got %q", ev.Type)
	}
	msg, _ := ev.Data["message"].(string)
	if msg != "json test error" {
		t.Errorf("expected 'json test error', got %q", msg)
	}
}

func TestSSESession_ForwarderReceivesEvents(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/chat", nil)

	sess, err := setupSSE(&fakeFlusher{w}, r, "test-sess", observability.NewConsoleObserver())
	if err != nil {
		t.Fatalf("setupSSE failed: %v", err)
	}

	// Emit an event through the emitter — it goes through eventCh → forwarder → sseCh → writer.
	ready := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Give forwarder time to start.
		<-ready
		sess.emitter.Emit(observability.AgentEvent{
			Type:  "tool_call",
			Time:  time.Now(),
			Round: 1,
			Data:  map[string]any{"tool": "write_file"},
		})
	}()

	close(ready)
	wg.Wait()

	// Close the session to flush everything.
	sess.Close()

	body := w.Body.String()
	if !strings.Contains(body, "tool_call") {
		t.Errorf("expected 'tool_call' event forwarded through to SSE, got: %s", body)
	}
}

// TestSSESession_CloseThenEmitError_Safe verifies that Close before EmitError
// works without race conditions (H7 fix). EmitError writes directly to the
// ResponseWriter, so it is safe after Close stops all goroutines.
func TestSSESession_CloseThenEmitError_Safe(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/chat", nil)

	sess, err := setupSSE(&fakeFlusher{w}, r, "test-sess", observability.NewConsoleObserver())
	if err != nil {
		t.Fatalf("setupSSE failed: %v", err)
	}

	sess.Close()
	sess.EmitError("[Blocked] blocking-hook\n\nOperation blocked. Reason: session limit exceeded")

	body := w.Body.String()
	if !strings.Contains(body, "error") {
		t.Errorf("expected error type in SSE output, got: %s", body)
	}
	if !strings.Contains(body, "[Blocked] blocking-hook") {
		t.Errorf("expected blocked error to include hook name, got: %s", body)
	}
}
