package registry

import (
	"context"
	"sync"
	"testing"

	"agentgo/internal/toolkit/contracts"
)

// stubTool is a minimal tool implementation for testing.
type stubTool struct {
	name        string
	description string
	prompt      string
	flags       contracts.ToolBehaviorFlags
	aliases     []string
}

func (s *stubTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:        s.name,
		Description: s.description,
		Prompt:      s.prompt,
		Flags:       s.flags,
		Aliases:     s.aliases,
	}
}

func (s *stubTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	return contracts.ToolResult{Content: "ok"}
}

func TestRegistry_Register(t *testing.T) {
	r := NewToolRegistry()
	err := r.Register(&stubTool{name: "echo"})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	tool, err := r.Resolve("echo")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if tool.Descriptor().Name != "echo" {
		t.Errorf("expected name='echo', got %q", tool.Descriptor().Name)
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&stubTool{name: "echo"})
	err := r.Register(&stubTool{name: "echo"})
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegistry_RegisterWithAliases(t *testing.T) {
	r := NewToolRegistry()
	err := r.Register(&stubTool{name: "read", aliases: []string{"cat", "view"}})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// All aliases should resolve to the same tool.
	for _, name := range []string{"read", "cat", "view"} {
		tool, err := r.Resolve(name)
		if err != nil {
			t.Errorf("Resolve(%q) failed: %v", name, err)
			continue
		}
		if tool.Descriptor().Name != "read" {
			t.Errorf("Resolve(%q): expected name='read', got %q", name, tool.Descriptor().Name)
		}
	}
}

func TestRegistry_DuplicateAlias(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&stubTool{name: "tool1", aliases: []string{"alias1"}})
	err := r.Register(&stubTool{name: "tool2", aliases: []string{"alias1"}})
	if err == nil {
		t.Fatal("expected error for duplicate alias")
	}
}

func TestRegistry_EmptyName(t *testing.T) {
	r := NewToolRegistry()
	err := r.Register(&stubTool{name: ""})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestRegistry_ResolveUnknown(t *testing.T) {
	r := NewToolRegistry()
	_, err := r.Resolve("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestRegistry_DeferredTool(t *testing.T) {
	r := NewToolRegistry()
	loaded := false
	err := r.RegisterDeferred("agent", func() (contracts.Tool, error) {
		loaded = true
		return &stubTool{name: "agent", description: "Agent tool"}, nil
	})
	if err != nil {
		t.Fatalf("RegisterDeferred failed: %v", err)
	}

	// Resolve before activation should fail.
	_, err = r.Resolve("agent")
	if err == nil {
		t.Fatal("expected error for unactivated deferred tool")
	}

	// Activate.
	err = r.ActivateDeferred("agent")
	if err != nil {
		t.Fatalf("ActivateDeferred failed: %v", err)
	}
	if !loaded {
		t.Fatal("expected loader to be called")
	}
	if !r.IsDeferredActivated("agent") {
		t.Fatal("expected deferred tool to be activated")
	}

	// Now resolve should work.
	tool, err := r.Resolve("agent")
	if err != nil {
		t.Fatalf("Resolve after activation failed: %v", err)
	}
	if tool.Descriptor().Name != "agent" {
		t.Errorf("expected name='agent', got %q", tool.Descriptor().Name)
	}
}

func TestRegistry_ActivateDeferred_DoubleActivate(t *testing.T) {
	r := NewToolRegistry()
	loadCount := 0
	r.RegisterDeferred("agent", func() (contracts.Tool, error) {
		loadCount++
		return &stubTool{name: "agent"}, nil
	})

	r.ActivateDeferred("agent")
	r.ActivateDeferred("agent") // Should be no-op.

	if loadCount != 1 {
		t.Errorf("expected loader called once, got %d", loadCount)
	}
}

func TestRegistry_ActivateDeferred_NotDeferred(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&stubTool{name: "echo"})

	err := r.ActivateDeferred("echo")
	if err == nil {
		t.Fatal("expected error when activating non-deferred tool")
	}
}

func TestRegistry_ActivateDeferred_Unknown(t *testing.T) {
	r := NewToolRegistry()
	err := r.ActivateDeferred("nope")
	if err == nil {
		t.Fatal("expected error for unknown deferred tool")
	}
}

func TestRegistry_EmptyDeferredName(t *testing.T) {
	r := NewToolRegistry()
	err := r.RegisterDeferred("", func() (contracts.Tool, error) {
		return &stubTool{}, nil
	})
	if err == nil {
		t.Fatal("expected error for empty deferred name")
	}
}

func TestRegistry_NilLoader(t *testing.T) {
	r := NewToolRegistry()
	err := r.RegisterDeferred("tool", nil)
	if err == nil {
		t.Fatal("expected error for nil loader")
	}
}

func TestRegistry_GetDeferredToolNames(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&stubTool{name: "echo"})
	r.RegisterDeferred("agent", func() (contracts.Tool, error) { return &stubTool{name: "agent"}, nil })
	r.RegisterDeferred("plan_mode", func() (contracts.Tool, error) { return &stubTool{name: "plan_mode"}, nil })

	names := r.GetDeferredToolNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 deferred names, got %v", names)
	}

	// Activate one.
	r.ActivateDeferred("agent")
	names = r.GetDeferredToolNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 deferred name after activation, got %v", names)
	}
	if names[0] != "plan_mode" {
		t.Errorf("expected 'plan_mode' remaining, got %q", names[0])
	}
}

func TestRegistry_GetActiveToolDefinitions(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&stubTool{name: "echo", description: "Echo tool"})
	r.RegisterDeferred("agent", func() (contracts.Tool, error) {
		return &stubTool{name: "agent", description: "Agent tool"}, nil
	})

	defs := r.GetActiveToolDefinitions()
	// Should have: echo (immediate) + agent (deferred placeholder)
	if len(defs) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(defs))
	}

	// Find echo (full definition).
	foundEcho := false
	for _, d := range defs {
		if d.Function.Name == "echo" && d.Function.Description == "Echo tool" {
			foundEcho = true
		}
	}
	if !foundEcho {
		t.Error("expected full echo tool definition")
	}

	// Activate agent.
	r.ActivateDeferred("agent")
	defs = r.GetActiveToolDefinitions()
	foundAgent := false
	for _, d := range defs {
		if d.Function.Name == "agent" && d.Function.Description == "Agent tool" {
			foundAgent = true
		}
	}
	if !foundAgent {
		t.Error("expected full agent definition after activation")
	}
}

func TestRegistry_GetActiveToolPrompts(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&stubTool{name: "echo", description: "Echo", prompt: "Use echo to repeat text."})
	r.Register(&stubTool{name: "no_prompt_tool", description: "Silent"})

	prompts := r.GetActiveToolPrompts()
	if prompts == "" {
		t.Fatal("expected non-empty prompts")
	}

	// Deferred tools without activation should be skipped.
	r.RegisterDeferred("agent", func() (contracts.Tool, error) {
		return &stubTool{name: "agent", prompt: "Agent prompt"}, nil
	})

	prompts2 := r.GetActiveToolPrompts()
	// Same as before — deferred not yet activated.
	if prompts != prompts2 {
		t.Error("prompts should not include unactivated deferred tools")
	}

	// After activation, should include.
	r.ActivateDeferred("agent")
	prompts3 := r.GetActiveToolPrompts()
	if prompts3 == "" || prompts3 == prompts2 {
		t.Error("prompts should include activated deferred tool")
	}
}

func TestRegistry_GetToolFlags(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&stubTool{
		name: "read",
		flags: contracts.ToolBehaviorFlags{
			ReadOnly:        true,
			ConcurrencySafe: true,
		},
	})

	flags, err := r.GetToolFlags("read")
	if err != nil {
		t.Fatalf("GetToolFlags failed: %v", err)
	}
	if !flags.ReadOnly {
		t.Error("expected ReadOnly=true")
	}
	if !flags.ConcurrencySafe {
		t.Error("expected ConcurrencySafe=true")
	}

	_, err = r.GetToolFlags("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewToolRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "tool_" + string(rune('a'+idx))
			r.Register(&stubTool{name: name})
		}(i)
	}
	wg.Wait()

	// Concurrent resolutions.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "tool_" + string(rune('a'+idx))
			tool, err := r.Resolve(name)
			if err != nil {
				t.Errorf("concurrent resolve %s: %v", name, err)
				return
			}
			if tool.Descriptor().Name != name {
				t.Errorf("expected %s, got %s", name, tool.Descriptor().Name)
			}
		}(i)
	}
	wg.Wait()
}

func TestRegistry_IsDeferredActivated_NotDeferred(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&stubTool{name: "echo"})
	if r.IsDeferredActivated("echo") {
		t.Error("echo is not deferred, should return false")
	}
	if r.IsDeferredActivated("nonexistent") {
		t.Error("nonexistent should return false")
	}
}
