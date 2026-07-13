package model

// Message represents a message sent to the chat completion API (OpenAI-compatible format).
type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	Name             string     `json:"name,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a tool call fragment returned by the model.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the tool name and arguments JSON string.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolDefinition is the tool schema registered with the model.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}

// FunctionSpec describes a function tool.
type FunctionSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// Usage holds token usage for a request.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
