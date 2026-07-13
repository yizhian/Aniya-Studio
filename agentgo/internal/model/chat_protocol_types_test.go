package model

import (
	"encoding/json"
	"testing"
)

func TestMessage_JSONRoundtrip(t *testing.T) {
	msg := Message{
		Role:             "assistant",
		Content:          "Hello, world!",
		ReasoningContent: "Let me think about this...",
		ToolCalls: []ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: ToolCallFunction{
					Name:      "read_file",
					Arguments: `{"path":"test.html"}`,
				},
			},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Role != "assistant" {
		t.Errorf("role: got %q", decoded.Role)
	}
	if decoded.Content != "Hello, world!" {
		t.Errorf("content: got %q", decoded.Content)
	}
	if decoded.ReasoningContent != "Let me think about this..." {
		t.Errorf("reasoning_content: got %q", decoded.ReasoningContent)
	}
	if len(decoded.ToolCalls) != 1 {
		t.Fatalf("tool_calls: expected 1, got %d", len(decoded.ToolCalls))
	}
	if decoded.ToolCalls[0].Function.Name != "read_file" {
		t.Errorf("tool name: got %q", decoded.ToolCalls[0].Function.Name)
	}
}

func TestMessage_OmitEmpty(t *testing.T) {
	msg := Message{Role: "user", Content: "hi"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	json.Unmarshal(data, &raw)
	if _, ok := raw["tool_call_id"]; ok {
		t.Error("tool_call_id should be omitted when empty")
	}
	if _, ok := raw["name"]; ok {
		t.Error("name should be omitted when empty")
	}
	if _, ok := raw["reasoning_content"]; ok {
		t.Error("reasoning_content should be omitted when empty")
	}
}

func TestToolDefinition_JSONRoundtrip(t *testing.T) {
	td := ToolDefinition{
		Type: "function",
		Function: FunctionSpec{
			Name:        "read_file",
			Description: "Read a file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
			},
		},
	}
	data, err := json.Marshal(td)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ToolDefinition
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Function.Name != "read_file" {
		t.Errorf("name: got %q", decoded.Function.Name)
	}
}

func TestUsage_JSONRoundtrip(t *testing.T) {
	usage := Usage{PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300}
	data, err := json.Marshal(usage)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Usage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.TotalTokens != 300 {
		t.Errorf("total: got %d", decoded.TotalTokens)
	}
}
