package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"agentgo/internal/observability"
)

// sseHeartbeatInterval controls how often SSE keepalive comments are sent.
var sseHeartbeatInterval = 15 * time.Second

// sseEvent is the JSON envelope sent to the frontend over SSE.
type sseEvent struct {
	Type  string         `json:"type"`
	Time  time.Time      `json:"time"`
	Round int            `json:"round"`
	Data  map[string]any `json:"data"`
}

// sseSession holds the resources for a single SSE connection.
type sseSession struct {
	w         http.ResponseWriter
	flusher   http.Flusher
	sseCh     chan string
	writerWG  *sync.WaitGroup
	emitter   *observability.Emitter
	logObs    *observability.LogFileObserver
	eventCh   chan observability.AgentEvent
	forwardWG *sync.WaitGroup
}

// setupSSE initializes an SSE connection and returns the session.
func setupSSE(w http.ResponseWriter, r *http.Request, sessID string, consoleObs *observability.ConsoleObserver) (*sseSession, error) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Session-Id", sessID)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported")
	}

	emitter := observability.NewEmitter()
	consoleObs.Subscribe(emitter)

	// Subscribe to global log file (if configured via AGENTGO_LOG_FILE)
	observability.SubscribeGlobalLog(emitter)

	// Emit SSE connect event
	emitter.Emit(observability.AgentEvent{
		Type: observability.EventSSEConnect,
		Data: map[string]any{"session_id": sessID},
	})

	logFileObs, _ := observability.NewLogFileObserver(".agentgo/logs", sessID)
	if logFileObs != nil {
		logFileObs.Subscribe(emitter)
	}

	sseCh := make(chan string, 64)
	var writerWG sync.WaitGroup
	writerWG.Add(1)
	go func() {
		defer writerWG.Done()
		heartbeat := time.NewTicker(sseHeartbeatInterval)
		defer heartbeat.Stop()
		for {
			select {
			case msg, ok := <-sseCh:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			case <-heartbeat.C:
				fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}()

	eventCh := make(chan observability.AgentEvent, 128)
	emitter.Subscribe(eventCh)
	var forwardWG sync.WaitGroup
	forwardWG.Add(1)
	go func() {
		defer forwardWG.Done()
		for ev := range eventCh {
			data, _ := json.Marshal(sseEvent{
				Type:  ev.Type,
				Time:  ev.Time,
				Round: ev.Round,
				Data:  ev.Data,
			})
			select {
			case sseCh <- string(data):
			case <-r.Context().Done():
				return
			}
		}
	}()

	return &sseSession{
		w:         w,
		flusher:   flusher,
		sseCh:     sseCh,
		writerWG:  &writerWG,
		emitter:   emitter,
		logObs:    logFileObs,
		eventCh:   eventCh,
		forwardWG: &forwardWG,
	}, nil
}

// Emitter returns the session's event emitter.
func (s *sseSession) Emitter() *observability.Emitter {
	return s.emitter
}

// Close tears down the SSE session, waiting for goroutines to finish.
func (s *sseSession) Close() {
	// Emit disconnect before cleanup
	s.emitter.Emit(observability.AgentEvent{
		Type: observability.EventSSEDisconnect,
	})
	close(s.eventCh)
	s.forwardWG.Wait()
	close(s.sseCh)
	s.writerWG.Wait()
	if s.logObs != nil {
		s.logObs.Close()
	}
}

// EmitError sends an error event to the SSE client.
func (s *sseSession) EmitError(reason string) {
	emitSSEError(s.w, s.flusher, reason)
}

// SendEvent writes an SSE event directly to the client. Safe to call after Close()
// (writes directly, bypassing the closed channel).
func (s *sseSession) SendEvent(ev sseEvent) {
	data, _ := json.Marshal(ev)
	fmt.Fprintf(s.w, "data: %s\n\n", data)
	s.flusher.Flush()
}

func emitSSEError(w http.ResponseWriter, flusher http.Flusher, reason string) {
	data, _ := json.Marshal(sseEvent{
		Type: "error",
		Time: time.Now(),
		Data: map[string]any{"message": reason},
	})
	fmt.Fprintf(w, "data: %s\n\n", data)
	if flusher != nil {
		flusher.Flush()
	}
}
