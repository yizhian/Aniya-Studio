package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agentgo/internal/model"
	"agentgo/internal/retry"
)

// ============================================================================
// SIT: Provider + Retry integration tests
// ============================================================================

func TestSIT_RetryOnTransientError(t *testing.T) {
	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return &retry.RetryableHTTPError{Code: 500}
		}
		return nil
	}

	err := retry.Do(context.Background(), 5, fn)
	if err != nil {
		t.Fatalf("retry should succeed after transient errors: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestSIT_Retry_Exhausted(t *testing.T) {
	callCount := 0
	fn := func() error {
		callCount++
		return &retry.RetryableHTTPError{Code: 503}
	}

	err := retry.Do(context.Background(), 2, fn)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if !strings.Contains(err.Error(), "retries exhausted") {
		t.Errorf("unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount)
	}
}

func TestSIT_Retry_NonRetryableError(t *testing.T) {
	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("permanent failure")
	}

	err := retry.Do(context.Background(), 5, fn)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call for non-retryable error, got %d", callCount)
	}
}

func TestSIT_Retry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 1 {
			cancel()
		}
		return &retry.RetryableHTTPError{Code: 500}
	}

	err := retry.Do(ctx, 5, fn)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSIT_Retry_ContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	fn := func() error {
		return &retry.RetryableHTTPError{Code: 500}
	}

	err := retry.Do(ctx, 10, fn)
	if err == nil {
		t.Fatal("expected error from timed out context")
	}
}

func TestSIT_Retry_ExponentialBackoff(t *testing.T) {
	start := time.Now()
	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return &retry.RetryableHTTPError{Code: 500}
		}
		return nil
	}

	err := retry.Do(context.Background(), 5, fn)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed < 500*time.Millisecond {
		t.Errorf("expected at least some backoff delay, got %v", elapsed)
	}
}

func TestSIT_Retry_HTTPStatusCodes(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{500, true}, {502, true}, {503, true}, {504, true},
		{429, true}, {408, true},
		{400, false}, {401, false}, {403, false}, {404, false},
		{200, false}, {301, false},
	}
	for _, tc := range tests {
		if retry.IsRetryableHTTPStatus(tc.code) != tc.expected {
			t.Errorf("IsRetryableHTTPStatus(%d) = %v, want %v", tc.code, !tc.expected, tc.expected)
		}
	}
}

func TestSIT_Retry_RetryableHTTPError(t *testing.T) {
	// Verify RetryableHTTPError implements error.
	err := &retry.RetryableHTTPError{Code: 429}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ============================================================================
// SIT: Provider + Mock Server integration
// ============================================================================

func TestSIT_Provider_StreamChat_WholeFlow(t *testing.T) {
	script := MockSSEScript{
		Rounds: [][]MockSSEFrame{
			{
				ThinkingFrame("Let me think..."),
				ThinkingFrame(" analyzing the request..."),
				TextFrame("I'll help you "),
				TextFrame("with that."),
				DoneFrame(),
			},
		},
	}
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Help me"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	eventCount := 0
	var textContent strings.Builder
	var thinkingContent strings.Builder
	var doneReceived bool

	for ev := range ch {
		eventCount++
		switch ev.Type {
		case EventThinking:
			thinkingContent.WriteString(ev.Delta)
		case EventTextDelta:
			textContent.WriteString(ev.Delta)
		case EventDone:
			doneReceived = true
		}
	}

	if !doneReceived {
		t.Error("expected EventDone at end")
	}
	if thinkingContent.String() != "Let me think... analyzing the request..." {
		t.Errorf("unexpected thinking: %q", thinkingContent.String())
	}
	if textContent.String() != "I'll help you with that." {
		t.Errorf("unexpected text: %q", textContent.String())
	}
	if eventCount < 5 {
		t.Errorf("expected at least 5 events, got %d", eventCount)
	}
}

func TestSIT_Provider_MultiRound_WithToolCalls(t *testing.T) {
	script := MockSSEScript{
		Rounds: [][]MockSSEFrame{
			{
				ToolStartFrame("read_file", "toolu_001", 0),
				ToolCompleteFrame("read_file", "toolu_001", `{"path":"test.txt"}`, 0),
				DoneFrameWithReason("tool_calls"),
			},
			{
				TextFrame("I read the file "),
				TextFrame("successfully."),
				DoneFrame(),
			},
		},
	}
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req1 := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Read test.txt"}},
		Stream:   true,
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.FunctionSpec{Name: "read_file"}}},
	}
	ch1, err := p.StreamChat(context.Background(), req1)
	if err != nil {
		t.Fatalf("round 1 failed: %v", err)
	}
	toolNames := collectToolNames(ch1)
	if len(toolNames) != 1 || toolNames[0] != "read_file" {
		t.Errorf("expected [read_file] tool call, got %v", toolNames)
	}

	req2 := ChatRequest{
		Model: "test-model",
		Messages: []model.Message{
			{Role: "user", Content: "Read test.txt"},
			{Role: "assistant", ToolCalls: []model.ToolCall{
				{ID: "toolu_001", Type: "function", Function: model.ToolCallFunction{Name: "read_file", Arguments: `{"path":"test.txt"}`}},
			}},
			{Role: "tool", Content: "file contents here", ToolCallID: "toolu_001"},
		},
		Stream: true,
	}
	ch2, err := p.StreamChat(context.Background(), req2)
	if err != nil {
		t.Fatalf("round 2 failed: %v", err)
	}
	text := collectText(ch2)
	if !strings.Contains(text, "successfully") {
		t.Errorf("unexpected round 2 text: %q", text)
	}
}

func TestSIT_Provider_TruncationFinishReason(t *testing.T) {
	script := MockSSEScript{
		Rounds: [][]MockSSEFrame{
			{
				TextFrame("Starting to write "),
				ToolStartFrame("write_file", "toolu_trunc", 0),
				ToolCompleteFrame("write_file", "toolu_trunc", `{"path":"out.html","content":"<html>...`, 0),
				DoneFrameWithReason("length"),
			},
		},
	}
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Write a file"}},
		Stream:   true,
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.FunctionSpec{Name: "write_file"}}},
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	var doneEvent StreamEvent
	for ev := range ch {
		if ev.Type == EventDone {
			doneEvent = ev
		}
	}

	if doneEvent.FinishReason != "length" {
		t.Errorf("expected finish_reason 'length', got %q", doneEvent.FinishReason)
	}
}

func TestSIT_Provider_RequestCapture(t *testing.T) {
	script := MockSSEScript{
		Rounds: [][]MockSSEFrame{
			{TextFrame("OK"), DoneFrame()},
		},
	}
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "capture-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "capture-model",
		Messages: []model.Message{{Role: "user", Content: "Test request capture"}},
		Stream:   true,
	}

	_, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	if server.RequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", server.RequestCount())
	}
}

func TestSIT_Provider_HTTPErrorStatusCode(t *testing.T) {
	script := MockSSEScript{
		Rounds: [][]MockSSEFrame{
			{{StatusCode: 500, ErrorMsg: "Internal Server Error"}},
		},
	}
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "test-model")
	p := NewOpenAIProvider(cfg)

	req := ChatRequest{
		Model:    "test-model",
		Messages: []model.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	ch, err := p.StreamChat(context.Background(), req)
	if err != nil {
		t.Logf("HTTP error returned directly: %v", err)
		return
	}
	for ev := range ch {
		if ev.Type == EventError {
			return
		}
	}
}

func TestSIT_Provider_429RetryScenario(t *testing.T) {
	attempts := 0
	err := retry.Do(context.Background(), 3, func() error {
		attempts++
		if attempts < 3 {
			return &retry.RetryableHTTPError{Code: 429}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("retry should succeed: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestSIT_Provider_CompleteEndToEndFlow(t *testing.T) {
	// Full flow: thinking → tool calls → text → done.
	script := MockSSEScript{
		Rounds: [][]MockSSEFrame{
			{
				ThinkingFrame("Analyzing..."),
				TextFrame("I will read the file for you."),
				ToolStartFrame("read_file", "toolu_e2e", 0),
				ToolCompleteFrame("read_file", "toolu_e2e", `{"path":"data.txt"}`, 0),
				DoneFrameWithReason("tool_calls"),
			},
			{
				TextFrame("The file contains important data."),
				DoneFrame(),
			},
		},
	}
	server := NewMockProviderServer(script)
	defer server.Close()

	cfg := configFromMockServer(server, "e2e-model")
	p := NewOpenAIProvider(cfg)

	// Round 1
	req1 := ChatRequest{
		Model:    "e2e-model",
		Messages: []model.Message{{Role: "user", Content: "Read data.txt"}},
		Stream:   true,
		Tools:    []model.ToolDefinition{{Type: "function", Function: model.FunctionSpec{Name: "read_file"}}},
	}
	ch1, err := p.StreamChat(context.Background(), req1)
	if err != nil {
		t.Fatalf("round 1 failed: %v", err)
	}
	collectStream(ch1)

	// Round 2
	req2 := ChatRequest{
		Model: "e2e-model",
		Messages: []model.Message{
			{Role: "user", Content: "Read data.txt"},
			{Role: "assistant", Content: "", ToolCalls: []model.ToolCall{
				{ID: "toolu_e2e", Type: "function", Function: model.ToolCallFunction{Name: "read_file", Arguments: `{"path":"data.txt"}`}},
			}},
			{Role: "tool", Content: "important data", ToolCallID: "toolu_e2e"},
		},
		Stream: true,
	}
	ch2, err := p.StreamChat(context.Background(), req2)
	if err != nil {
		t.Fatalf("round 2 failed: %v", err)
	}
	text := collectText(ch2)
	if !strings.Contains(text, "important") {
		t.Errorf("unexpected round 2 text: %q", text)
	}
}

// ============================================================================
// Helpers
// ============================================================================

func collectToolNames(ch <-chan StreamEvent) []string {
	var names []string
	for ev := range ch {
		if ev.Type == EventToolCallComplete {
			if ev.ToolCall != nil {
				names = append(names, ev.ToolCall.Name)
			}
		}
	}
	return names
}

func collectText(ch <-chan StreamEvent) string {
	var text strings.Builder
	for ev := range ch {
		if ev.Type == EventTextDelta {
			text.WriteString(ev.Delta)
		}
	}
	return text.String()
}

func collectStream(ch <-chan StreamEvent) {
	for range ch {
	}
}

var _ = context.Background
var _ = errors.New
