package provider

import (
	"testing"

	"agentgo/internal/model"
)

func TestToolResultToAnthropic(t *testing.T) {
	msg := model.Message{
		Role:       "tool",
		Content:    "file contents here",
		ToolCallID: "toolu_abc123",
	}
	result := toolResultToAnthropic(msg)
	if result.Role != "user" {
		t.Errorf("role: expected user, got %q", result.Role)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}
	block := result.Content[0]
	if block.Type != "tool_result" {
		t.Errorf("type: expected tool_result, got %q", block.Type)
	}
	if block.ToolUseID != "toolu_abc123" {
		t.Errorf("tool_use_id: got %q", block.ToolUseID)
	}
	if block.Content != "file contents here" {
		t.Errorf("content: got %q", block.Content)
	}
}

func TestRegularToAnthropic_WithReasoning(t *testing.T) {
	msg := model.Message{
		Role:             "assistant",
		Content:          "Here is the answer",
		ReasoningContent: "Let me think...",
	}
	result := regularToAnthropic(msg)
	if len(result.Content) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result.Content))
	}
	if result.Content[0].Type != "text" {
		t.Errorf("first block type: got %q", result.Content[0].Type)
	}
	if result.Content[1].Type != "thinking" {
		t.Errorf("second block type: got %q", result.Content[1].Type)
	}
	if result.Content[1].Thinking != "Let me think..." {
		t.Errorf("thinking content: got %q", result.Content[1].Thinking)
	}
}

func TestRegularToAnthropic_EmptyContent(t *testing.T) {
	msg := model.Message{Role: "assistant"}
	result := regularToAnthropic(msg)
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 block for empty content, got %d", len(result.Content))
	}
	if result.Content[0].Type != "text" || result.Content[0].Text != " " {
		t.Errorf("expected placeholder text, got type=%q text=%q", result.Content[0].Type, result.Content[0].Text)
	}
}

func TestRegularToAnthropic_WithToolCalls(t *testing.T) {
	msg := model.Message{
		Role:    "assistant",
		Content: "Using tools",
		ToolCalls: []model.ToolCall{
			{ID: "tc1", Type: "function", Function: model.ToolCallFunction{Name: "read", Arguments: `{}`}},
		},
	}
	result := regularToAnthropic(msg)
	if len(result.Content) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result.Content))
	}
	if result.Content[1].Type != "tool_use" {
		t.Errorf("expected tool_use block, got %q", result.Content[1].Type)
	}
}

func TestMessagesToAnthropic_SkipsSystem(t *testing.T) {
	msgs := []model.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
	}
	result := messagesToAnthropic(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message (system skipped), got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("role: got %q", result[0].Role)
	}
}

func TestMessagesToAnthropic_ToolRole(t *testing.T) {
	msgs := []model.Message{
		{Role: "tool", Content: "result", ToolCallID: "tc1"},
	}
	result := messagesToAnthropic(msgs)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("tool result role: got %q", result[0].Role)
	}
}

func TestToolsToOpenAI_NilParams(t *testing.T) {
	tools := []model.ToolDefinition{
		{Type: "function", Function: model.FunctionSpec{Name: "test", Parameters: nil}},
	}
	result := toolsToOpenAI(tools)
	if len(result) != 1 {
		t.Fatal("expected 1 tool")
	}
	if result[0].Function.Parameters == nil {
		t.Error("expected default params for nil")
	}
}

func TestToolsToAnthropic_NilParams(t *testing.T) {
	tools := []model.ToolDefinition{
		{Type: "function", Function: model.FunctionSpec{Name: "test", Parameters: nil}},
	}
	result := toolsToAnthropic(tools)
	if len(result) != 1 {
		t.Fatal("expected 1 tool")
	}
	if result[0].InputSchema == nil {
		t.Error("expected default schema for nil")
	}
}
