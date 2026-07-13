package observability

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// LogFileObserver writes agent events as JSONL to a log file.
type LogFileObserver struct {
	ch   chan AgentEvent
	file *os.File
	seq  atomic.Int64
	done chan struct{}
}

// NewLogFileObserver creates the log directory if needed, opens the log file,
// and starts a background goroutine that writes JSONL entries.
func NewLogFileObserver(dir, sessionID string) (*LogFileObserver, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("log dir: %w", err)
	}

	name := fmt.Sprintf("sess_%s_%s.jsonl", sessionID, time.Now().Format("20060102"))
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return nil, fmt.Errorf("log file: %w", err)
	}

	o := &LogFileObserver{
		ch:   make(chan AgentEvent, 256),
		file: f,
		done: make(chan struct{}),
	}
	go o.loop()
	return o, nil
}

// Subscribe registers this observer with the emitter.
func (o *LogFileObserver) Subscribe(e *Emitter) {
	e.Subscribe(o.ch)
}

// Close stops the observer, drains buffered events, and flushes the log file.
// Blocks until the file is safely closed.
func (o *LogFileObserver) Close() {
	close(o.ch)
	<-o.done
}

type logEntry struct {
	Seq   int64          `json:"seq"`
	Time  time.Time      `json:"time"`
	Type  string         `json:"type"`
	Round int            `json:"round"`
	Data  map[string]any `json:"data"`
}

func (o *LogFileObserver) loop() {
	defer close(o.done)
	defer o.file.Close()
	enc := json.NewEncoder(o.file)
	for ev := range o.ch {
		entry := logEntry{
			Seq:   o.seq.Add(1),
			Time:  ev.Time,
			Type:  ev.Type,
			Round: ev.Round,
			Data:  ev.Data,
		}
		enc.Encode(entry)
		o.file.Sync()
	}
}
