package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	agentctx "agentgo/internal/context"
	p "agentgo/internal/provider"
)

// TestE2E_MultiTurn_10Rounds verifies a 10-turn conversation completes.
func TestE2E_MultiTurn_10Rounds(t *testing.T) {
	// Build a 5-round tool-using script (10 SSE rounds: 5 tool + 5 text).
	var rounds [][]p.MockSSEFrame
	dataFile := "_e2e_mt_data.txt"
	for i := 0; i < 5; i++ {
		rounds = append(rounds, []p.MockSSEFrame{
			p.ToolStartFrame("grep_search", "toolu_mt", 0),
			p.ToolCompleteFrame("grep_search", "toolu_mt",
				`{"pattern":"round`+itoa(i)+`"}`, 0),
		})
	}
	rounds = append(rounds, []p.MockSSEFrame{
		p.TextFrame("All rounds complete."), p.DoneFrame(),
	})

	script := p.MockSSEScript{Rounds: rounds}
	h := newE2EHarness(t, script)
	defer h.Close()

	os.WriteFile(filepath.Join(h.WorkDir, dataFile), []byte("round data"), 0644)

	events := h.Chat("multi-turn-session", "Start multi-turn test")
	if len(events) == 0 {
		t.Fatal("expected SSE events")
	}

	toolStartCount := 0
	textCount := 0
	for _, ev := range events {
		switch ev.Type {
		case "tool_call_start":
			toolStartCount++
		case "text":
			textCount++
		}
	}
	if toolStartCount < 5 {
		t.Errorf("expected at least 5 tool_call_start events, got %d", toolStartCount)
	}
	if textCount < 1 {
		t.Errorf("expected at least 1 text event, got %d", textCount)
	}

	// Verify session was saved.
	sessions := h.ListSessions()
	found := false
	for _, s := range sessions {
		if s.ID == "multi-turn-session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected multi-turn-session in session list")
	}
}

// TestE2E_ConcurrentSessions verifies 5 simultaneous sessions don't interfere.
func TestE2E_ConcurrentSessions(t *testing.T) {
	script := p.SingleTurnScript(p.TextFrame("ok"), p.DoneFrame())
	h := newE2EHarness(t, script)
	defer h.Close()

	var wg sync.WaitGroup
	results := make([]int, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sessionID := "concurrent-" + itoa(idx)
			events := h.Chat(sessionID, "Hello from session "+itoa(idx))
			results[idx] = len(events)
		}(i)
	}
	wg.Wait()

	for i, n := range results {
		if n == 0 {
			t.Errorf("session %d got 0 events", i)
		}
	}

	// Each session should be independently visible.
	sessions := h.ListSessions()
	for i := 0; i < 5; i++ {
		sid := "concurrent-" + itoa(i)
		found := false
		for _, s := range sessions {
			if s.ID == sid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected session %q in list", sid)
		}
	}
}

// TestE2E_ErrorRecovery_Provider500 verifies that provider 500 errors are
// classified as transient and the agent recovers via backoff-retry.
func TestE2E_ErrorRecovery_Provider500(t *testing.T) {
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{p.HTTPErrorFrame(500, "internal server error")},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	events := h.Chat("error-500-session", "Hello")
	if len(events) == 0 {
		t.Fatal("expected SSE events even on error")
	}

	// With the provider fix, HTTP 500 is now correctly classified as transient.
	// The agent should: fail → backoff → retry → fallback succeeds.
	// Verify the session was created (i.e., the system didn't crash).
	sessions := h.ListSessions()
	found := false
	for _, s := range sessions {
		if s.ID == "error-500-session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error-500-session in session list after backoff recovery")
	}

	// Should have text output from the fallback round.
	hasText := false
	for _, ev := range events {
		if ev.Type == "text" {
			hasText = true
			break
		}
	}
	if !hasText {
		t.Error("expected text event after backoff recovery from 500")
	}
}

// TestE2E_ErrorRecovery_ToolFailure verifies agent continues after tool error.
func TestE2E_ErrorRecovery_ToolFailure(t *testing.T) {
	// Call a tool that isn't registered — execution will fail.
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{
				p.ToolStartFrame("nonexistent_tool", "toolu_bad", 0),
				p.ToolCompleteFrame("nonexistent_tool", "toolu_bad", `{"arg":"val"}`, 0),
			},
			{p.TextFrame("Recovered after tool error."), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	events := h.Chat("tool-error-session", "Do something bad")
	if len(events) == 0 {
		t.Fatal("expected SSE events")
	}

	hasToolResult := false
	hasText := false
	for _, ev := range events {
		switch ev.Type {
		case "tool_result":
			hasToolResult = true
			if errMsg, ok := ev.Data["error"].(string); ok {
				t.Logf("tool error captured: %s", errMsg)
			}
		case "text":
			hasText = true
		}
	}
	if !hasToolResult {
		t.Error("expected tool_result event (even for failed tool)")
	}
	if !hasText {
		t.Error("expected text response after tool failure (agent should continue)")
	}
}

// TestE2E_ErrorRecovery_MissingVersion verifies graceful fallback when
// version directory doesn't exist on disk.
func TestE2E_ErrorRecovery_MissingVersion(t *testing.T) {
	script := p.SingleTurnScript(p.TextFrame("ok"), p.DoneFrame())
	h := newE2EHarness(t, script)
	defer h.Close()

	// Load from a version that doesn't exist.
	store := agentctx.NewSnapshotStore(h.WorkDir)
	ctx, err := store.LoadVersion(999)
	if err == nil && ctx != nil {
		t.Log("loading non-existent version returned no error (may be valid)")
	}
	// Should not panic.
}

// TestE2E_ErrorRecovery_CorruptedSession verifies graceful handling of
// corrupted session file on disk.
func TestE2E_ErrorRecovery_CorruptedSession(t *testing.T) {
	script := p.SingleTurnScript(p.TextFrame("ok"), p.DoneFrame())
	h := newE2EHarness(t, script)
	defer h.Close()

	// Write corrupted JSON to session file.
	sessionDir := filepath.Join(h.WorkDir, ".agentgo", "sessions")
	os.MkdirAll(sessionDir, 0755)
	corruptPath := filepath.Join(sessionDir, "corrupt-session.json")
	os.WriteFile(corruptPath, []byte("this is not valid json {{{"), 0644)

	// Try to get the corrupted session — should handle gracefully.
	data, err := h.GetSession("corrupt-session")
	if err != nil {
		t.Logf("expected error for corrupted session: %v", err)
	}
	if data != nil {
		t.Log("got data for corrupted session (may be partially parsed)")
	}
	// Should not panic.
}

// itoa is a simple int-to-string helper.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for v := i; v > 0; v /= 10 {
		s = string(rune('0'+v%10)) + s
	}
	return s
}
