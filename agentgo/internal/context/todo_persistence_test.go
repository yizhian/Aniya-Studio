package context

import (
	"testing"
)

func TestParseTodoArgs_Valid(t *testing.T) {
	argsJSON := `{
		"todos": [
			{"content": "Task 1", "status": "pending", "activeForm": "Working on task 1"},
			{"content": "Task 2", "status": "completed", "activeForm": "Working on task 2"}
		]
	}`
	items, err := ParseTodoArgs(argsJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Content != "Task 1" {
		t.Fatalf("expected Content 'Task 1', got %q", items[0].Content)
	}
	if items[0].Status != "pending" {
		t.Fatalf("expected Status pending, got %q", items[0].Status)
	}
	if items[0].ActiveForm != "Working on task 1" {
		t.Fatalf("expected ActiveForm, got %q", items[0].ActiveForm)
	}
	if items[1].Status != "completed" {
		t.Fatalf("expected second item status completed, got %q", items[1].Status)
	}
}

func TestParseTodoArgs_EmptyList(t *testing.T) {
	items, err := ParseTodoArgs(`{"todos":[]}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestParseTodoArgs_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{"not json", `not-json`, true},
		{"missing todos key", `{"other":[]}`, false}, // parses fine, just empty todos
		{"todos is string not array", `{"todos":"string"}`, true},
		{"null", `null`, false}, // JSON null unmarshals to nil slice, not error
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTodoArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("expected error=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}
