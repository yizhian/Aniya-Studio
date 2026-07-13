package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agentgo/internal/toolkit/contracts"
)

func TestTodoWrite_Descriptor(t *testing.T) {
	tool := NewTodoWriteTool()
	d := tool.Descriptor()

	if d.Name != "todo_write" {
		t.Fatalf("expected name todo_write, got %q", d.Name)
	}
	if d.Description == "" {
		t.Fatal("expected non-empty Description")
	}
	if !d.Flags.ReadOnly {
		t.Fatal("expected ReadOnly flag")
	}
	if !d.Flags.ConcurrencySafe {
		t.Fatal("expected ConcurrencySafe flag")
	}
	schema := d.InputJSONSchema
	if schema == nil {
		t.Fatal("expected non-nil InputJSONSchema")
	}
	if schema["type"] != "object" {
		t.Fatalf("expected schema type object, got %v", schema["type"])
	}
	if schema["required"] == nil {
		t.Fatal("expected required field in schema")
	}
}

func TestTodoWrite_PromptLoaded(t *testing.T) {
	prompt := todolistPromptText
	if prompt == "" {
		t.Fatal("expected non-empty prompt from embedded todolistPromptText")
	}
	if !strings.Contains(prompt, "TodoWrite") && !strings.Contains(prompt, "todo") {
		t.Fatal("prompt should reference todo/todolist concepts")
	}
}

func TestTodoWrite_Call_Success(t *testing.T) {
	tool := NewTodoWriteTool()
	args := `{
		"todos": [
			{"content": "Build the Agenda slide", "status": "in_progress", "activeForm": "Building the Agenda slide"},
			{"content": "Define the visual system", "status": "pending", "activeForm": "Defining the visual system"},
			{"content": "Scaffold HTML", "status": "completed", "activeForm": "Scaffolding HTML"}
		]
	}`
	result := tool.Call(context.Background(), contracts.ToolCallArgs{ArgsJSON: args})
	if result.IsError {
		t.Fatalf("unexpected error: %s (code=%s)", result.ErrorMessage, result.ErrorCode)
	}
	if !strings.Contains(result.Content, "Todo list updated") {
		t.Fatalf("expected success message, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "3 items") {
		t.Fatalf("expected 3 items count, got: %s", result.Content)
	}
	count, ok := result.Metadata["count"].(int)
	if !ok || count != 3 {
		t.Fatalf("expected metadata count=3, got %v (%T)", result.Metadata["count"], result.Metadata["count"])
	}
}

func TestTodoWrite_Call_EmptyList(t *testing.T) {
	tool := NewTodoWriteTool()
	result := tool.Call(context.Background(), contracts.ToolCallArgs{ArgsJSON: `{"todos":[]}`})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ErrorMessage)
	}
	count, _ := result.Metadata["count"].(int)
	if count != 0 {
		t.Fatalf("expected count=0, got %v", count)
	}
}

func TestTodoWrite_Call_InvalidJSON(t *testing.T) {
	tool := NewTodoWriteTool()
	result := tool.Call(context.Background(), contracts.ToolCallArgs{ArgsJSON: `{not valid}`})
	if !result.IsError {
		t.Fatal("expected error for invalid JSON")
	}
	if result.ErrorCode != "invalid_json" {
		t.Fatalf("expected error_code invalid_json, got %q", result.ErrorCode)
	}
}

func TestTodoWrite_Call_EmptyContent(t *testing.T) {
	tool := NewTodoWriteTool()
	args := `{"todos":[{"content":"","status":"pending","activeForm":"Doing stuff"}]}`
	result := tool.Call(context.Background(), contracts.ToolCallArgs{ArgsJSON: args})
	if !result.IsError {
		t.Fatal("expected error for empty content")
	}
	if result.ErrorCode != "invalid_todo" {
		t.Fatalf("expected error_code invalid_todo, got %q", result.ErrorCode)
	}
	if !strings.Contains(result.ErrorMessage, "todo item 0") {
		t.Fatalf("expected error message to mention item index, got: %s", result.ErrorMessage)
	}
}

func TestTodoWrite_Call_EmptyActiveForm(t *testing.T) {
	tool := NewTodoWriteTool()
	args := `{"todos":[{"content":"Do something","status":"pending","activeForm":""}]}`
	result := tool.Call(context.Background(), contracts.ToolCallArgs{ArgsJSON: args})
	if !result.IsError {
		t.Fatal("expected error for empty activeForm")
	}
	if result.ErrorCode != "invalid_todo" {
		t.Fatalf("expected error_code invalid_todo, got %q", result.ErrorCode)
	}
}

func TestTodoWrite_Call_InvalidStatus(t *testing.T) {
	tool := NewTodoWriteTool()
	args := `{"todos":[{"content":"Do something","status":"done","activeForm":"Doing something"}]}`
	result := tool.Call(context.Background(), contracts.ToolCallArgs{ArgsJSON: args})
	if !result.IsError {
		t.Fatal("expected error for invalid status")
	}
	if result.ErrorCode != "invalid_status" {
		t.Fatalf("expected error_code invalid_status, got %q", result.ErrorCode)
	}
	if !strings.Contains(result.ErrorMessage, `"done"`) {
		t.Fatalf("expected error message to include invalid status value, got: %s", result.ErrorMessage)
	}
}

func TestTodoWrite_Call_AllStatuses(t *testing.T) {
	tool := NewTodoWriteTool()
	tests := []struct {
		status string
		valid  bool
	}{
		{"pending", true},
		{"in_progress", true},
		{"completed", true},
		{"unknown", false},
		{"", false},
		{"PENDING", false},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			args := `{"todos":[{"content":"Task","status":"` + tt.status + `","activeForm":"Doing task"}]}`
			result := tool.Call(context.Background(), contracts.ToolCallArgs{ArgsJSON: args})
			if tt.valid && result.IsError {
				t.Fatalf("expected success for status %q, got error: %s", tt.status, result.ErrorMessage)
			}
			if !tt.valid && !result.IsError {
				t.Fatalf("expected error for status %q, got success", tt.status)
			}
		})
	}
}

func TestTodoWrite_Call_LargeList(t *testing.T) {
	tool := NewTodoWriteTool()
	todos := make([]map[string]string, 20)
	for i := 0; i < 20; i++ {
		todos[i] = map[string]string{
			"content":    "Task " + string(rune('A'+i%26)),
			"status":     "pending",
			"activeForm": "Working on task " + string(rune('A'+i%26)),
		}
	}
	argsJSON, _ := json.Marshal(map[string]any{"todos": todos})
	result := tool.Call(context.Background(), contracts.ToolCallArgs{ArgsJSON: string(argsJSON)})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ErrorMessage)
	}
	count, _ := result.Metadata["count"].(int)
	if count != 20 {
		t.Fatalf("expected count=20, got %v", count)
	}
}
