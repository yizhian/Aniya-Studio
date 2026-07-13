package bootstrap

import (
	"strings"
	"testing"

	"agentgo/internal/model"
	"agentgo/internal/toolkit/contracts"
	"agentgo/internal/toolkit/extended/skill"
	"agentgo/internal/toolkit/registry"
)

func TestRegisterAllTools(t *testing.T) {
	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}
	defs := r.GetActiveToolDefinitions()
	if len(defs) < 8 {
		t.Fatalf("expected at least 8 tool defs, got %d", len(defs))
	}
	var names []string
	for _, d := range defs {
		names = append(names, d.Function.Name)
	}
	if !contains(names, "read_file") || !contains(names, "write_file") || !contains(names, "edit_file") ||
		!contains(names, "grep_search") || !contains(names, "web_fetch") || !contains(names, "tool_search") {
		t.Fatalf("missing core defs: %v", names)
	}
	if !contains(names, "skill") {
		t.Fatalf("expected immediate skill tool: %v", names)
	}
	if !contains(names, "todo_write") {
		t.Fatalf("expected todo_write tool: %v", names)
	}

	// No deferred tools registered.
	deferred := r.GetDeferredToolNames()
	if len(deferred) != 0 {
		t.Fatalf("expected 0 deferred tools, got %v", deferred)
	}

	// Verify skill tool has real descriptor, not a stub.
	if !hasSkillToolDef(defs) {
		t.Fatalf("skill should have the real skill tool descriptor")
	}
}

func TestRegisterAllTools_DuplicateNameReturnsError(t *testing.T) {
	r := registry.NewToolRegistry()
	if err := r.RegisterDeferred("read_file", func() (contracts.Tool, error) {
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{}))
	if err == nil {
		t.Error("expected error when registering duplicate tool name")
	}
}

func TestRegisterAllTools_AllImmediate(t *testing.T) {
	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}
	deferred := r.GetDeferredToolNames()
	if len(deferred) != 0 {
		t.Errorf("expected 0 deferred tools, got %d: %v", len(deferred), deferred)
	}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func hasSkillToolDef(defs []model.ToolDefinition) bool {
	for _, d := range defs {
		if d.Function.Name != "skill" {
			continue
		}
		return strings.Contains(strings.ToLower(d.Function.Description), "query")
	}
	return false
}
