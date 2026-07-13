package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"agentgo/internal/model"
	"agentgo/internal/observability"
)

// OpenAIProvider implements StreamingProvider for OpenAI-compatible APIs
// (DeepSeek, OpenAI, etc.).
type OpenAIProvider struct {
	baseProvider
}

func NewOpenAIProvider(cfg Config) *OpenAIProvider {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}
	return &OpenAIProvider{
		baseProvider: baseProvider{
			apiKey:  cfg.APIKey,
			baseURL: strings.TrimRight(cfg.BaseURL, "/"),
			model:   cfg.Model,
			client:  &http.Client{Transport: transport},
			emitter: cfg.Emitter,
		},
	}
}

func (p *OpenAIProvider) Type() ProviderType { return ProviderOpenAI }

// providerAdapter methods
func (p *OpenAIProvider) buildBody(req ChatRequest, stream bool) map[string]any {
	messages := normalizeOpenAIMessages(req.Messages)
	body := map[string]any{
		"model":    p.model,
		"messages": messages,
		"stream":   stream,
	}
	if stream {
		body["stream_options"] = map[string]any{"include_usage": true}
	}
	if len(req.Tools) > 0 {
		body["tools"] = toolsToOpenAI(req.Tools)
		body["tool_choice"] = "auto"
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Thinking {
		body["thinking"] = map[string]string{"type": "enabled"}
	} else {
		body["thinking"] = map[string]string{"type": "disabled"}
	}
	return body
}

func (p *OpenAIProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

func (p *OpenAIProvider) setStreamHeaders(req *http.Request) {
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
}

func (p *OpenAIProvider) endpoint() string    { return "/v1/chat/completions" }
func (p *OpenAIProvider) providerName() string { return "openai" }

// ---------------------------------------------------------------------------
// Non-streaming Chat
// ---------------------------------------------------------------------------

func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return p.doChatRequest(ctx, req, p, func(r io.Reader) (*ChatResponse, error) {
		var wire struct {
			Choices []struct {
				Message      model.Message `json:"message"`
				FinishReason string        `json:"finish_reason"`
			} `json:"choices"`
			Usage *model.Usage `json:"usage,omitempty"`
		}
		if err := json.NewDecoder(r).Decode(&wire); err != nil {
			return nil, fmt.Errorf("openai decode: %w", err)
		}
		if len(wire.Choices) == 0 {
			return nil, fmt.Errorf("openai: empty choices")
		}

		msg := wire.Choices[0].Message
		if msg.Role == "" {
			msg.Role = "assistant"
		}

		result := &ChatResponse{
			Message:      msg,
			FinishReason: wire.Choices[0].FinishReason,
		}
		if wire.Usage != nil {
			result.Usage = *wire.Usage
		}
		return result, nil
	})
}

// ---------------------------------------------------------------------------
// Streaming Chat (SSE)
// ---------------------------------------------------------------------------

func (p *OpenAIProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	httpResp, err := p.doStreamRequest(ctx, req, p)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamEvent, 64)
	go p.readSSE(ctx, httpResp.Body, ch)
	return ch, nil
}

func (p *OpenAIProvider) readSSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamEvent) {
	reader := newSSEReader(ctx, body)
	defer reader.close()
	defer close(ch)

	accum := make(map[int]*toolCallAccumulator)
	var streamUsage *model.Usage

	flushToolCall := func(idx int) {
		ac := accum[idx]
		if ac == nil {
			return
		}
		ch <- StreamEvent{
			Type:         EventToolCallComplete,
			ToolCallID:   ac.ID,
			ToolCallName: ac.Name,
			ToolCall:     ac,
		}
		delete(accum, idx)
	}

	emitDone := func(finishReason string) {
		for idx := range accum {
			flushToolCall(idx)
		}
		ch <- StreamEvent{
			Type:         EventDone,
			FinishReason: finishReason,
			Usage:        streamUsage,
		}
	}

	parseErrors := 0
	for {
		data, done, eof := reader.readLine()
		if eof {
			if err := reader.scannerErr(); err != nil {
				ch <- StreamEvent{Type: EventError, Error: err}
				return
			}
			emitDone("")
			return
		}
		if done {
			emitDone("")
			return
		}

		var chunk struct {
			Choices []openAIChunkChoice `json:"choices"`
			Usage   *model.Usage        `json:"usage,omitempty"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			parseErrors++
			if parseErrors == 1 || parseErrors%5 == 0 {
				observability.EmitOrLog(p.emitter, observability.AgentEvent{
					Type: observability.EventProviderStreamErr,
					Data: map[string]any{
						"provider":      "openai",
						"error":         err.Error(),
						"skipped_count": parseErrors,
					},
				})
			}
			if parseErrors > 10 {
				ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("openai: too many parse errors (%d consecutive)", parseErrors)}
				return
			}
			continue
		}
		parseErrors = 0

		if chunk.Usage != nil {
			streamUsage = chunk.Usage
		}

		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		delta := choice.Delta

		if delta.Reasoning != "" {
			ch <- StreamEvent{Type: EventThinking, Delta: delta.Reasoning}
		}

		if delta.Content != "" {
			ch <- StreamEvent{Type: EventTextDelta, Delta: delta.Content}
		}

		for _, tc := range delta.ToolCalls {
			idx := tc.Index
			ac, exists := accum[idx]

			if !exists {
				ac = &toolCallAccumulator{ID: tc.ID, Name: tc.Function.Name}
				accum[idx] = ac
				ch <- StreamEvent{
					Type:          EventToolCallStart,
					ToolCallIndex: idx,
					ToolCallID:    tc.ID,
					ToolCallName:  tc.Function.Name,
				}
			}

			if tc.Function.Arguments != "" {
				ac.Arguments += tc.Function.Arguments
				ch <- StreamEvent{
					Type:          EventToolCallDelta,
					ToolCallIndex: idx,
					ToolCallName:  ac.Name,
					Delta:         tc.Function.Arguments,
				}
			}
		}

		if choice.FinishReason != nil {
			emitDone(*choice.FinishReason)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func normalizeOpenAIMessages(msgs []model.Message) []map[string]any {
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		item := map[string]any{
			"role":    m.Role,
			"content": m.Content,
		}
		if m.ReasoningContent != "" {
			item["reasoning_content"] = m.ReasoningContent
		}
		if m.Name != "" {
			item["name"] = m.Name
		}
		if len(m.ToolCalls) > 0 {
			item["tool_calls"] = m.ToolCalls
		}
		if m.ToolCallID != "" {
			item["tool_call_id"] = m.ToolCallID
		}
		out = append(out, item)
	}
	return out
}

func parseOpenAIError(r io.Reader) error {
	var apiErr openAIErrorBody
	if err := json.NewDecoder(r).Decode(&apiErr); err != nil {
		return fmt.Errorf("openai: HTTP error (body unparseable)")
	}
	return fmt.Errorf("openai: %s (type=%s code=%s)", apiErr.Error.Message, apiErr.Error.Type, apiErr.Error.Code)
}
