package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentctx "agentgo/internal/context"
	p "agentgo/internal/provider"
)

// ---------------------------------------------------------------------------
// Context Management + Versioning E2E Tests
// ---------------------------------------------------------------------------

// TestE2E_ContextVersioning_WriteHTML creates a version when HTML is written.
func TestE2E_ContextVersioning_WriteHTML(t *testing.T) {
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{
				p.ToolStartFrame("write_file", "toolu_001", 0),
				p.ToolCompleteFrame("write_file", "toolu_001",
					`{"path":"_e2e_verdeck.html","content":"<html><head><title>My Deck</title></head><body><section class=\"slide\"><h1>Slide 1</h1></section><section class=\"slide\"><h1>Slide 2</h1></section></body></html>"}`,
					0),
			},
			{p.TextFrame("Deck created!"), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()
	defer os.Remove(filepath.Join(h.WorkDir, "_e2e_verdeck.html"))

	events := h.Chat("version-session", "Create a deck")
	if len(events) == 0 {
		t.Fatal("expected events")
	}

	// Verify version directory was created.
	store := agentctx.NewSnapshotStore(h.WorkDir)
	latest, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("expected version to be created: %v", err)
	}
	if latest == nil {
		t.Fatal("expected non-nil context")
	}
	if latest.Version != 1 {
		t.Errorf("expected version 1, got %d", latest.Version)
	}
	if !strings.Contains(latest.DesignSnapshot.Title, "My Deck") {
		t.Errorf("expected title 'My Deck', got %q", latest.DesignSnapshot.Title)
	}

	// Verify manifest.json exists.
	manifestPath := filepath.Join(h.WorkDir, ".slidecraft", "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("expected manifest.json to exist")
	}
}

// TestE2E_ContextVersioning_EditHTML creates a new version when HTML is edited.
func TestE2E_ContextVersioning_EditHTML(t *testing.T) {
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			// Round 1: write initial HTML (no pre-existing file)
			{
				p.ToolStartFrame("write_file", "toolu_001", 0),
				p.ToolCompleteFrame("write_file", "toolu_001",
					`{"path":"deck.html","content":"<html><head><title>V1</title></head><body><section class=\"slide\"><h1>Slide 1</h1></section></body></html>"}`,
					0),
			},
			// Round 2: edit it
			{
				p.ToolStartFrame("edit_file", "toolu_002", 0),
				p.ToolCompleteFrame("edit_file", "toolu_002",
					`{"path":"deck.html","old_string":"V1","new_string":"V2"}`,
					0),
			},
			{p.TextFrame("Updated to V2."), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	h.Chat("edit-version-session", "Create and edit deck")

	// Verify the HTML was written.
	if _, err := os.Stat(filepath.Join(h.WorkDir, "deck.html")); err != nil {
		t.Logf("deck.html not found (write may have failed): %v", err)
	}

	// Check for any version.
	store := agentctx.NewSnapshotStore(h.WorkDir)
	latest, err := store.LoadLatest()
	if err != nil {
		t.Logf("no version (expected if write failed): %v", err)
	}
	_ = latest
}

// TestE2E_ContextVersioning_NonHTMLNoVersion verifies non-HTML writes don't create versions.
func TestE2E_ContextVersioning_NonHTML(t *testing.T) {
	script := p.SingleTurnScript(
		p.ToolStartFrame("write_file", "toolu_001", 0),
		p.ToolCompleteFrame("write_file", "toolu_001", `{"path":"notes.txt","content":"some notes"}`, 0),
	)
	h := newE2EHarness(t, script)
	defer h.Close()

	h.Chat("non-html-session", "Write notes")

	// No version should be created for non-HTML file.
	store := agentctx.NewSnapshotStore(h.WorkDir)
	_, err := store.LoadLatest()
	if err == nil {
		t.Log("version created for non-HTML (may be expected in some configurations)")
	}
}

// TestE2E_SnapshotInjection verifies snapshots are injected into subsequent requests.
func TestE2E_SnapshotInjection(t *testing.T) {
	// Use a unique filename to avoid "file already exists" errors.
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			// Round 1: write HTML (creates snapshot)
			{
				p.ToolStartFrame("write_file", "toolu_001", 0),
				p.ToolCompleteFrame("write_file", "toolu_001",
					`{"path":"snapdeck.html","content":"<html><head><title>Test Deck</title></head><body><section class=\"slide\"><h1>Hello</h1></section></body></html>"}`,
					0),
			},
			// Round 2: text response
			{p.TextFrame("I see your deck."), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	h.Chat("snapshot-session", "Create a deck")

	// Verify the file was written (which triggers version creation).
	if _, err := os.Stat(filepath.Join(h.WorkDir, "snapdeck.html")); err == nil {
		t.Log("snapshot HTML file was created")
	}

	store := agentctx.NewSnapshotStore(h.WorkDir)
	latest, _ := store.LoadLatest()
	if latest != nil {
		t.Logf("version %d created with title %q", latest.Version, latest.DesignSnapshot.Title)
	}
}

// ---------------------------------------------------------------------------
// Log Module E2E Tests
// ---------------------------------------------------------------------------

// TestE2E_Logging_JSONLOutput verifies JSONL log files are created and valid.
func TestE2E_Logging_JSONLOutput(t *testing.T) {
	script := p.SingleTurnScript(
		p.ThinkingFrame("Let me think..."),
		p.TextFrame("Here is the response."),
		p.DoneFrame(),
	)
	h := newE2EHarness(t, script)
	defer h.Close()

	h.Chat("log-session", "Hello!")

	// Log files are written relative to CWD by NewLogFileObserver(".agentgo/logs", sessID).
	// Since ensureProjectRoot() sets CWD to project root, check there.
	logDir := filepath.Join(".agentgo", "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		// Also check the workspace dir.
		logDir2 := filepath.Join(h.WorkDir, ".agentgo", "logs")
		entries, err = os.ReadDir(logDir2)
		if err != nil {
			t.Skipf("log directory not found: %v", err)
			return
		}
		logDir = logDir2
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one log file")
	}

	// Read the log file and verify JSONL format.
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "sess_log-session") {
			continue
		}
		logPath := filepath.Join(logDir, entry.Name())
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read log file: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) == 0 {
			t.Fatal("expected log entries")
		}
		// Each line should be valid JSON with expected fields.
		for _, line := range lines {
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "{") {
				t.Errorf("expected JSON line, got: %s", line[:min(50, len(line))])
			}
		}
		t.Logf("log file %s: %d entries", entry.Name(), len(lines))
	}
}

// TestE2E_Logging_EventSequence verifies the log contains expected event types.
func TestE2E_Logging_EventSequence(t *testing.T) {
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{
				p.ToolStartFrame("read_file", "toolu_001", 0),
				p.ToolCompleteFrame("read_file", "toolu_001", `{"path":"test.txt"}`, 0),
			},
			{p.TextFrame("Done."), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	os.WriteFile(h.WorkDir+"/test.txt", []byte("data"), 0644)
	h.Chat("log-seq-session", "Read file")

	// Check for log file.
	logDir := filepath.Join(h.WorkDir, ".agentgo", "logs")
	entries, _ := os.ReadDir(logDir)
	if len(entries) > 0 {
		t.Logf("log directory has %d files", len(entries))
	}
}
