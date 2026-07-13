package engine

import (
	"context"
	"strings"
	"testing"

	"agentgo/internal/toolkit/contracts"
	"agentgo/internal/toolkit/registry"
)

// stubTool is a minimal tool for testing the executor.
type stubTool struct {
	name        string
	description string
	flags       contracts.ToolBehaviorFlags
	maxSize     int
	output      string
	errMsg      string
}

func (s *stubTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:               s.name,
		Description:        s.description,
		Flags:              s.flags,
		MaxResultSizeChars: s.maxSize,
	}
}

func (s *stubTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	if s.errMsg != "" {
		return contracts.ToolResult{IsError: true, ErrorCode: "stub_err", ErrorMessage: s.errMsg}
	}
	return contracts.ToolResult{Content: s.output}
}

func TestExecutor_Execute(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.Register(&stubTool{name: "echo", output: "hello world"})

	exec := &StreamingToolExecutor{Registry: reg}
	result := exec.Execute(context.Background(), "echo", contracts.ToolCallArgs{
		CanUseTool: true,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ErrorMessage)
	}
	if result.Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", result.Content)
	}
}

func TestExecutor_ToolNotFound(t *testing.T) {
	reg := registry.NewToolRegistry()
	exec := &StreamingToolExecutor{Registry: reg}

	result := exec.Execute(context.Background(), "nonexistent", contracts.ToolCallArgs{
		CanUseTool: true,
	})
	if !result.IsError {
		t.Fatal("expected error for unknown tool")
	}
	if result.ErrorCode != "tool_not_found" {
		t.Errorf("expected error_code='tool_not_found', got %q", result.ErrorCode)
	}
}

func TestExecutor_ToolForbidden(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.Register(&stubTool{name: "echo", output: "ok"})

	exec := &StreamingToolExecutor{Registry: reg}
	result := exec.Execute(context.Background(), "echo", contracts.ToolCallArgs{
		CanUseTool: false,
	})
	if !result.IsError {
		t.Fatal("expected error when tool use is forbidden")
	}
	if result.ErrorCode != "tool_forbidden" {
		t.Errorf("expected error_code='tool_forbidden', got %q", result.ErrorCode)
	}
}

func TestExecutor_NilRegistry(t *testing.T) {
	exec := &StreamingToolExecutor{Registry: nil}
	result := exec.Execute(context.Background(), "echo", contracts.ToolCallArgs{
		CanUseTool: true,
	})
	if !result.IsError {
		t.Fatal("expected error for nil registry")
	}
	if result.ErrorCode != "registry_nil" {
		t.Errorf("expected error_code='registry_nil', got %q", result.ErrorCode)
	}
}

func TestExecutor_ResultTruncation(t *testing.T) {
	reg := registry.NewToolRegistry()
	longOutput := strings.Repeat("x", 10000)
	reg.Register(&stubTool{
		name:    "big_output",
		output:  longOutput,
		maxSize: 100, // explicit truncation limit
	})

	exec := &StreamingToolExecutor{Registry: reg}
	result := exec.Execute(context.Background(), "big_output", contracts.ToolCallArgs{
		CanUseTool: true,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ErrorMessage)
	}
	if len(result.Content) >= len(longOutput) {
		t.Errorf("expected truncated output, but got %d chars (original %d)", len(result.Content), len(longOutput))
	}
	if !strings.Contains(result.Content, "[truncated") {
		t.Error("expected '[truncated]' marker in output")
	}
}

func TestExecutor_DefaultMaxSize(t *testing.T) {
	reg := registry.NewToolRegistry()
	longOutput := strings.Repeat("y", 10000)
	reg.Register(&stubTool{
		name:   "auto_truncate",
		output: longOutput,
		// maxSize=0 means default 8000.
	})

	exec := &StreamingToolExecutor{Registry: reg}
	result := exec.Execute(context.Background(), "auto_truncate", contracts.ToolCallArgs{
		CanUseTool: true,
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ErrorMessage)
	}
	if len(result.Content) >= 10000 {
		t.Errorf("expected truncation at default 8000, got %d chars", len(result.Content))
	}
}

func TestExecutor_ToolError(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.Register(&stubTool{name: "failer", errMsg: "something went wrong"})

	exec := &StreamingToolExecutor{Registry: reg}
	result := exec.Execute(context.Background(), "failer", contracts.ToolCallArgs{
		CanUseTool: true,
	})
	if !result.IsError {
		t.Fatal("expected error from tool")
	}
	if result.ErrorMessage != "something went wrong" {
		t.Errorf("expected 'something went wrong', got %q", result.ErrorMessage)
	}
}

func TestExecutor_ConcurrentSafeFlag(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.Register(&stubTool{
		name: "safe_tool",
		flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: true,
			ReadOnly:        true,
		},
	})

	flags, err := reg.GetToolFlags("safe_tool")
	if err != nil {
		t.Fatalf("GetToolFlags failed: %v", err)
	}
	if !flags.ConcurrencySafe {
		t.Error("expected ConcurrencySafe=true")
	}
	if !flags.ReadOnly {
		t.Error("expected ReadOnly=true")
	}
}

func TestExecutor_NonConcurrentFlag(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.Register(&stubTool{
		name: "unsafe_tool",
		flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: false,
			ReadOnly:        false,
		},
	})

	flags, err := reg.GetToolFlags("unsafe_tool")
	if err != nil {
		t.Fatalf("GetToolFlags failed: %v", err)
	}
	if flags.ConcurrencySafe {
		t.Error("expected ConcurrencySafe=false")
	}
	if flags.ReadOnly {
		t.Error("expected ReadOnly=false")
	}
}

func TestExecutor_ContextCancellation(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.Register(&stubTool{name: "echo", output: "ok"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	exec := &StreamingToolExecutor{Registry: reg}
	// The executor itself doesn't check context, but tools might.
	// This test verifies the executor doesn't panic with cancelled context.
	result := exec.Execute(ctx, "echo", contracts.ToolCallArgs{
		CanUseTool: true,
		Context: contracts.ToolCallContext{
			WorkspacePath: "/tmp",
		},
	})
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ErrorMessage)
	}
}
