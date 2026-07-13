package main

import (
	"context"
	"fmt"

	"agentgo/internal/observability"
	"agentgo/internal/provider"
	"agentgo/internal/toolkit/contracts"
	"agentgo/internal/toolkit/engine"
	"agentgo/internal/toolkit/registry"
)

// registryToolExecutor adapts the toolkit registry to agent.ToolExecutor,
// with fail-closed, truncation, and permission enforcement via StreamingToolExecutor.
type registryToolExecutor struct {
	inner        *engine.StreamingToolExecutor
	workspaceDir string
}

func newRegistryToolExecutor(reg *registry.ToolRegistry, workspaceDir string) *registryToolExecutor {
	return &registryToolExecutor{
		inner:        engine.NewStreamingToolExecutor(reg),
		workspaceDir: workspaceDir,
	}
}

func (e *registryToolExecutor) Execute(ctx context.Context, name string, argumentsJSON string) (string, map[string]any, error) {
	wsPath, _ := ctx.Value(workspacePathCtxKey).(string)
	if wsPath == "" {
		wsPath = e.workspaceDir
	}
	prov, _ := ctx.Value(providerCtxKey).(provider.StreamingProvider)
	result := e.inner.Execute(ctx, name, contracts.ToolCallArgs{
		ArgsJSON:   argumentsJSON,
		CanUseTool: true,
		Context: contracts.ToolCallContext{
			WorkspacePath: wsPath,
			Provider:      prov,
		},
	})
	if result.IsError {
		return result.Content, result.Metadata, fmt.Errorf("%s", result.ErrorMessage)
	}
	return result.Content, result.Metadata, nil
}

// SetToolEmitter sets the observability emitter on the inner streaming executor.
func (e *registryToolExecutor) SetToolEmitter(emitter *observability.Emitter) {
	e.inner.Emitter = emitter
}

func (e *registryToolExecutor) GetToolFlags(name string) (contracts.ToolBehaviorFlags, error) {
	return e.inner.Registry.GetToolFlags(name)
}
