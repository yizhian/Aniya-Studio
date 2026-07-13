// Package provider abstracts over OpenAI-compatible and Anthropic API backends.
//
// DeepSeek supports both formats:
//
//	OpenAI:    POST https://api.deepseek.com/v1/chat/completions
//	Anthropic: POST https://api.deepseek.com/anthropic/v1/messages
package provider

import (
	"context"

	"agentgo/internal/model"
	"agentgo/internal/observability"
)

// ProviderType identifies the API wire format.
type ProviderType string

const (
	ProviderOpenAI    ProviderType = "openai"
	ProviderAnthropic ProviderType = "anthropic"
)

// Config holds provider-level connection settings.
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Type    ProviderType
	// Emitter is an optional observability emitter for provider-level events.
	// When nil, events fall back to structured JSON via log.Print.
	Emitter *observability.Emitter
}

// ChatRequest is a provider-agnostic request, modelled on the common subset
// of OpenAI Chat Completions and Anthropic Messages.
type ChatRequest struct {
	Model     string
	Messages  []model.Message
	Tools     []model.ToolDefinition
	MaxTokens int
	Stream    bool
	// Thinking enables reasoning tokens (supported by DeepSeek and Anthropic).
	Thinking bool
}

// StreamingProvider is the interface each provider backend must implement.
type StreamingProvider interface {
	// Chat performs a non-streaming round-trip. Used by the legacy agent loop.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// StreamChat opens an SSE connection and returns a channel of events.
	// The caller must drain the channel until EventDone or EventError.
	StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
	// Type returns the provider's wire format.
	Type() ProviderType
}

// ChatResponse is a provider-agnostic non-streaming response.
type ChatResponse struct {
	Message    model.Message
	Usage      model.Usage
	FinishReason string
}

// New creates a StreamingProvider based on the config type.
// This is the single factory entry point for all providers.
func New(cfg Config) (StreamingProvider, error) {
	switch cfg.Type {
	case ProviderAnthropic:
		return NewAnthropicProvider(cfg), nil
	default:
		return NewOpenAIProvider(cfg), nil
	}
}
