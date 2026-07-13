package observability

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

var (
	globalLogFile   *os.File
	globalLogMu     sync.Mutex
	globalLogSeq    int64
)

// SetGlobalLogFile opens a single log file that receives ALL events — both those
// emitted through active SSE sessions AND those that fall back to stdout.
//
// Call this once at startup. Pass an empty string to disable global file logging.
func SetGlobalLogFile(path string) error {
	globalLogMu.Lock()
	defer globalLogMu.Unlock()

	// Close previous file if any
	if globalLogFile != nil {
		globalLogFile.Close()
		globalLogFile = nil
	}

	if path == "" {
		return nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("global log file: %w", err)
	}
	globalLogFile = f
	log.Printf("global log file: %s", path)
	return nil
}

// SubscribeGlobalLog subscribes this emitter's events to the global log file.
// Call this for every emitter you create (e.g., per SSE session).
func SubscribeGlobalLog(e *Emitter) {
	globalLogMu.Lock()
	defer globalLogMu.Unlock()
	if globalLogFile == nil || e == nil {
		return
	}
	ch := make(chan AgentEvent, 256)
	e.Subscribe(ch)
	go func() {
		enc := json.NewEncoder(globalLogFile)
		for ev := range ch {
			entry := logEntry{
				Seq:   globalLogSeq,
				Time:  ev.Time,
				Type:  ev.Type,
				Round: ev.Round,
				Data:  ev.Data,
			}
			globalLogSeq++
			globalLogMu.Lock()
			enc.Encode(entry)
			globalLogFile.Sync()
			globalLogMu.Unlock()
		}
	}()
}

// EmitOrLog sends an event through the Emitter when available, falling back to
// structured JSON written to stdout via log.Print.
//
// When a global log file is configured via SetGlobalLogFile, fallback events
// are also written there.
func EmitOrLog(emitter *Emitter, ev AgentEvent) {
	if emitter != nil {
		emitter.Emit(ev)
		return
	}

	// Fallback: structured JSON to stdout via log.Print
	entry := map[string]any{
		"ts":   time.Now().Format(time.RFC3339Nano),
		"type": ev.Type,
	}
	if ev.Round != 0 {
		entry["round"] = ev.Round
	}
	for k, v := range ev.Data {
		entry[k] = v
	}
	b, err := json.Marshal(entry)
	if err != nil {
		log.Printf("emitOrLog marshal error: %v", err)
		return
	}
	log.Print(string(b))

	// Also write to global log file
	globalLogMu.Lock()
	if globalLogFile != nil {
		globalLogFile.Write(append(b, '\n'))
		globalLogFile.Sync()
	}
	globalLogMu.Unlock()
}
