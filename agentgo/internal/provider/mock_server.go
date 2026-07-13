package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// MockSSEScript defines a sequence of responses the mock server should return.
// Each element represents one turn/round: the server streams the events, then
// closes the stream (signalling [DONE]). The next chat request gets the next
// script entry.
type MockSSEScript struct {
	Rounds [][]MockSSEFrame // one slice per round
}

// MockSSEFrame describes a single SSE frame to emit.
type MockSSEFrame struct {
	Type         string // "thinking", "text", "tool_call_start", "tool_call_complete", "done", "error", "usage"
	Delta        string // for thinking/text deltas
	ToolName     string // for tool_call_start / tool_call_complete
	ToolCallID   string // for tool call frames
	Index        int    // for tool_call_start
	Args         string // for tool_call_complete (JSON arguments string)
	ErrorMsg     string // for error frames
	StatusCode   int    // if set, return this HTTP status instead of streaming
	FinishReason string // for "done" frames, overrides default "stop". Anthropic: "end_turn"/"max_tokens". OpenAI: "stop"/"length"/"tool_calls".
	Usage        *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} // OpenAI: usage-only chunk before [DONE] when stream_options.include_usage is set
}

// MockProviderServer is an httptest server that speaks the OpenAI-compatible
// SSE protocol. It executes a pre-scripted sequence of responses.
// Set AnthropicFormat to true for Anthropic SSE output format.
type MockProviderServer struct {
	*httptest.Server

	mu              sync.Mutex
	script          MockSSEScript
	roundN          int
	requests        []MockRequest // captured incoming requests for assertion
	AnthropicFormat bool

	// Optional: custom handler for validating requests per-test.
	OnRequest func(body map[string]any)
}

type MockRequest struct {
	Body map[string]any
}

// NewMockProviderServer creates a started mock server from a script.
func NewMockProviderServer(script MockSSEScript) *MockProviderServer {
	m := &MockProviderServer{script: script}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	return m
}

// Requests returns all captured incoming request bodies.
func (m *MockProviderServer) Requests() []MockRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MockRequest, len(m.requests))
	copy(out, m.requests)
	return out
}

// RequestCount returns the number of requests received.
func (m *MockProviderServer) RequestCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests)
}

func (m *MockProviderServer) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.roundN++
	m.mu.Unlock()

	// Capture request body.
	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)
	m.mu.Lock()
	m.requests = append(m.requests, MockRequest{Body: body})
	m.mu.Unlock()

	if m.OnRequest != nil {
		m.OnRequest(body)
	}

	roundIdx := m.roundN - 1
	if roundIdx >= len(m.script.Rounds) {
		m.emitFallback(w)
		return
	}

	frames := m.script.Rounds[roundIdx]

	// Check for HTTP error injection (non-200 status).
	for _, f := range frames {
		if f.StatusCode > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.StatusCode)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": f.ErrorMsg,
					"type":    "api_error",
				},
			})
			return
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	if m.AnthropicFormat {
		m.emitAnthropicFrames(w, flusher, frames)
	} else {
		m.emitOpenAIFrames(w, flusher, frames)
	}
}

func (m *MockProviderServer) emitFallback(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)

	if m.AnthropicFormat {
		fmt.Fprintf(w, "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"Done.\"}}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: {\"type\":\"message_stop\"}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		return
	}

	fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Done.\"},\"finish_reason\":\"stop\"}]}\n\n")
	if flusher != nil {
		flusher.Flush()
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func (m *MockProviderServer) emitOpenAIFrames(w http.ResponseWriter, flusher http.Flusher, frames []MockSSEFrame) {
	for _, f := range frames {
		switch f.Type {
		case "thinking":
			chunk := map[string]any{
				"choices": []map[string]any{{
					"delta": map[string]any{
						"reasoning_content": f.Delta,
					},
				}},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case "text":
			chunk := map[string]any{
				"choices": []map[string]any{{
					"delta": map[string]any{
						"content": f.Delta,
					},
				}},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case "tool_call_start":
			chunk := map[string]any{
				"choices": []map[string]any{{
					"delta": map[string]any{
						"tool_calls": []map[string]any{{
							"index": f.Index,
							"id":    f.ToolCallID,
							"type":  "function",
							"function": map[string]any{
								"name":      f.ToolName,
								"arguments": "",
							},
						}},
					},
				}},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case "tool_call_delta":
			chunk := map[string]any{
				"choices": []map[string]any{{
					"delta": map[string]any{
						"tool_calls": []map[string]any{{
							"index": f.Index,
							"function": map[string]any{
								"arguments": f.Delta,
							},
						}},
					},
				}},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case "tool_call_complete":
			if f.Args != "" {
				chunk := map[string]any{
					"choices": []map[string]any{{
						"delta": map[string]any{
							"tool_calls": []map[string]any{{
								"index": f.Index,
								"function": map[string]any{
									"arguments": f.Args,
								},
							}},
						},
					}},
				}
				data, _ := json.Marshal(chunk)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}

		case "usage":
			// Emit a usage-only chunk (choices empty, usage present).
			// Real OpenAI sends this before [DONE] when stream_options.include_usage is set.
			chunk := map[string]any{
				"choices": []map[string]any{},
			}
			if f.Usage != nil {
				chunk["usage"] = map[string]any{
					"prompt_tokens":     f.Usage.PromptTokens,
					"completion_tokens": f.Usage.CompletionTokens,
					"total_tokens":      f.Usage.TotalTokens,
				}
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case "done":
			reason := f.FinishReason
			if reason == "" {
				reason = "stop"
			}
			chunk := map[string]any{
				"choices": []map[string]any{{
					"finish_reason": reason,
				}},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case "error":
			chunk := map[string]any{
				"error": map[string]any{
					"message": f.ErrorMsg,
					"type":    "server_error",
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}

	// Always end with [DONE].
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func (m *MockProviderServer) emitAnthropicFrames(w http.ResponseWriter, flusher http.Flusher, frames []MockSSEFrame) {
	for _, f := range frames {
		switch f.Type {
		case "thinking":
			ev := map[string]any{
				"type": "content_block_start", "index": f.Index,
				"content_block": map[string]any{"type": "thinking", "thinking": f.Delta},
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			stop := map[string]any{"type": "content_block_stop", "index": f.Index}
			sd, _ := json.Marshal(stop)
			fmt.Fprintf(w, "data: %s\n\n", sd)
			flusher.Flush()

		case "text":
			start := map[string]any{
				"type": "content_block_start", "index": f.Index,
				"content_block": map[string]any{"type": "text", "text": f.Delta},
			}
			sd, _ := json.Marshal(start)
			fmt.Fprintf(w, "data: %s\n\n", sd)
			flusher.Flush()

			stop := map[string]any{"type": "content_block_stop", "index": f.Index}
			sd2, _ := json.Marshal(stop)
			fmt.Fprintf(w, "data: %s\n\n", sd2)
			flusher.Flush()

		case "tool_call_start":
			var input any
			if f.Args != "" {
				json.Unmarshal([]byte(f.Args), &input)
			}
			cb := map[string]any{
				"type": "tool_use",
				"id":   f.ToolCallID,
				"name": f.ToolName,
			}
			if input != nil {
				cb["input"] = input
			}
			start := map[string]any{
				"type": "content_block_start", "index": f.Index,
				"content_block": cb,
			}
			sd, _ := json.Marshal(start)
			fmt.Fprintf(w, "data: %s\n\n", sd)
			flusher.Flush()

		case "tool_call_complete":
			if f.Args != "" {
				delta := map[string]any{
					"type": "content_block_delta", "index": f.Index,
					"delta": map[string]any{"type": "input_json_delta", "partial_json": f.Args},
				}
				sd, _ := json.Marshal(delta)
				fmt.Fprintf(w, "data: %s\n\n", sd)
				flusher.Flush()
			}
			stop := map[string]any{"type": "content_block_stop", "index": f.Index}
			sd2, _ := json.Marshal(stop)
			fmt.Fprintf(w, "data: %s\n\n", sd2)
			flusher.Flush()

		case "done":
			// Only emit message_delta when finish_reason or usage is explicitly set.
			// Otherwise just message_stop (backward compatible with existing tests).
			if f.FinishReason != "" || f.Usage != nil {
				reason := f.FinishReason
				if reason == "" {
					reason = "end_turn"
				}
				outputTokens := 0
				if f.Usage != nil {
					outputTokens = f.Usage.CompletionTokens
				}
				msgDelta := map[string]any{
					"type": "message_delta",
					"delta": map[string]any{
						"stop_reason": reason,
					},
					"usage": map[string]any{
						"output_tokens": outputTokens,
					},
				}
				sd, _ := json.Marshal(msgDelta)
				fmt.Fprintf(w, "data: %s\n\n", sd)
				flusher.Flush()
			}
			fmt.Fprintf(w, "data: {\"type\":\"message_stop\"}\n\n")
			flusher.Flush()

		case "error":
			ev := map[string]any{
				"type": "error",
				"error": map[string]any{
					"type":    "api_error",
					"message": f.ErrorMsg,
				},
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// Script helpers for building test scripts concisely.

// TextFrame creates a text delta frame.
func TextFrame(text string) MockSSEFrame {
	return MockSSEFrame{Type: "text", Delta: text}
}

// ThinkingFrame creates a thinking delta frame.
func ThinkingFrame(text string) MockSSEFrame {
	return MockSSEFrame{Type: "thinking", Delta: text}
}

// ToolStartFrame creates a tool_call_start frame.
func ToolStartFrame(name, id string, index int) MockSSEFrame {
	return MockSSEFrame{Type: "tool_call_start", ToolName: name, ToolCallID: id, Index: index}
}

// ToolDeltaFrame creates a tool_call_delta frame (streaming argument chunk).
func ToolDeltaFrame(argsDelta string, index int) MockSSEFrame {
	return MockSSEFrame{Type: "tool_call_delta", Delta: argsDelta, Index: index}
}

// ToolCompleteFrame creates a tool_call_complete frame.
func ToolCompleteFrame(name, id, args string, index int) MockSSEFrame {
	return MockSSEFrame{Type: "tool_call_complete", ToolName: name, ToolCallID: id, Index: index, Args: args}
}

// DoneFrame creates a done frame with default finish_reason ("stop" for OpenAI, "end_turn" for Anthropic when usage is set).
func DoneFrame() MockSSEFrame {
	return MockSSEFrame{Type: "done"}
}

// DoneFrameWithReason creates a done frame with a custom finish_reason (e.g. "length", "max_tokens").
func DoneFrameWithReason(reason string) MockSSEFrame {
	return MockSSEFrame{Type: "done", FinishReason: reason}
}

// DoneFrameWithUsage creates a done frame with custom finish_reason and usage data.
func DoneFrameWithUsage(reason string, promptTokens, completionTokens int) MockSSEFrame {
	return MockSSEFrame{
		Type:         "done",
		FinishReason: reason,
		Usage: &struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}
}

// UsageFrame creates a usage-only chunk (OpenAI: choices=[] + usage, sent before [DONE]
// when stream_options.include_usage is set).
func UsageFrame(promptTokens, completionTokens int) MockSSEFrame {
	return MockSSEFrame{
		Type: "usage",
		Usage: &struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}
}

// ErrorFrame creates an error frame.
func ErrorFrame(msg string) MockSSEFrame {
	return MockSSEFrame{Type: "error", ErrorMsg: msg}
}

// HTTPErrorFrame creates a non-200 HTTP response (not an SSE frame).
func HTTPErrorFrame(code int, msg string) MockSSEFrame {
	return MockSSEFrame{StatusCode: code, ErrorMsg: msg}
}

// SingleTurnScript creates a script with one round of frames.
func SingleTurnScript(frames ...MockSSEFrame) MockSSEScript {
	allFrames := make([]MockSSEFrame, len(frames))
	copy(allFrames, frames)
	return MockSSEScript{Rounds: [][]MockSSEFrame{allFrames}}
}

// Ensure provider.Config can be constructed from mock server URL.
func configFromMockServer(server *MockProviderServer, model string) Config {
	return Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   model,
		Type:    ProviderOpenAI,
	}
}

// RequireMessageRole asserts that the request contains a message with the given role.
func RequireMessageRole(body map[string]any, role string) (map[string]any, bool) {
	messages, ok := body["messages"].([]any)
	if !ok {
		return nil, false
	}
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if r, _ := msg["role"].(string); r == role {
			return msg, true
		}
	}
	return nil, false
}

// RequireToolInRequest asserts that the request includes a tool with the given name.
func RequireToolInRequest(body map[string]any, toolName string) bool {
	tools, ok := body["tools"].([]any)
	if !ok {
		return false
	}
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok {
			continue
		}
		fn, ok := tool["function"].(map[string]any)
		if !ok {
			continue
		}
		if name, _ := fn["name"].(string); name == toolName {
			return true
		}
	}
	return false
}

// RequireToolResultInMessages checks that messages include a tool result with the given content.
func RequireToolResultInMessages(body map[string]any, contentSubstr string) bool {
	messages, ok := body["messages"].([]any)
	if !ok {
		return false
	}
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role == "tool" {
			if c, _ := msg["content"].(string); strings.Contains(c, contentSubstr) {
				return true
			}
		}
	}
	return false
}

// MessageCount returns the number of messages in the request.
func MessageCount(body map[string]any) int {
	messages, ok := body["messages"].([]any)
	if !ok {
		return 0
	}
	return len(messages)
}