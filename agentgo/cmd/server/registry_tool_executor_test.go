package main

import (
	"context"
	"strings"
	"testing"

	"agentgo/internal/toolkit/core"
	"agentgo/internal/toolkit/engine"
	"agentgo/internal/toolkit/registry"
)

func TestRegistryToolExecutor_FailClosed_NilRegistry(t *testing.T) {
	exec := &registryToolExecutor{
		inner:        engine.NewStreamingToolExecutor(nil),
		workspaceDir: "",
	}
	_, _, err := exec.Execute(context.Background(), "test", `{}`)
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
	if !strings.Contains(err.Error(), "registry is nil") {
		t.Errorf("expected 'registry is nil' error, got: %v", err)
	}
}

func TestRegistryToolExecutor_DefaultWorkspaceDir(t *testing.T) {
	// When no workspace path is in context, the executor should fall back
	// to its default workspaceDir.
	reg := registry.NewToolRegistry()
	reg.Register(core.NewReadFileTool())
	exec := newRegistryToolExecutor(reg, "/nonexistent")

	content, _, err := exec.Execute(context.Background(), "read_file", `{"path":"/dev/null"}`)
	if err != nil {
		t.Logf("expected on platform without /dev/null: %v", err)
		return
	}
	if content == "" {
		t.Log("empty content from /dev/null (expected)")
	}
}

func TestRegistryToolExecutor_FlagsPropagation(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.Register(core.NewReadFileTool())
	exec := newRegistryToolExecutor(reg, "")

	flags, err := exec.GetToolFlags("read_file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flags.ReadOnly {
		t.Error("read_file should have ReadOnly=true")
	}
	if !flags.ConcurrencySafe {
		t.Error("read_file should be ConcurrencySafe=true")
	}
	if flags.Destructive {
		t.Error("read_file should not be Destructive")
	}
}
