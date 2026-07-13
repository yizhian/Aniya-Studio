package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	p "agentgo/internal/provider"
)

// Tools resolve file paths relative to the workspace directory (h.WorkDir).

// TestE2E_ToolUse_ReadFile verifies a tool-using conversation flow.
func TestE2E_ToolUse_ReadFile(t *testing.T) {
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{
				p.ToolStartFrame("read_file", "toolu_001", 0),
				p.ToolCompleteFrame("read_file", "toolu_001", `{"path":"_e2e_read_test.txt"}`, 0),
			},
			{p.TextFrame("File contents: hello world"), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	// Create the file in the workspace so the tool can find it.
	os.WriteFile(filepath.Join(h.WorkDir, "_e2e_read_test.txt"), []byte("hello world"), 0644)

	events := h.Chat("tool-session", "Read _e2e_read_test.txt")
	if len(events) == 0 {
		t.Fatal("expected SSE events")
	}

	hasToolStart := false
	hasToolResult := false
	hasText := false
	for _, ev := range events {
		switch ev.Type {
		case "tool_call_start":
			hasToolStart = true
		case "tool_result":
			hasToolResult = true
		case "text":
			hasText = true
		}
	}
	if !hasToolStart {
		t.Error("expected tool_call_start event")
	}
	if !hasToolResult {
		t.Error("expected tool_result event")
	}
	if !hasText {
		t.Error("expected text event in second round")
	}
}

// TestE2E_ToolUse_WriteFile verifies write_file tool execution.
func TestE2E_ToolUse_WriteFile(t *testing.T) {
	testFile := "_e2e_write_test.html"
	script := p.SingleTurnScript(
		p.ToolStartFrame("write_file", "toolu_001", 0),
		p.ToolCompleteFrame("write_file", "toolu_001",
			`{"path":"`+testFile+`","content":"<h1>Hello</h1>"}`, 0),
	)
	h := newE2EHarness(t, script)
	defer h.Close()

	events := h.Chat("write-session", "Write a file")
	_ = events

	// Verify the file was written in the workspace.
	data, err := os.ReadFile(filepath.Join(h.WorkDir, testFile))
	if err != nil {
		t.Fatalf("expected %s to be written: %v", testFile, err)
	}
	if !strings.Contains(string(data), "<h1>Hello</h1>") {
		t.Errorf("unexpected file content: %s", string(data))
	}
}

// TestE2E_ToolUse_GrepSearch verifies grep_search tool execution.
func TestE2E_ToolUse_GrepSearch(t *testing.T) {
	testFile := "_e2e_grep_test.txt"

	script := p.SingleTurnScript(
		p.ToolStartFrame("grep_search", "toolu_001", 0),
		p.ToolCompleteFrame("grep_search", "toolu_001", `{"pattern":"TODO"}`, 0),
	)
	h := newE2EHarness(t, script)
	defer h.Close()

	// Create the file in the workspace so grep_search can find it.
	os.WriteFile(filepath.Join(h.WorkDir, testFile), []byte("TODO: finish this\nNormal line\n"), 0644)

	events := h.Chat("grep-session", "Search for TODO")

	hasToolResult := false
	for _, ev := range events {
		if ev.Type == "tool_result" {
			hasToolResult = true
		}
	}
	if !hasToolResult {
		t.Error("expected tool_result event")
	}
}

// TestE2E_ToolUse_MultipleRound verifies a multi-round tool-using conversation.
func TestE2E_ToolUse_MultiRound(t *testing.T) {
	dataFile := "_e2e_data.txt"

	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{
				p.ToolStartFrame("read_file", "toolu_001", 0),
				p.ToolCompleteFrame("read_file", "toolu_001", `{"path":"`+dataFile+`"}`, 0),
			},
			{
				p.ToolStartFrame("grep_search", "toolu_002", 0),
				p.ToolCompleteFrame("grep_search", "toolu_002", `{"pattern":"important"}`, 0),
			},
			{p.TextFrame("Found the data you need."), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	// Create the file in the workspace so read_file can find it.
	os.WriteFile(filepath.Join(h.WorkDir, dataFile), []byte("important data here"), 0644)

	events := h.Chat("multi-tool-session", "Find important data")
	if len(events) == 0 {
		t.Fatal("expected SSE events")
	}

	toolStartCount := 0
	toolResultCount := 0
	for _, ev := range events {
		switch ev.Type {
		case "tool_call_start":
			toolStartCount++
		case "tool_result":
			toolResultCount++
		}
	}
	if toolStartCount != 2 {
		t.Errorf("expected 2 tool_call_start events, got %d", toolStartCount)
	}
	if toolResultCount < 2 {
		t.Errorf("expected at least 2 tool_result events, got %d", toolResultCount)
	}
}
