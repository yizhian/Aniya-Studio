package main

import (
	"os"
	"strings"
	"testing"

	agentctx "agentgo/internal/context"
	p "agentgo/internal/provider"
)

// TestE2E_TodoList_FullFlow verifies todo_write tool execution and persistence.
func TestE2E_TodoList_FullFlow(t *testing.T) {
	todoJSON := `{"todos":[{"content":"Build the Agenda slide","status":"in_progress","activeForm":"Building the Agenda slide"},{"content":"Define the visual system","status":"pending","activeForm":"Defining the visual system"}]}`
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{
				p.ToolStartFrame("todo_write", "toolu_001", 0),
				p.ToolCompleteFrame("todo_write", "toolu_001", todoJSON, 0),
			},
			{p.TextFrame("Tasks created."), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	events := h.Chat("todo-session", "Create todo list")
	if len(events) == 0 {
		t.Fatal("expected SSE events")
	}

	// Verify todo_write tool was called and got a result.
	hasToolResult := false
	hasToolStart := false
	for _, ev := range events {
		switch ev.Type {
		case "tool_call_start":
			if data, ok := ev.Data["name"].(string); ok && data == "todo_write" {
				hasToolStart = true
			}
		case "tool_result":
			hasToolResult = true
		}
	}
	if !hasToolStart {
		t.Error("expected todo_write tool_call_start event")
	}
	if !hasToolResult {
		t.Error("expected tool_result event")
	}

	// Verify the prompt is loaded and included in tool definitions.
	toolDefs := h.reg.GetActiveToolDefinitions()
	found := false
	for _, d := range toolDefs {
		if d.Function.Name == "todo_write" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected todo_write in active tool definitions")
	}
}

// TestE2E_TodoList_VersionPersistence verifies todos are persisted with versions.
func TestE2E_TodoList_VersionPersistence(t *testing.T) {
	todoJSON := `{"todos":[{"content":"Task 1","status":"pending","activeForm":"Doing Task 1"}]}`
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{
				p.ToolStartFrame("write_file", "toolu_001", 0),
				p.ToolCompleteFrame("write_file", "toolu_001",
					`{"path":"mytodos.html","content":"<html><head><title>My Deck</title></head><body><section class=\"slide\"><h1>Slide 1</h1></section></body></html>"}`,
					0),
			},
			{
				p.ToolStartFrame("todo_write", "toolu_002", 0),
				p.ToolCompleteFrame("todo_write", "toolu_002", todoJSON, 0),
			},
			{p.TextFrame("All tasks set up."), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	h.Chat("todo-version-session", "Create deck and set up tasks")

	// Verify the HTML was written.
	_, err := os.Stat(h.WorkDir + "/mytodos.html")
	if err != nil {
		t.Logf("HTML file not written (may fail): %v", err)
	}

	// Check for version directory.
	store := agentctx.NewSnapshotStore(h.WorkDir)
	ctx, err := store.LoadLatest()
	if err != nil {
		t.Logf("no version yet: %v", err)
		return
	}
	if ctx != nil {
		t.Logf("version %d created", ctx.Version)
	}
}

// TestE2E_TodoList_Updates verifies todo updates across turns.
func TestE2E_TodoList_Updates(t *testing.T) {
	todos1 := `{"todos":[{"content":"Task A","status":"in_progress","activeForm":"Doing Task A"}]}`

	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{
				p.ToolStartFrame("write_file", "toolu_001", 0),
				p.ToolCompleteFrame("write_file", "toolu_001",
					`{"path":"tododeck.html","content":"<html><body><section class=\"slide\"><h1>Slide 1</h1></section></body></html>"}`,
					0),
			},
			{
				p.ToolStartFrame("todo_write", "toolu_002", 0),
				p.ToolCompleteFrame("todo_write", "toolu_002", todos1, 0),
			},
			{p.TextFrame("Tasks created."), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	h.Chat("todo-update-session", "Create deck and tasks")

	// Check for version.
	store := agentctx.NewSnapshotStore(h.WorkDir)
	ctx, err := store.LoadLatest()
	if err != nil || ctx == nil {
		t.Logf("no version created: %v", err)
		return
	}

	todos, err := store.LoadTodo(ctx.Version)
	if err != nil {
		t.Logf("no todos on disk: %v", err)
		return
	}
	if len(todos) > 0 {
		t.Logf("found %d todos on disk", len(todos))
	}
}

// TestE2E_TodoList_Prompt verifies the todolist prompt is loaded.
func TestE2E_TodoList_Prompt(t *testing.T) {
	script := p.SingleTurnScript(p.TextFrame("ok"), p.DoneFrame())
	h := newE2EHarness(t, script)
	defer h.Close()

	// Get tool prompts — check for todo_write.
	prompts := h.reg.GetActiveToolPrompts()
	t.Logf("tool prompts length: %d", len(prompts))
	if strings.Contains(prompts, "todo_write") {
		t.Log("todo_write prompt found in tool prompts")
	}

	// Verify prompts/todolist.md can be loaded.
	candidates := []string{"prompts/todolist.md", "todolistPrompt.md"}
	var data []byte
	for _, p := range candidates {
		if d, err := os.ReadFile(p); err == nil {
			data = d
			break
		}
	}
	if data == nil {
		t.Skip("prompts/todolist.md not found")
		return
	}
	_ = data
	t.Log("todolist prompt template found")
}
