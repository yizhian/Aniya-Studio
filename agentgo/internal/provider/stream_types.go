package provider

import "agentgo/internal/model"

// StreamEventType classifies each event emitted during SSE streaming.
type StreamEventType int

const (
	EventThinking    StreamEventType = iota // reasoning / thinking content delta
	EventTextDelta                          // incremental output text
	EventToolCallStart                      // new tool call detected (name known, arguments begin streaming)
	EventToolCallDelta                      // incremental tool-call arguments (partial JSON)
	EventToolCallComplete                   // all arguments received for one tool call
	EventError                              // stream-level error (not tool-execution error)
	EventDone                               // stream finished normally
)

// toolCallAccumulator holds partial state for one tool call during streaming.
type toolCallAccumulator struct {
	ID        string
	Name      string
	Arguments string // accumulated partial JSON
}

// NewToolCallCompleteEvent creates a StreamEvent of type EventToolCallComplete
// with the given tool call arguments. Exported for use by test packages.
func NewToolCallCompleteEvent(name, id, arguments string) StreamEvent {
	return StreamEvent{
		Type:         EventToolCallComplete,
		ToolCallName: name,
		ToolCallID:   id,
		ToolCall: &toolCallAccumulator{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		},
	}
}

// StreamEvent is yielded one-at-a-time by the SSE parser.
type StreamEvent struct {
	Type  StreamEventType
	Delta string // text/thinking content for Event*Delta types

	// ToolCall fields are populated for EventToolCall* events.
	ToolCallIndex int
	ToolCallID    string
	ToolCallName  string

	// ToolCall is populated only on EventToolCallComplete.
	ToolCall *toolCallAccumulator

	Error error // populated for EventError

	// FinishReason from the API. Populated on EventDone.
	// "stop", "length", "max_tokens", "tool_calls", "end_turn", "" (ambiguous EOF).
	FinishReason string
	// Usage from the API's final chunk. Reuses model.Usage directly.
	Usage *model.Usage
}
