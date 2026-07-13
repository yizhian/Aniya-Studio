package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"agentgo/internal/toolkit/contracts"
	"agentgo/internal/toolkit/registry"
)

func TestNewToolSearchTool(t *testing.T) {
	reg := registry.NewToolRegistry()
	ts := NewToolSearchTool(reg)
	if ts == nil {
		t.Fatal("expected non-nil ToolSearchTool")
	}
	if ts.reg != reg {
		t.Fatal("registry not set correctly")
	}
}

func TestToolSearchToolDescriptor(t *testing.T) {
	reg := registry.NewToolRegistry()
	ts := NewToolSearchTool(reg)
	desc := ts.Descriptor()
	if desc.Name != "tool_search" {
		t.Errorf("expected name tool_search, got %q", desc.Name)
	}
	if !desc.Flags.ReadOnly {
		t.Error("expected ReadOnly flag")
	}
	if !desc.Flags.ConcurrencySafe {
		t.Error("expected ConcurrencySafe flag")
	}
	props, ok := desc.InputJSONSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing properties in schema")
	}
	if _, ok := props["query"]; !ok {
		t.Error("missing query property")
	}
	if _, ok := props["exact"]; !ok {
		t.Error("missing exact property")
	}
	required, ok := desc.InputJSONSchema["required"].([]any)
	if !ok {
		t.Fatal("missing required array")
	}
	found := false
	for _, r := range required {
		if r == "query" {
			found = true
			break
		}
	}
	if !found {
		t.Error("query not in required")
	}
}

func TestToolSearchTool_Call_NilRegistry(t *testing.T) {
	ts := &ToolSearchTool{reg: nil}
	result := ts.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"query":"test"}`,
	})
	if !result.IsError || result.ErrorCode != "registry_nil" {
		t.Fatalf("expected registry_nil error, got %+v", result)
	}
}

func TestToolSearchTool_Call_InvalidJSON(t *testing.T) {
	reg := registry.NewToolRegistry()
	ts := NewToolSearchTool(reg)
	result := ts.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{broken`,
	})
	if !result.IsError || result.ErrorCode != "invalid_json" {
		t.Fatalf("expected invalid_json, got %+v", result)
	}
}

func TestToolSearchTool_Call_EmptyQuery(t *testing.T) {
	reg := registry.NewToolRegistry()
	ts := NewToolSearchTool(reg)
	result := ts.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"query":"  "}`,
	})
	if !result.IsError || result.ErrorCode != "invalid_query" {
		t.Fatalf("expected invalid_query, got %+v", result)
	}
}

func TestToolSearchTool_Call_NoMatches(t *testing.T) {
	reg := registry.NewToolRegistry()
	// Register a deferred tool first
	_ = reg.RegisterDeferred("pdf_tool", func() (contracts.Tool, error) { return nil, nil })
	ts := NewToolSearchTool(reg)
	result := ts.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"query":"nonexistent"}`,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatal(err)
	}
	activated := payload["activated"].([]any)
	if len(activated) != 0 {
		t.Errorf("expected 0 activated, got %d", len(activated))
	}
	if matched, ok := result.Metadata["matched"]; !ok || matched.(int) != 0 {
		t.Errorf("expected matched=0, got %v", result.Metadata["matched"])
	}
}

func TestToolSearchTool_Call_SubstringMatch(t *testing.T) {
	reg := registry.NewToolRegistry()
	activated := false
	_ = reg.RegisterDeferred("pdf_tool", func() (contracts.Tool, error) {
		activated = true
		return nil, nil
	})
	ts := NewToolSearchTool(reg)
	result := ts.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"query":"pdf"}`,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}
	if !activated {
		t.Error("deferred tool loader was not called")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatal(err)
	}
	activatedList := payload["activated"].([]any)
	if len(activatedList) != 1 || activatedList[0] != "pdf_tool" {
		t.Errorf("expected [pdf_tool], got %v", activatedList)
	}
	if count, ok := result.Metadata["activated_count"]; !ok || count.(int) != 1 {
		t.Errorf("expected activated_count=1, got %v", result.Metadata["activated_count"])
	}
}

func TestToolSearchTool_Call_ExactMatch(t *testing.T) {
	reg := registry.NewToolRegistry()
	_ = reg.RegisterDeferred("img_gen", func() (contracts.Tool, error) { return nil, nil })
	_ = reg.RegisterDeferred("img_gen_v2", func() (contracts.Tool, error) { return nil, nil })
	ts := NewToolSearchTool(reg)
	result := ts.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"query":"img_gen","exact":true}`,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatal(err)
	}
	activatedList := payload["activated"].([]any)
	if len(activatedList) != 1 || activatedList[0] != "img_gen" {
		t.Errorf("expected [img_gen], got %v", activatedList)
	}
	// img_gen_v2 should still be deferred
	still := payload["still_deferred"].([]any)
	stillStr := make([]string, len(still))
	for i, s := range still {
		stillStr[i] = s.(string)
	}
	if !contains(stillStr, "img_gen_v2") {
		t.Error("img_gen_v2 should still be deferred")
	}
}

func TestToolSearchTool_Call_CaseInsensitive(t *testing.T) {
	reg := registry.NewToolRegistry()
	_ = reg.RegisterDeferred("PDF_TOOL", func() (contracts.Tool, error) { return nil, nil })
	ts := NewToolSearchTool(reg)
	result := ts.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"query":"pdf"}`,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		t.Fatal(err)
	}
	activatedList := payload["activated"].([]any)
	if len(activatedList) != 1 {
		t.Errorf("expected 1 activated, got %d", len(activatedList))
	}
}

func TestToolSearchTool_Call_ActivatesAlreadyActive(t *testing.T) {
	// After activation, the tool is no longer deferred so re-search doesn't find it.
	reg := registry.NewToolRegistry()
	_ = reg.RegisterDeferred("tool_a", func() (contracts.Tool, error) { return nil, nil })
	ts := NewToolSearchTool(reg)

	result1 := ts.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"query":"tool_a"}`,
	})
	if result1.IsError {
		t.Fatalf("first activation failed: %+v", result1)
	}

	// Re-activation: tool_a is no longer in deferred list
	result2 := ts.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"query":"tool_a"}`,
	})
	if result2.IsError {
		t.Fatalf("unexpected error on re-activation: %+v", result2)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result2.Content), &payload); err != nil {
		t.Fatal(err)
	}
	activatedList := payload["activated"].([]any)
	if len(activatedList) != 0 {
		t.Errorf("expected 0 activated on re-search, got %v", activatedList)
	}
}

func TestMustMarshalJSON(t *testing.T) {
	// Valid struct
	s := mustMarshalJSON(map[string]string{"key": "value"})
	if !strings.Contains(s, "key") || !strings.Contains(s, "value") {
		t.Errorf("unexpected marshal output: %q", s)
	}

	// Invalid (unmarshallable) input falls back to "[]"
	ch := make(chan int)
	s2 := mustMarshalJSON(ch)
	if s2 != "[]" {
		t.Errorf("expected fallback [], got %q", s2)
	}
}

func contains(s []string, target string) bool {
	for _, item := range s {
		if item == target {
			return true
		}
	}
	return false
}
