package contracts

import (
	"context"

	"agentgo/internal/model"
)

// InputJSONSchema 对外暴露给模型 API 的 JSON Schema。
type InputJSONSchema = map[string]any

// ProgressEvent 工具执行中的流式进度。
type ProgressEvent struct {
	Stage   string         `json:"stage"`
	Message string         `json:"message,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

// ToolCallContext 工具调用上下文（会话级 + 调用级）。
type ToolCallContext struct {
	SessionID      string
	UserID         string
	WorkspacePath  string
	Round          int
	ConversationID string
	Metadata       map[string]any
	Provider       any // LLM provider for skill search; cast to provider.StreamingProvider as needed
}

// ToolResult 统一返回结构。错误也作为数据返回（is_error=true）。
type ToolResult struct {
	Content      string         `json:"content"`
	IsError      bool           `json:"is_error"`
	ErrorCode    string         `json:"error_code,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// ToolBehaviorFlags 安全与调度特性。
type ToolBehaviorFlags struct {
	ConcurrencySafe bool
	ReadOnly        bool
	Destructive     bool
	RequiresAuth    bool
	Deferred        bool
}

// ToolDescriptor 工具元数据 + 模型可见定义 + prompt 注入文案。
type ToolDescriptor struct {
	Name               string
	Aliases            []string
	Description        string
	Prompt             string
	MaxResultSizeChars int
	InputJSONSchema    InputJSONSchema
	Flags              ToolBehaviorFlags
}

// ToolCallArgs 统一入参。
type ToolCallArgs struct {
	ArgsJSON      string
	Context       ToolCallContext
	CanUseTool    bool
	ParentMessage model.Message
	OnProgress    func(ProgressEvent)
}

// Tool 统一行为契约（泛型思想在 Go 里通过解析器/校验器落地）。
type Tool interface {
	Descriptor() ToolDescriptor
	Call(ctx context.Context, args ToolCallArgs) ToolResult
}
