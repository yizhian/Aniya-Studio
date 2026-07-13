package provider

import (
	"encoding/json"

	"agentgo/internal/model"
)

// ---------------------------------------------------------------------------
// OpenAI wire types
// ---------------------------------------------------------------------------

type openAITool struct {
	Type     string          `json:"type"`
	Function openAIFunction  `json:"function"`
}

type openAIFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIChunkChoice struct {
	Delta struct {
		Role      string            `json:"role,omitempty"`
		Content   string            `json:"content,omitempty"`
		Reasoning string            `json:"reasoning_content,omitempty"`
		ToolCalls []openAIDeltaTool `json:"tool_calls,omitempty"`
	} `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type openAIDeltaTool struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

type openAIErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// Anthropic wire types
// ---------------------------------------------------------------------------

type anthropicMessage struct {
	Role    string              `json:"role"` // "user" | "assistant"
	Content []anthropicBlock    `json:"content"`
}

type anthropicBlock struct {
	Type  string `json:"type"` // "text" | "tool_use" | "tool_result" | "thinking"
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
	// For tool_result blocks
	ToolUseID string            `json:"tool_use_id,omitempty"`
	Content   string            `json:"content,omitempty"`
	IsError   bool              `json:"is_error,omitempty"`
	Thinking  string            `json:"thinking,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicContentBlockDelta struct {
	Type         string `json:"type"` // "text_delta" | "input_json_delta"
	Text         string `json:"text,omitempty"`
	PartialJSON  string `json:"partial_json,omitempty"`
	Thinking     string `json:"thinking,omitempty"`
}

type anthropicStreamEvent struct {
	Type string `json:"type"` // content_block_start, content_block_delta, content_block_stop, ...

	Index int `json:"index,omitempty"`

	// Present on content_block_start
	ContentBlock *anthropicBlock `json:"content_block,omitempty"`

	// Present on content_block_delta
	Delta *anthropicContentBlockDelta `json:"delta,omitempty"`

	// Present on message_start
	Message *struct {
		ID      string            `json:"id"`
		Model   string            `json:"model"`
		Role    string            `json:"role"`
		Content []anthropicBlock  `json:"content"`
		Usage   struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message,omitempty"`

	// Error on stream.
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Conversions: Internal → Wire
// ---------------------------------------------------------------------------

// toolsToOpenAI converts internal ToolDefinitions to the OpenAI wire format.
func toolsToOpenAI(tools []model.ToolDefinition) []openAITool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]openAITool, 0, len(tools))
	for _, t := range tools {
		params := t.Function.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  params,
			},
		})
	}
	return out
}

// toolsToAnthropic converts internal ToolDefinitions to the Anthropic wire format.
func toolsToAnthropic(tools []model.ToolDefinition) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		schema := t.Function.Parameters
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: schema,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Anthropic message conversion
// ---------------------------------------------------------------------------

func messagesToAnthropic(msgs []model.Message) []anthropicMessage {
	out := make([]anthropicMessage, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "system":
			// Anthropic passes system as a separate parameter, not in messages.
			// We'll handle it at the request level. Skip here.
			continue
		case "tool":
			out = append(out, toolResultToAnthropic(m))
		default:
			out = append(out, regularToAnthropic(m))
		}
	}
	return out
}

func regularToAnthropic(m model.Message) anthropicMessage {
	am := anthropicMessage{
		Role:    m.Role,
		Content: []anthropicBlock{},
	}
	if m.Content != "" {
		am.Content = append(am.Content, anthropicBlock{Type: "text", Text: m.Content})
	}
	if m.ReasoningContent != "" {
		am.Content = append(am.Content, anthropicBlock{Type: "thinking", Thinking: m.ReasoningContent})
	}
	for _, tc := range m.ToolCalls {
		am.Content = append(am.Content, anthropicBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}
	// Anthropic requires non-empty content for user/assistant roles.
	if len(am.Content) == 0 {
		am.Content = append(am.Content, anthropicBlock{Type: "text", Text: " "})
	}
	return am
}

func toolResultToAnthropic(m model.Message) anthropicMessage {
	return anthropicMessage{
		Role: "user",
		Content: []anthropicBlock{
			{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   m.Content,
			},
		},
	}
}
