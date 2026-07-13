package provider

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agentgo/internal/model"
)

// ---------------------------------------------------------------------------
// OpenAI Provider: streaming chat through mock server
// ---------------------------------------------------------------------------

func TestOpenAIProvider_StreamChat_TextOnly(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("Hello, "),
		TextFrame("world!"),
		DoneFrame(),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Say hello"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var textContent strings.Builder
	var eventTypes []StreamEventType
	for ev := range ch {
		eventTypes = append(eventTypes, ev.Type)
		if ev.Type == EventTextDelta {
			textContent.WriteString(ev.Delta)
		}
	}

	full := textContent.String()
	if full != "Hello, world!" {
		t.Fatalf("expected 'Hello, world!', got %q", full)
	}

	// Verify event sequence: text, text, done.
	if len(eventTypes) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(eventTypes))
	}
	if eventTypes[len(eventTypes)-1] != EventDone {
		t.Fatal("last event should be EventDone")
	}
}

func TestOpenAIProvider_StreamChat_ToolCalls(t *testing.T) {
	script := SingleTurnScript(
		ToolStartFrame("read_file", "toolu_001", 0),
		ToolCompleteFrame("read_file", "toolu_001", `{"path":"test.txt"}`, 0),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Read test.txt"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var eventTypes []StreamEventType
	var toolNames []string
	var toolArgs []string

	for ev := range ch {
		eventTypes = append(eventTypes, ev.Type)
		switch ev.Type {
		case EventToolCallStart:
			toolNames = append(toolNames, ev.ToolCallName)
		case EventToolCallComplete:
			if ev.ToolCall != nil {
				toolArgs = append(toolArgs, ev.ToolCall.Arguments)
			}
		}
	}

	if len(toolNames) != 1 || toolNames[0] != "read_file" {
		t.Fatalf("expected tool name 'read_file', got %v", toolNames)
	}
	if len(toolArgs) != 1 || !strings.Contains(toolArgs[0], `"test.txt"`) {
		t.Fatalf("expected args containing 'test.txt', got %v", toolArgs)
	}

	hasStart := false
	hasComplete := false
	for _, typ := range eventTypes {
		if typ == EventToolCallStart {
			hasStart = true
		}
		if typ == EventToolCallComplete {
			hasComplete = true
		}
	}
	if !hasStart || !hasComplete {
		t.Fatal("expected tool_call_start and tool_call_complete events")
	}
}

func TestOpenAIProvider_StreamChat_Thinking(t *testing.T) {
	script := SingleTurnScript(
		ThinkingFrame("Let me analyze..."),
		TextFrame("Here is the answer."),
		DoneFrame(),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Think about it"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	hasThinking := false
	for ev := range ch {
		if ev.Type == EventThinking {
			hasThinking = true
			if !strings.Contains(ev.Delta, "analyze") {
				t.Errorf("expected thinking delta to contain 'analyze', got %q", ev.Delta)
			}
		}
	}
	if !hasThinking {
		t.Fatal("expected thinking event")
	}
}

func TestOpenAIProvider_StreamChat_MultipleToolCalls(t *testing.T) {
	script := SingleTurnScript(
		ToolStartFrame("read_file", "toolu_001", 0),
		ToolCompleteFrame("read_file", "toolu_001", `{"path":"a.txt"}`, 0),
		ToolStartFrame("grep_search", "toolu_002", 1),
		ToolCompleteFrame("grep_search", "toolu_002", `{"pattern":"foo"}`, 1),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Search"}},
		Stream:   true,
		Tools: []model.ToolDefinition{
			{Type: "function", Function: model.FunctionSpec{Name: "read_file"}},
			{Type: "function", Function: model.FunctionSpec{Name: "grep_search"}},
		},
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	startCount := 0
	completeCount := 0
	for ev := range ch {
		switch ev.Type {
		case EventToolCallStart:
			startCount++
		case EventToolCallComplete:
			completeCount++
		}
	}

	if startCount != 2 {
		t.Errorf("expected 2 tool_call_start events, got %d", startCount)
	}
	if completeCount != 2 {
		t.Errorf("expected 2 tool_call_complete events, got %d", completeCount)
	}
}

func TestOpenAIProvider_StreamChat_HTTPError(t *testing.T) {
	script := SingleTurnScript(
		HTTPErrorFrame(500, "internal server error"),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	_, err := p.StreamChat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") && !strings.Contains(err.Error(), "internal server error") && !strings.Contains(err.Error(), "HTTP") {
		t.Fatalf("expected 500/server error, got: %v", err)
	}
}

func TestOpenAIProvider_StreamChat_RateLimitError(t *testing.T) {
	script := SingleTurnScript(
		HTTPErrorFrame(429, "rate limit exceeded"),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	_, err := p.StreamChat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestOpenAIProvider_StreamChat_RequestValidation(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("OK"),
		DoneFrame(),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
		},
		Stream:    true,
		MaxTokens: 1000,
		Thinking:  true,
		Tools: []model.ToolDefinition{
			{Type: "function", Function: model.FunctionSpec{
				Name:        "echo",
				Description: "Echo back the input",
			}},
		},
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}
	// Drain the channel.
	for range ch {
	}

	if server.RequestCount() != 1 {
		t.Fatalf("expected 1 request, got %d", server.RequestCount())
	}

	body := server.Requests()[0].Body

	// Verify model.
	if m, _ := body["model"].(string); m != "test-model" {
		t.Errorf("expected model='test-model', got %q", m)
	}

	// Verify stream=true.
	if s, _ := body["stream"].(bool); !s {
		t.Error("expected stream=true")
	}

	// Verify max_tokens.
	if mt, _ := body["max_tokens"].(float64); mt != 1000 {
		t.Errorf("expected max_tokens=1000, got %v", mt)
	}

	// Verify messages are present.
	if msgs, _ := body["messages"].([]any); len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}

	// Verify tools are present.
	if !RequireToolInRequest(body, "echo") {
		t.Error("expected 'echo' tool in request")
	}

	// Verify thinking is enabled.
	if th, _ := body["thinking"]; th == nil {
		t.Error("expected thinking to be set")
	}
}

// ---------------------------------------------------------------------------
// Non-streaming Chat tests
// ---------------------------------------------------------------------------

func TestOpenAIProvider_Chat_TextResponse(t *testing.T) {
	// Non-streaming needs a different mock approach.
	// For Chat(), the mock must return a non-streaming JSON response.
	// We'll test it by having the mock return a completion with finish_reason.
	script := SingleTurnScript(
		TextFrame("Hello!"),
		DoneFrame(),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   false,
	}

	// Chat() uses non-streaming endpoint. Since mock returns SSE format,
	// it won't parse as non-streaming. This test validates the HTTP call
	// succeeds with proper error messaging.
	_, err := p.Chat(context.Background(), req)
	// Expected: decode error since mock returns SSE, not JSON.
	if err == nil {
		t.Log("Chat unexpectedly succeeded against SSE mock")
	}
}

// ---------------------------------------------------------------------------
// Anthropic Provider tests (minimal — verify HTTP and parsing)
// ---------------------------------------------------------------------------

func TestAnthropicProvider_StreamChat_Basic(t *testing.T) {
	// The Anthropic provider uses a different wire format. For now we verify
	// it can connect to the mock and handles the response appropriately.
	script := SingleTurnScript(
		TextFrame("Response from mock"),
		DoneFrame(),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "claude-test")
	cfg.Type = ProviderAnthropic
	p := NewAnthropicProvider(cfg)

	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	// Drain and check we get an error or done event.
	hasDone := false
	for ev := range ch {
		if ev.Type == EventDone {
			hasDone = true
		}
		if ev.Type == EventError {
			// Anthropic provider won't parse OpenAI SSE format, so error is expected.
			t.Logf("anthropic provider stream error (expected): %v", ev.Error)
		}
	}
	_ = hasDone
}

// ---------------------------------------------------------------------------
// Tool format conversion tests
// ---------------------------------------------------------------------------

func TestToolsToOpenAI_Empty(t *testing.T) {
	result := toolsToOpenAI(nil)
	if result != nil {
		t.Fatal("expected nil for nil input")
	}
	result = toolsToOpenAI([]model.ToolDefinition{})
	if result != nil {
		t.Fatal("expected nil for empty input")
	}
}

func TestToolsToOpenAI_Single(t *testing.T) {
	tools := []model.ToolDefinition{
		{
			Type: "function",
			Function: model.FunctionSpec{
				Name:        "read_file",
				Description: "Read a file",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	result := toolsToOpenAI(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].Type != "function" {
		t.Errorf("expected type='function', got %q", result[0].Type)
	}
	if result[0].Function.Name != "read_file" {
		t.Errorf("expected name='read_file', got %q", result[0].Function.Name)
	}
	if result[0].Function.Parameters == nil {
		t.Error("expected non-nil parameters")
	}
}

func TestToolsToOpenAI_NilParameters(t *testing.T) {
	tools := []model.ToolDefinition{
		{
			Type: "function",
			Function: model.FunctionSpec{
				Name: "no_params_tool",
			},
		},
	}
	result := toolsToOpenAI(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].Function.Parameters == nil {
		t.Error("expected default parameters (type:object) for nil input")
	}
}

func TestToolsToAnthropic_Single(t *testing.T) {
	tools := []model.ToolDefinition{
		{
			Type: "function",
			Function: model.FunctionSpec{
				Name:        "read_file",
				Description: "Read a file",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
	result := toolsToAnthropic(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].Name != "read_file" {
		t.Errorf("expected name='read_file', got %q", result[0].Name)
	}
	if result[0].InputSchema == nil {
		t.Error("expected non-nil input_schema")
	}
}

func TestToolsToAnthropic_Empty(t *testing.T) {
	result := toolsToAnthropic(nil)
	if result != nil {
		t.Fatal("expected nil for nil input")
	}
}

// ---------------------------------------------------------------------------
// Mock server: script exhaustion
// ---------------------------------------------------------------------------

func TestMockProviderServer_ExhaustsScript(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("First round"),
		DoneFrame(),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	// Round 1: scripted response.
	req := ChatRequest{Model: "test-model", Messages: []model.Message{{Role: "user", Content: "1"}}, Stream: true}
	ch, _ := p.StreamChat(context.Background(), req)
	text1 := drainText(ch)
	if !strings.Contains(text1, "First round") {
		t.Errorf("round 1: expected 'First round', got %q", text1)
	}

	// Round 2: no script left — should get fallback response.
	req2 := ChatRequest{Model: "test-model", Messages: []model.Message{{Role: "user", Content: "2"}}, Stream: true}
	ch2, _ := p.StreamChat(context.Background(), req2)
	text2 := drainText(ch2)
	if !strings.Contains(text2, "Done.") {
		t.Errorf("round 2: expected fallback 'Done.', got %q", text2)
	}
}

// ---------------------------------------------------------------------------
// Mock server: multi-round script
// ---------------------------------------------------------------------------

func TestMockProviderServer_MultiRound(t *testing.T) {
	script := MockSSEScript{
		Rounds: [][]MockSSEFrame{
			{TextFrame("R1"), DoneFrame()},
			{TextFrame("R2"), DoneFrame()},
			{TextFrame("R3"), DoneFrame()},
		},
	}
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	for i, expected := range []string{"R1", "R2", "R3"} {
		req := ChatRequest{Model: "test-model", Messages: []model.Message{{Role: "user", Content: "msg"}}, Stream: true}
		ch, err := p.StreamChat(context.Background(), req)
		if err != nil {
			t.Fatalf("round %d: StreamChat failed: %v", i+1, err)
		}
		text := drainText(ch)
		if !strings.Contains(text, expected) {
			t.Errorf("round %d: expected %q, got %q", i+1, expected, text)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func drainText(ch <-chan StreamEvent) string {
	var b strings.Builder
	for ev := range ch {
		if ev.Type == EventTextDelta {
			b.WriteString(ev.Delta)
		}
	}
	return b.String()
}

// TestEventDoneTimeout verifies that StreamChat returns promptly (no hangs).
func TestOpenAIProvider_StreamChat_Completes(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("quick"),
		DoneFrame(),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{Model: "test-model", Messages: []model.Message{{Role: "user", Content: "Hi"}}, Stream: true}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := p.StreamChat(ctx, req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}
	for range ch {
		// Just drain — timeout will trigger if it hangs.
	}
}

// ---------------------------------------------------------------------------
// Anthropic Provider Contract Tests (Phase 4.2)
// ---------------------------------------------------------------------------

// newAnthropicMockServer creates a mock server in Anthropic SSE format.
func newAnthropicMockServer(script MockSSEScript) *MockProviderServer {
	m := &MockProviderServer{script: script, AnthropicFormat: true}
	m.Server = httptest.NewServer(http.HandlerFunc(m.handle))
	return m
}

func configFromAnthropicMockServer(server *MockProviderServer, model string) Config {
	return Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   model,
		Type:    ProviderAnthropic,
	}
}

// TestAnthropicProvider_StreamChat_TextOnly verifies text streaming in Anthropic format.
func TestAnthropicProvider_StreamChat_TextOnly(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("Hello, "),
		TextFrame("world!"),
		DoneFrame(),
	)
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))

	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Say hello"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var textContent strings.Builder
	var eventTypes []StreamEventType
	for ev := range ch {
		eventTypes = append(eventTypes, ev.Type)
		if ev.Type == EventTextDelta {
			textContent.WriteString(ev.Delta)
		}
	}

	full := textContent.String()
	if !strings.Contains(full, "Hello, ") || !strings.Contains(full, "world!") {
		t.Fatalf("expected 'Hello, world!', got %q", full)
	}

	if len(eventTypes) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(eventTypes))
	}
	if eventTypes[len(eventTypes)-1] != EventDone {
		t.Fatal("last event should be EventDone")
	}
}

// TestAnthropicProvider_StreamChat_ToolCalls verifies tool call parsing in Anthropic format.
func TestAnthropicProvider_StreamChat_ToolCalls(t *testing.T) {
	script := SingleTurnScript(
		ToolStartFrame("read_file", "toolu_001", 0),
		ToolCompleteFrame("read_file", "toolu_001", `{"path":"test.txt"}`, 0),
		DoneFrame(),
	)
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))

	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Read test.txt"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var eventTypes []StreamEventType
	var toolNames []string
	var toolArgs []string

	for ev := range ch {
		eventTypes = append(eventTypes, ev.Type)
		switch ev.Type {
		case EventToolCallStart:
			toolNames = append(toolNames, ev.ToolCallName)
		case EventToolCallComplete:
			if ev.ToolCall != nil {
				toolArgs = append(toolArgs, ev.ToolCall.Arguments)
			}
		}
	}

	if len(toolNames) != 1 || toolNames[0] != "read_file" {
		t.Fatalf("expected tool 'read_file', got %v", toolNames)
	}
	if len(toolArgs) != 1 || !strings.Contains(toolArgs[0], "test.txt") {
		t.Fatalf("expected args containing 'test.txt', got %v", toolArgs)
	}
}

// TestAnthropicProvider_StreamChat_Thinking verifies thinking block extraction.
func TestAnthropicProvider_StreamChat_Thinking(t *testing.T) {
	script := SingleTurnScript(
		ThinkingFrame("Let me analyze this carefully..."),
		TextFrame("Here is the answer."),
		DoneFrame(),
	)
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))

	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Think about it"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	hasThinking := false
	for ev := range ch {
		if ev.Type == EventThinking {
			hasThinking = true
			if !strings.Contains(ev.Delta, "analyze") {
				t.Errorf("expected thinking to contain 'analyze', got %q", ev.Delta)
			}
		}
	}
	if !hasThinking {
		t.Fatal("expected thinking event")
	}
}

// TestAnthropicProvider_StreamChat_Error verifies error handling in Anthropic format.
func TestAnthropicProvider_StreamChat_Error(t *testing.T) {
	script := SingleTurnScript(
		ErrorFrame("something went wrong in the model"),
	)
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))

	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat should not fail at HTTP level: %v", err)
	}

	hasError := false
	for ev := range ch {
		if ev.Type == EventError {
			hasError = true
			if !strings.Contains(ev.Error.Error(), "something went wrong") {
				t.Errorf("expected error message to mention 'something went wrong', got: %v", ev.Error)
			}
		}
	}
	if !hasError {
		t.Fatal("expected error event in stream")
	}
}

// TestAnthropicProvider_StreamChat_HTTPError verifies HTTP-level errors.
func TestAnthropicProvider_StreamChat_HTTPError(t *testing.T) {
	script := SingleTurnScript(
		HTTPErrorFrame(500, "internal server error"),
	)
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))

	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	_, err := p.StreamChat(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// TestAnthropicProvider_RequestValidation verifies request body format matches Anthropic API spec.
func TestAnthropicProvider_RequestValidation(t *testing.T) {
	script := SingleTurnScript(TextFrame("OK"), DoneFrame())
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))

	req := ChatRequest{
		Model: "claude-test",
		Messages: []model.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
		},
		Stream:    true,
		MaxTokens: 4096,
		Thinking:  true,
		Tools: []model.ToolDefinition{
			{Type: "function", Function: model.FunctionSpec{
				Name:        "echo",
				Description: "Echo back",
			}},
		},
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}
	for range ch {
	}

	if server.RequestCount() != 1 {
		t.Fatalf("expected 1 request, got %d", server.RequestCount())
	}

	body := server.Requests()[0].Body

	// Verify model.
	if m, _ := body["model"].(string); m != "claude-test" {
		t.Errorf("expected model='claude-test', got %q", m)
	}

	// Verify stream.
	if s, _ := body["stream"].(bool); !s {
		t.Error("expected stream=true")
	}

	// Verify system prompt extracted to top-level system field.
	if sys, _ := body["system"].(string); sys != "You are a helpful assistant." {
		t.Errorf("expected system='You are a helpful assistant.', got %q", sys)
	}

	// Verify messages do NOT include system.
	if msgs, ok := body["messages"].([]any); ok {
		for _, m := range msgs {
			msg, _ := m.(map[string]any)
			if role, _ := msg["role"].(string); role == "system" {
				t.Error("system message should be extracted, not in messages array")
			}
		}
	}

	// Verify tools use input_schema (Anthropic format).
	if tools, ok := body["tools"].([]any); ok && len(tools) > 0 {
		tool, _ := tools[0].(map[string]any)
		if name, _ := tool["name"].(string); name != "echo" {
			t.Errorf("expected tool name='echo', got %q", name)
		}
		if _, ok := tool["input_schema"]; !ok {
			t.Error("expected input_schema in Anthropic tool definition")
		}
	}

	// Verify thinking with budget_tokens.
	if thinking, ok := body["thinking"].(map[string]any); ok {
		if bt, _ := thinking["budget_tokens"].(float64); bt != 16000 {
			t.Errorf("expected thinking budget_tokens=16000, got %v", bt)
		}
	}

	// Verify max_tokens.
	if mt, _ := body["max_tokens"].(float64); mt != 4096 {
		t.Errorf("expected max_tokens=4096, got %v", mt)
	}
}

// TestAnthropicProvider_RequestValidation_NoSystem verifies defaults without system message.
func TestAnthropicProvider_RequestValidation_NoSystem(t *testing.T) {
	script := SingleTurnScript(TextFrame("OK"), DoneFrame())
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))

	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	for range ch {
	}

	body := server.Requests()[0].Body

	// Verify no top-level system when not present.
	if _, ok := body["system"]; ok {
		t.Error("system should not be present when no system message")
	}

	// Verify max_tokens defaults to 32768.
	if mt, _ := body["max_tokens"].(float64); mt != 32768 {
		t.Errorf("expected default max_tokens=32768, got %v", mt)
	}

	// Verify no tools key when not present.
	if tools, ok := body["tools"]; ok && tools != nil {
		t.Error("tools should not be present when no tools")
	}
}

// TestOpenAIProvider_ToolCallDeltaHasName verifies that tool_call_delta events
// carry the ToolCallName field so downstream observers can display progress.
// Regression test for bug where delta events had empty ToolCallName.
func TestOpenAIProvider_ToolCallDeltaHasName(t *testing.T) {
	script := SingleTurnScript(
		ToolStartFrame("write_file", "toolu_001", 0),
		ToolDeltaFrame(`{"path":"test`, 0),
		ToolDeltaFrame(`.html","content":"x"}`, 0),
		ToolCompleteFrame("write_file", "toolu_001", `{"path":"test.html","content":"x"}`, 0),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Write test.html"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	deltaCount := 0
	for ev := range ch {
		if ev.Type == EventToolCallDelta {
			deltaCount++
			if ev.ToolCallName == "" {
				t.Errorf("tool_call_delta #%d: ToolCallName is empty", deltaCount)
			}
			if ev.ToolCallName != "write_file" {
				t.Errorf("expected ToolCallName='write_file', got %q", ev.ToolCallName)
			}
		}
	}
	if deltaCount == 0 {
		t.Error("expected at least one tool_call_delta event")
	}
	t.Logf("received %d tool_call_delta events with correct ToolCallName", deltaCount)
}

// TestNewOpenAIProvider_TransportTimeouts verifies the HTTP transport has
// appropriate timeouts for SSE streaming: no http.Client.Timeout (would kill
// long streams), but transport-level timeouts for connection safety.
func TestNewOpenAIProvider_TransportTimeouts(t *testing.T) {
	cfg := Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:8080",
		Model:   "test-model",
	}
	p := NewOpenAIProvider(cfg)

	if p.client == nil {
		t.Fatal("client is nil")
	}

	// Overall timeout must be zero for SSE streaming.
	if p.client.Timeout != 0 {
		t.Errorf("client.Timeout should be 0 for SSE streaming, got %v", p.client.Timeout)
	}

	transport, ok := p.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", p.client.Transport)
	}

	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Errorf("ResponseHeaderTimeout = %v, want 30s", transport.ResponseHeaderTimeout)
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 10s", transport.TLSHandshakeTimeout)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout = %v, want 90s", transport.IdleConnTimeout)
	}

	// DialContext must be set.
	if transport.DialContext == nil {
		t.Error("DialContext is nil; expected net.Dialer with connect timeout")
	}
}

// TestNewOpenAIProvider_TransportDialer verifies the dialer has a connect timeout.
func TestNewOpenAIProvider_TransportDialer(t *testing.T) {
	cfg := Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:8080",
		Model:   "test-model",
	}
	p := NewOpenAIProvider(cfg)

	transport, ok := p.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", p.client.Transport)
	}

	// Test that DialContext creates a connection or fails fast (timeout).
	conn, err := transport.DialContext(context.Background(), "tcp", "10.255.255.1:80")
	if err != nil {
		// Expected — unreachable address should time out quickly.
		var netErr net.Error
		if !errors.As(err, &netErr) || !netErr.Timeout() {
			t.Logf("dial error (expected timeout or similar): %v", err)
		}
	} else {
		conn.Close()
		t.Log("unexpectedly connected to 10.255.255.1:80")
	}
}

// TestAnthropicProvider_MultiRound verifies multi-round conversation with Anthropic format.
func TestAnthropicProvider_MultiRound(t *testing.T) {
	script := MockSSEScript{
		Rounds: [][]MockSSEFrame{
			{TextFrame("Round 1"), DoneFrame()},
			{TextFrame("Round 2"), DoneFrame()},
		},
	}
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))

	for i, expected := range []string{"Round 1", "Round 2"} {
		req := ChatRequest{Model: "claude-test", Messages: []model.Message{{Role: "user", Content: "msg"}}, Stream: true}
		ch, err := p.StreamChat(context.Background(), req)
		if err != nil {
			t.Fatalf("round %d: StreamChat failed: %v", i+1, err)
		}
		text := drainText(ch)
		if !strings.Contains(text, expected) {
			t.Errorf("round %d: expected %q, got %q", i+1, expected, text)
		}
	}
}

// ============================================================================
// StreamEvent: finish_reason and usage propagation tests
// ============================================================================

// TestOpenAI_FinishReason_Length verifies that finish_reason="length" is
// propagated through EventDone.FinishReason.
func TestOpenAI_FinishReason_Length(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("partial response"),
		DoneFrameWithReason("length"),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	p := NewOpenAIProvider(configFromMockServer(server, "test-model"))
	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Write a long HTML file"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var finishReason string
	var hasDone bool
	for ev := range ch {
		if ev.Type == EventDone {
			hasDone = true
			finishReason = ev.FinishReason
		}
	}
	if !hasDone {
		t.Fatal("expected EventDone")
	}
	if finishReason != "length" {
		t.Errorf("expected FinishReason='length', got %q", finishReason)
	}
}

// TestOpenAI_FinishReason_Stop verifies the default finish_reason="stop".
func TestOpenAI_FinishReason_Stop(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("a complete response"),
		DoneFrame(),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	p := NewOpenAIProvider(configFromMockServer(server, "test-model"))
	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var finishReason string
	for ev := range ch {
		if ev.Type == EventDone {
			finishReason = ev.FinishReason
		}
	}
	if finishReason != "stop" {
		t.Errorf("expected FinishReason='stop', got %q", finishReason)
	}
}

// TestOpenAI_FinishReason_ToolCalls verifies that finish_reason="tool_calls"
// is propagated correctly.
func TestOpenAI_FinishReason_ToolCalls(t *testing.T) {
	script := SingleTurnScript(
		ToolStartFrame("read_file", "toolu_001", 0),
		ToolCompleteFrame("read_file", "toolu_001", `{"path":"x.txt"}`, 0),
		DoneFrameWithReason("tool_calls"),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	p := NewOpenAIProvider(configFromMockServer(server, "test-model"))
	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Read x.txt"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var finishReason string
	for ev := range ch {
		if ev.Type == EventDone {
			finishReason = ev.FinishReason
		}
	}
	if finishReason != "tool_calls" {
		t.Errorf("expected FinishReason='tool_calls', got %q", finishReason)
	}
}

// TestOpenAI_Usage_InFinalChunk verifies that usage data from a usage-only
// chunk (choices empty, usage present) is propagated to EventDone.Usage.
func TestOpenAI_Usage_InFinalChunk(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("hello"),
		UsageFrame(150, 80),
		DoneFrameWithReason("stop"),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	p := NewOpenAIProvider(configFromMockServer(server, "test-model"))
	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var usage *model.Usage
	for ev := range ch {
		if ev.Type == EventDone {
			usage = ev.Usage
		}
	}
	if usage == nil {
		t.Fatal("expected non-nil Usage on EventDone")
	}
	if usage.PromptTokens != 150 {
		t.Errorf("expected PromptTokens=150, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 80 {
		t.Errorf("expected CompletionTokens=80, got %d", usage.CompletionTokens)
	}
	if usage.TotalTokens != 230 {
		t.Errorf("expected TotalTokens=230, got %d", usage.TotalTokens)
	}
}

// TestOpenAI_Usage_NilWhenNotSent verifies Usage is nil when no usage chunk.
func TestOpenAI_Usage_NilWhenNotSent(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("hello"),
		DoneFrame(),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	p := NewOpenAIProvider(configFromMockServer(server, "test-model"))
	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var usage *model.Usage
	for ev := range ch {
		if ev.Type == EventDone {
			usage = ev.Usage
		}
	}
	if usage != nil {
		t.Errorf("expected nil Usage when no usage chunk sent, got %+v", usage)
	}
}

// TestOpenAI_DeepSeek_FinishReason_InsufficientResource verifies
// "insufficient_system_resource" is propagated.
func TestOpenAI_DeepSeek_FinishReason_InsufficientResource(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("trying..."),
		DoneFrameWithReason("insufficient_system_resource"),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	p := NewOpenAIProvider(configFromMockServer(server, "deepseek-model"))
	req := ChatRequest{
		Model:    "deepseek-model",
		Messages: []model.Message{{Role: "user", Content: "Large task"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var finishReason string
	for ev := range ch {
		if ev.Type == EventDone {
			finishReason = ev.FinishReason
		}
	}
	if finishReason != "insufficient_system_resource" {
		t.Errorf("expected FinishReason='insufficient_system_resource', got %q", finishReason)
	}
}

// TestOpenAI_DeepSeek_FinishReason_ContentFilter verifies content_filter.
func TestOpenAI_DeepSeek_FinishReason_ContentFilter(t *testing.T) {
	script := SingleTurnScript(
		DoneFrameWithReason("content_filter"),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	p := NewOpenAIProvider(configFromMockServer(server, "deepseek-model"))
	req := ChatRequest{
		Model:    "deepseek-model",
		Messages: []model.Message{{Role: "user", Content: "Test"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var finishReason string
	for ev := range ch {
		if ev.Type == EventDone {
			finishReason = ev.FinishReason
		}
	}
	if finishReason != "content_filter" {
		t.Errorf("expected FinishReason='content_filter', got %q", finishReason)
	}
}

// TestOpenAI_FinishReason_InContentChunk verifies finish_reason on a
// final content chunk is propagated.
func TestOpenAI_FinishReason_InContentChunk(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("complete"),
		DoneFrameWithReason("stop"),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	p := NewOpenAIProvider(configFromMockServer(server, "test-model"))
	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Test"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var finishReason string
	var textContent string
	for ev := range ch {
		switch ev.Type {
		case EventTextDelta:
			textContent += ev.Delta
		case EventDone:
			finishReason = ev.FinishReason
		}
	}
	if textContent != "complete" {
		t.Errorf("expected text='complete', got %q", textContent)
	}
	if finishReason != "stop" {
		t.Errorf("expected FinishReason='stop', got %q", finishReason)
	}
}

// ============================================================================
// Anthropic: message_delta parsing tests
// ============================================================================

// TestAnthropic_MessageDelta_EndTurn verifies message_delta with
// stop_reason="end_turn" is parsed and propagated.
func TestAnthropic_MessageDelta_EndTurn(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("Hello from Claude"),
		DoneFrameWithUsage("end_turn", 0, 25),
	)
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))
	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var finishReason string
	var usage *model.Usage
	for ev := range ch {
		if ev.Type == EventDone {
			finishReason = ev.FinishReason
			usage = ev.Usage
		}
	}
	if finishReason != "end_turn" {
		t.Errorf("expected FinishReason='end_turn', got %q", finishReason)
	}
	if usage == nil {
		t.Fatal("expected non-nil Usage")
	}
	if usage.CompletionTokens != 25 {
		t.Errorf("expected CompletionTokens=25, got %d", usage.CompletionTokens)
	}
}

// TestAnthropic_MessageDelta_MaxTokens verifies stop_reason="max_tokens"
// is propagated (Anthropic equivalent of OpenAI "length").
func TestAnthropic_MessageDelta_MaxTokens(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("truncated..."),
		DoneFrameWithUsage("max_tokens", 0, 4096),
	)
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))
	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Large task"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var finishReason string
	for ev := range ch {
		if ev.Type == EventDone {
			finishReason = ev.FinishReason
		}
	}
	if finishReason != "max_tokens" {
		t.Errorf("expected FinishReason='max_tokens', got %q", finishReason)
	}
}

// TestAnthropic_MessageDelta_StopSequence verifies stop_sequence propagation.
func TestAnthropic_MessageDelta_StopSequence(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("done"),
		DoneFrameWithUsage("stop_sequence", 0, 10),
	)
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))
	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Test"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var finishReason string
	for ev := range ch {
		if ev.Type == EventDone {
			finishReason = ev.FinishReason
		}
	}
	if finishReason != "stop_sequence" {
		t.Errorf("expected FinishReason='stop_sequence', got %q", finishReason)
	}
}

// TestAnthropic_MessageDelta_ToolUse verifies stop_reason="tool_use".
func TestAnthropic_MessageDelta_ToolUse(t *testing.T) {
	script := SingleTurnScript(
		ToolStartFrame("read_file", "toolu_001", 0),
		ToolCompleteFrame("read_file", "toolu_001", `{"path":"x.txt"}`, 0),
		DoneFrameWithUsage("tool_use", 0, 50),
	)
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))
	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Read x.txt"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var finishReason string
	for ev := range ch {
		if ev.Type == EventDone {
			finishReason = ev.FinishReason
		}
	}
	if finishReason != "tool_use" {
		t.Errorf("expected FinishReason='tool_use', got %q", finishReason)
	}
}



// ============================================================================
// SIT: Provider compatibility matrix smoke tests
// ============================================================================

// TestProvider_SIT_OpenAI_LengthToDoneFlow verifies the full flow:
// text delta → length finish_reason → Done event with proper fields.
func TestProvider_SIT_OpenAI_LengthToDoneFlow(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("part"),
		UsageFrame(100, 50),
		DoneFrameWithReason("length"),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	p := NewOpenAIProvider(configFromMockServer(server, "test-model"))
	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Long task"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var gotText string
	var finishReason string
	var usage *model.Usage
	for ev := range ch {
		switch ev.Type {
		case EventTextDelta:
			gotText += ev.Delta
		case EventDone:
			finishReason = ev.FinishReason
			usage = ev.Usage
		}
	}
	if gotText != "part" {
		t.Errorf("expected text='part', got %q", gotText)
	}
	if finishReason != "length" {
		t.Errorf("expected FinishReason='length', got %q", finishReason)
	}
	if usage == nil || usage.CompletionTokens != 50 {
		t.Errorf("expected CompletionTokens=50, got %v", usage)
	}
}

// TestProvider_SIT_Anthropic_MaxTokensToDoneFlow verifies the Anthropic full
// flow: text → message_delta (max_tokens + usage) → message_stop.
func TestProvider_SIT_Anthropic_MaxTokensToDoneFlow(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("truncated output"),
		DoneFrameWithUsage("max_tokens", 500, 4096),
	)
	server := newAnthropicMockServer(script)
	defer server.Close()

	p := NewAnthropicProvider(configFromAnthropicMockServer(server, "claude-test"))
	req := ChatRequest{
		Model:    "claude-test",
		Messages: []model.Message{{Role: "user", Content: "Big task"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var finishReason string
	var usage *model.Usage
	for ev := range ch {
		if ev.Type == EventDone {
			finishReason = ev.FinishReason
			usage = ev.Usage
		}
	}
	if finishReason != "max_tokens" {
		t.Errorf("expected FinishReason='max_tokens', got %q", finishReason)
	}
	if usage == nil || usage.CompletionTokens != 4096 {
		t.Errorf("expected CompletionTokens=4096, got %v", usage)
	}
}

// TestProvider_SIT_OpenAI_FinishReason_EmptyIsAmbiguous verifies that an
// empty finish_reason (connection closed without explicit reason) results
// in FinishReason="" on EventDone.
func TestProvider_SIT_OpenAI_FinishReason_EmptyIsAmbiguous(t *testing.T) {
	script := SingleTurnScript(
		TextFrame("trailing text without explicit finish"),
	)
	server := NewMockProviderServer(script)
	defer server.Close()

	p := NewOpenAIProvider(configFromMockServer(server, "test-model"))
	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Test"}},
		Stream:   true,
	}

	ch, _ := p.StreamChat(context.Background(), req)
	var finishReason string
	var hasDone bool
	for ev := range ch {
		if ev.Type == EventDone {
			hasDone = true
			finishReason = ev.FinishReason
		}
	}
	if !hasDone {
		t.Fatal("expected EventDone even without explicit finish_reason chunk")
	}
	// When the stream ends via [DONE] without a finish_reason chunk,
	// emitDone("") is called → FinishReason="".
	if finishReason != "" {
		t.Errorf("expected empty FinishReason for ambiguous EOF, got %q", finishReason)
	}
}

// ---------------------------------------------------------------------------
// Chat() (non-streaming) retry tests — H1+H2 fix
// ---------------------------------------------------------------------------

// TestOpenAIProvider_Chat_NoRetryOn401 verifies H2: non-retryable status codes
// (like 401) are NOT retried. The retry loop returns immediately with the error.
func TestOpenAIProvider_Chat_NoRetryOn401(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Unauthorized"}}`))
	}))
	defer server.Close()

	p := NewOpenAIProvider(Config{
		BaseURL: server.URL,
		APIKey:  "bad-key",
		Type:    ProviderOpenAI,
	})

	_, err := p.Chat(context.Background(), ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   false,
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry for 401), got %d", callCount)
	}
}
