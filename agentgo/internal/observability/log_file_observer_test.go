package observability

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogFileObserver_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	obs, err := NewLogFileObserver(dir, "test-session")
	if err != nil {
		t.Fatalf("NewLogFileObserver failed: %v", err)
	}
	defer obs.Close()

	// Check that a file was created with the correct naming pattern.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in log dir, got %d", len(entries))
	}
	name := entries[0].Name()
	if !strings.HasPrefix(name, "sess_test-session_") {
		t.Fatalf("expected file name prefix sess_test-session_, got %q", name)
	}
	if !strings.HasSuffix(name, ".jsonl") {
		t.Fatalf("expected .jsonl extension, got %q", name)
	}
}

func TestLogFileObserver_JSONL_Format(t *testing.T) {
	dir := t.TempDir()
	obs, err := NewLogFileObserver(dir, "test-session")
	if err != nil {
		t.Fatalf("NewLogFileObserver failed: %v", err)
	}

	e := NewEmitter()
	obs.Subscribe(e)

	// Emit 3 events.
	e.Emit(AgentEvent{Type: "round", Round: 1, Data: map[string]any{}})
	e.Emit(AgentEvent{Type: "text", Round: 1, Data: map[string]any{"text": "hello"}})
	e.Emit(AgentEvent{Type: "round_end", Round: 1, Data: map[string]any{"duration_ms": 42}})

	// Close flushes and waits for done.
	obs.Close()

	// Read the file and verify JSONL format.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 JSONL lines, got %d", len(lines))
	}

	for i, line := range lines {
		var entry logEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", i, err)
		}
		if entry.Seq != int64(i+1) {
			t.Fatalf("line %d: expected seq=%d, got %d", i, i+1, entry.Seq)
		}
		if entry.Type == "" {
			t.Fatalf("line %d: expected non-empty type", i)
		}
	}
}

func TestLogFileObserver_Close(t *testing.T) {
	dir := t.TempDir()
	obs, err := NewLogFileObserver(dir, "close-test")
	if err != nil {
		t.Fatalf("NewLogFileObserver failed: %v", err)
	}

	e := NewEmitter()
	obs.Subscribe(e)
	e.Emit(AgentEvent{Type: "round", Round: 1, Data: map[string]any{"marker": "pre-close"}})
	obs.Close()

	// After close, the file should exist and contain the event.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log file after close, got %d", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "pre-close") {
		t.Fatal("expected 'pre-close' marker in log file after Close")
	}
}

func TestLogFileObserver_EmptyDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-subdir")
	obs, err := NewLogFileObserver(dir, "empty-test")
	if err != nil {
		t.Fatalf("NewLogFileObserver on new subdir should succeed (MkdirAll): %v", err)
	}
	defer obs.Close()

	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("expected log file to be created in new subdir")
	}
}
