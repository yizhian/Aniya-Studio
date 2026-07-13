package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"agentgo/internal/model"
	"agentgo/internal/observability"
)

const anthropicVersion = "2023-06-01"

// AnthropicProvider implements StreamingProvider for the Anthropic Messages API
// (including DeepSeek's Anthropic-compatible endpoint).
type AnthropicProvider struct {
	baseProvider
}

func NewAnthropicProvider(cfg Config) *AnthropicProvider {
	return &AnthropicProvider{
		baseProvider: baseProvider{
			apiKey:  cfg.APIKey,
			baseURL: strings.TrimRight(cfg.BaseURL, "/"),
			model:   cfg.Model,
			client:  &http.Client{},
			emitter: cfg.Emitter,
		},
	}
}

func (p *AnthropicProvider) Type() ProviderType { return ProviderAnthropic }

// providerAdapter methods
func (p *AnthropicProvider) buildBody(req ChatRequest, stream bool) map[string]any {
	var system string
	var msgs []model.Message
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			msgs = append(msgs, m)
		}
	}

	body := map[string]any{
		"model":      p.model,
		"messages":   messagesToAnthropic(msgs),
		"max_tokens": 32768,
		"stream":     stream,
	}
	if system != "" {
		body["system"] = system
	}
	if len(req.Tools) > 0 {
		body["tools"] = toolsToAnthropic(req.Tools)
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Thinking {
		body["thinking"] = map[string]any{"type": "enabled", "budget_tokens": 16000}
	}
	return body
}

func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
}

func (p *AnthropicProvider) endpoint() string             { return "/v1/messages" }
func (p *AnthropicProvider) providerName() string          { return "anthropic" }
func (p *AnthropicProvider) setStreamHeaders(_ *http.Request) {}

// ---------------------------------------------------------------------------
// Non-streaming Messages
// ---------------------------------------------------------------------------

func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return p.doChatRequest(ctx, req, p, func(r io.Reader) (*ChatResponse, error) {
		var wire struct {
			Content    []anthropicBlock `json:"content"`
			StopReason string           `json:"stop_reason"`
			Usage      struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.NewDecoder(r).Decode(&wire); err != nil {
			return nil, fmt.Errorf("anthropic decode: %w", err)
		}

		msg := p.contentToMessage(wire.Content)
		msg.Role = "assistant"

		return &ChatResponse{
			Message:      msg,
			FinishReason: wire.StopReason,
			Usage: model.Usage{
				PromptTokens:     wire.Usage.InputTokens,
				CompletionTokens: wire.Usage.OutputTokens,
				TotalTokens:      wire.Usage.InputTokens + wire.Usage.OutputTokens,
			},
		}, nil
	})
}

// ---------------------------------------------------------------------------
// Streaming Chat (SSE)
// ---------------------------------------------------------------------------

func (p *AnthropicProvider) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	httpResp, err := p.doStreamRequest(ctx, req, p)
	if err != nil {
		return nil, err
	}

	ch := make(chan StreamEvent, 64)
	go p.readSSE(ctx, httpResp.Body, ch)
	return ch, nil
}

func (p *AnthropicProvider) readSSE(ctx context.Context, body io.ReadCloser, ch chan<- StreamEvent) {
	reader := newSSEReader(ctx, body)
	defer reader.close()
	defer close(ch)

	accum := make(map[int]*toolCallAccumulator)
	var stopReason string
	var outputTokens int

	parseErrors := 0
	for {
		data, done, eof := reader.readLine()
		if eof {
			if err := reader.scannerErr(); err != nil {
				ch <- StreamEvent{Type: EventError, Error: err}
				return
			}
			ch <- StreamEvent{Type: EventDone}
			return
		}
		if done {
			ch <- StreamEvent{Type: EventDone}
			return
		}

		var ev anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			parseErrors++
			if parseErrors == 1 || parseErrors%5 == 0 {
				observability.EmitOrLog(p.emitter, observability.AgentEvent{
					Type: observability.EventProviderStreamErr,
					Data: map[string]any{
						"provider":      "anthropic",
						"error":         err.Error(),
						"skipped_count": parseErrors,
					},
				})
			}
			if parseErrors > 10 {
				ch <- StreamEvent{Type: EventError, Error: fmt.Errorf("anthropic: too many parse errors (%d consecutive)", parseErrors)}
				return
			}
			continue
		}
		parseErrors = 0

		if ev.Type == "error" && ev.Error != nil {
			ch <- StreamEvent{
				Type:  EventError,
				Error: fmt.Errorf("anthropic: %s: %s", ev.Error.Type, ev.Error.Message),
			}
			return
		}

		switch ev.Type {
		case "content_block_start":
			if ev.ContentBlock == nil {
				continue
			}
			idx := ev.Index
			switch ev.ContentBlock.Type {
			case "text":
				ch <- StreamEvent{Type: EventTextDelta, Delta: ev.ContentBlock.Text}
			case "thinking":
				ch <- StreamEvent{Type: EventThinking, Delta: ev.ContentBlock.Thinking}
			case "tool_use":
				accum[idx] = &toolCallAccumulator{
					ID:   ev.ContentBlock.ID,
					Name: ev.ContentBlock.Name,
				}
				ch <- StreamEvent{
					Type:          EventToolCallStart,
					ToolCallIndex: idx,
					ToolCallID:    ev.ContentBlock.ID,
					ToolCallName:  ev.ContentBlock.Name,
				}
				if ev.ContentBlock.Input != nil {
					inputBytes, _ := json.Marshal(ev.ContentBlock.Input)
					inputStr := string(inputBytes)
					accum[idx].Arguments = inputStr
					ch <- StreamEvent{
						Type:          EventToolCallDelta,
						ToolCallIndex: idx,
						ToolCallName:  accum[idx].Name,
						Delta:         inputStr,
					}
				}
			}

		case "content_block_delta":
			if ev.Delta == nil {
				continue
			}
			idx := ev.Index
			switch ev.Delta.Type {
			case "text_delta":
				ch <- StreamEvent{Type: EventTextDelta, Delta: ev.Delta.Text}
			case "input_json_delta":
				if ac, ok := accum[idx]; ok {
					ac.Arguments += ev.Delta.PartialJSON
					ch <- StreamEvent{
						Type:          EventToolCallDelta,
						ToolCallIndex: idx,
						ToolCallName:  ac.Name,
						Delta:         ev.Delta.PartialJSON,
					}
				}
			case "thinking_delta":
				ch <- StreamEvent{Type: EventThinking, Delta: ev.Delta.Thinking}
			}

		case "content_block_stop":
			idx := ev.Index
			if ac, ok := accum[idx]; ok {
				ch <- StreamEvent{
					Type:         EventToolCallComplete,
					ToolCallID:   ac.ID,
					ToolCallName: ac.Name,
					ToolCall:     ac,
				}
				delete(accum, idx)
			}

		case "message_delta":
			var md struct {
				Delta struct {
					StopReason   string  `json:"stop_reason"`
					StopSequence *string `json:"stop_sequence"`
				} `json:"delta"`
				Usage struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &md); err == nil {
				stopReason = md.Delta.StopReason
				outputTokens = md.Usage.OutputTokens
			}

		case "message_stop":
			ch <- StreamEvent{
				Type:         EventDone,
				FinishReason: stopReason,
				Usage:        &model.Usage{CompletionTokens: outputTokens},
			}
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (p *AnthropicProvider) contentToMessage(blocks []anthropicBlock) model.Message {
	var msg model.Message
	for _, b := range blocks {
		switch b.Type {
		case "text":
			msg.Content += b.Text
		case "thinking":
			msg.ReasoningContent += b.Thinking
		case "tool_use":
			inputBytes, _ := json.Marshal(b.Input)
			msg.ToolCalls = append(msg.ToolCalls, model.ToolCall{
				ID:   b.ID,
				Type: "function",
				Function: model.ToolCallFunction{
					Name:      b.Name,
					Arguments: string(inputBytes),
				},
			})
		}
	}
	return msg
}

func parseAnthropicError(r io.Reader) error {
	var wire struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(r).Decode(&wire); err != nil {
		return fmt.Errorf("anthropic: HTTP error (body unparseable)")
	}
	return fmt.Errorf("anthropic: %s: %s", wire.Error.Type, wire.Error.Message)
}
