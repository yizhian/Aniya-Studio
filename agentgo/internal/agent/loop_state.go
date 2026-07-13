package agent

import (
	"context"
	"time"

	"agentgo/internal/model"
)

// TransitionReason 当前实现：仅根据本轮是否发生工具调用区分。
const (
	TransitionModelRequestedTools = "model_requested_tools"
	TransitionModelCompleted      = "model_completed_no_tools"
)

// ToolResultBlock 从工具执行结果中提取的结构化记录（对应 API 中的 tool 消息）。
type ToolResultBlock struct {
	Type      string `json:"type"`        // 固定为 "tool_result"
	ToolUseID string `json:"tool_use_id"` // 对应 tool_calls[].id
	Content   string `json:"content"`
}

// UsageTotals 累计 token。
type UsageTotals struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LoopState 显性记录：消息历史、轮次、过渡原因、工具结果块、累计用量、用户可见时间线。
type LoopState struct {
	Version          int               `json:"version"`
	Messages         []model.Message   `json:"messages"`
	Round            int               `json:"round"`
	TransitionReason string            `json:"transition_reason"`
	ToolResultBlocks []ToolResultBlock `json:"tool_result_blocks"`
	CumulativeUsage  UsageTotals       `json:"cumulative_usage"`
	Timeline         []TimelineEvent   `json:"timeline"`

	// Recovery tracks error recovery state within a RunStreaming call.
	// Single source of truth — SessionState does NOT duplicate recovery counters.
	Recovery RecoveryState `json:"recovery"`


}

// TimelineEvent is a user-facing event in the conversation, stored in
// chronological order so the frontend can replay the full conversation
// without reconstructing it from internal LLM messages.
type TimelineEvent struct {
	Event     string         `json:"event"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

func (u *UsageTotals) add(o model.Usage) {
	u.PromptTokens += o.PromptTokens
	u.CompletionTokens += o.CompletionTokens
	u.TotalTokens += o.TotalTokens
}

// ToolExecutor executes a tool given its name and raw JSON arguments.
type ToolExecutor interface {
	Execute(ctx context.Context, name string, argumentsJSON string) (content string, metadata map[string]any, err error)
}
