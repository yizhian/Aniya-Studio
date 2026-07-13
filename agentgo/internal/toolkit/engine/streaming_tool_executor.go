package engine

import (
	"context"
	"fmt"

	"agentgo/internal/toolkit/contracts"
	"agentgo/internal/observability"
	"agentgo/internal/toolkit/registry"
)

const MaxToolUseConcurrency = 10

// StreamingToolExecutor 负责工具执行生命周期（查找、校验、权限、调用、后处理）。
type StreamingToolExecutor struct {
	Registry *registry.ToolRegistry
	Emitter  *observability.Emitter
}

// NewStreamingToolExecutor creates a new StreamingToolExecutor.
func NewStreamingToolExecutor(reg *registry.ToolRegistry) *StreamingToolExecutor {
	return &StreamingToolExecutor{Registry: reg}
}

func (e *StreamingToolExecutor) Execute(ctx context.Context, toolName string, args contracts.ToolCallArgs) contracts.ToolResult {
	if e.Registry == nil {
		observability.EmitOrLog(e.Emitter, observability.AgentEvent{
			Type: "tool:registry_nil",
			Data: map[string]any{"tool_name": toolName},
		})
		return contracts.ToolResult{IsError: true, ErrorCode: "registry_nil", ErrorMessage: "tool registry is nil"}
	}
	tool, err := e.Registry.Resolve(toolName)
	if err != nil {
		observability.EmitOrLog(e.Emitter, observability.AgentEvent{
			Type: "tool:not_found",
			Data: map[string]any{"tool_name": toolName, "error": err.Error()},
		})
		return contracts.ToolResult{IsError: true, ErrorCode: "tool_not_found", ErrorMessage: err.Error()}
	}
	if !args.CanUseTool {
		observability.EmitOrLog(e.Emitter, observability.AgentEvent{
			Type: "tool:forbidden",
			Data: map[string]any{"tool_name": toolName},
		})
		return contracts.ToolResult{IsError: true, ErrorCode: "tool_forbidden", ErrorMessage: "tool use is disabled in current context"}
	}

	desc := tool.Descriptor()
	// Fail-Closed：默认禁止危险能力，除非工具显式声明。
	if desc.MaxResultSizeChars <= 0 {
		desc.MaxResultSizeChars = 8000
	}

	result := tool.Call(ctx, args)
	if len(result.Content) > desc.MaxResultSizeChars {
		origLen := len(result.Content)
		result.Content = fmt.Sprintf("%s\n\n[truncated: content exceeds %d chars]", result.Content[:desc.MaxResultSizeChars], desc.MaxResultSizeChars)
		observability.EmitOrLog(e.Emitter, observability.AgentEvent{
			Type: "tool:truncated",
			Data: map[string]any{
				"tool_name": toolName,
				"orig_chars": origLen,
				"max_chars":  desc.MaxResultSizeChars,
			},
		})
	}
	return result
}
