package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"agentgo/internal/model"
	"agentgo/internal/observability"
	"agentgo/internal/provider"
)

func makeEventCh(events ...provider.StreamEvent) <-chan provider.StreamEvent {
	ch := make(chan provider.StreamEvent, len(events))
	for _, ev := range events {
		ch <- ev
	}
	close(ch)
	return ch
}

func TestProcessStreamEvents_TextOnly(t *testing.T) {
	cfg := StreamingLoopConfig{}
	var st LoopState
	ch := makeEventCh(
		textDeltaEvent("Hello, world!"),
		provider.StreamEvent{Type: provider.EventDone},
	)

	sr := processStreamEvents(context.Background(), cfg, &st, ch, time.Now(), 0,
		observability.NewRoundEventBuffer(), NewCircuitBreaker())

	if sr.terminalError != nil {
		t.Fatalf("unexpected error: %v", sr.terminalError)
	}
	if !strings.Contains(sr.contentText, "Hello, world!") {
		t.Errorf("expected content, got: %s", sr.contentText)
	}
}

func TestProcessStreamEvents_ToolCalls(t *testing.T) {
	cfg := StreamingLoopConfig{}
	var st LoopState
	ch := makeEventCh(
		toolCallStartEvent("write_file", "call_1", 0),
		toolCallCompleteEvent("write_file", "call_1", `{"path":"index.html"}`),
		provider.StreamEvent{Type: provider.EventDone},
	)

	sr := processStreamEvents(context.Background(), cfg, &st, ch, time.Now(), 0,
		observability.NewRoundEventBuffer(), NewCircuitBreaker())

	if sr.terminalError != nil {
		t.Fatalf("unexpected error: %v", sr.terminalError)
	}
	if len(sr.toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(sr.toolCalls))
	}
	if sr.toolCalls[0].Function.Name != "write_file" {
		t.Errorf("expected write_file, got %q", sr.toolCalls[0].Function.Name)
	}
}

func TestProcessStreamEvents_DoneWithFinishReason(t *testing.T) {
	cfg := StreamingLoopConfig{}
	var st LoopState
	ch := makeEventCh(
		textDeltaEvent("done."),
		provider.StreamEvent{Type: provider.EventDone, FinishReason: "stop",
			Usage: &model.Usage{CompletionTokens: 42}},
	)

	sr := processStreamEvents(context.Background(), cfg, &st, ch, time.Now(), 0,
		observability.NewRoundEventBuffer(), NewCircuitBreaker())

	if sr.terminalError != nil {
		t.Fatalf("unexpected error: %v", sr.terminalError)
	}
	if sr.finishReason != "stop" {
		t.Errorf("expected 'stop', got: %s", sr.finishReason)
	}
	if sr.streamUsage == nil || sr.streamUsage.CompletionTokens != 42 {
		t.Error("expected usage with 42 tokens")
	}
}

func TestProcessStreamEvents_ErrorTerminal(t *testing.T) {
	cfg := StreamingLoopConfig{Emitter: observability.NewEmitter()}
	var st LoopState
	ch := makeEventCh(
		provider.StreamEvent{Type: provider.EventError, Error: errors.New("fatal")},
	)

	sr := processStreamEvents(context.Background(), cfg, &st, ch, time.Now(), 0,
		observability.NewRoundEventBuffer(), NewCircuitBreaker())

	if sr.terminalError == nil {
		t.Fatal("expected terminal error")
	}
}

func TestProcessStreamEvents_EmptyStream(t *testing.T) {
	cfg := StreamingLoopConfig{}
	var st LoopState
	ch := makeEventCh(
		provider.StreamEvent{Type: provider.EventDone},
	)

	sr := processStreamEvents(context.Background(), cfg, &st, ch, time.Now(), 0,
		observability.NewRoundEventBuffer(), NewCircuitBreaker())

	if sr.terminalError != nil {
		t.Fatalf("unexpected error: %v", sr.terminalError)
	}
	if sr.contentText != "" {
		t.Errorf("expected empty content, got: %s", sr.contentText)
	}
}

func TestProcessStreamEvents_PopulatesTimeline(t *testing.T) {
	cfg := StreamingLoopConfig{}
	var st LoopState
	ch := makeEventCh(
		textDeltaEvent("hello"),
		provider.StreamEvent{Type: provider.EventDone},
	)

	_ = processStreamEvents(context.Background(), cfg, &st, ch, time.Now(), 0,
		observability.NewRoundEventBuffer(), NewCircuitBreaker())

	if len(st.Timeline) == 0 {
		t.Error("expected timeline entries")
	}
}

func TestProcessStreamEvents_Thinking(t *testing.T) {
	cfg := StreamingLoopConfig{}
	var st LoopState
	ch := makeEventCh(
		provider.StreamEvent{Type: provider.EventThinking, Delta: "Let me think..."},
		textDeltaEvent("answer"),
		provider.StreamEvent{Type: provider.EventDone},
	)

	sr := processStreamEvents(context.Background(), cfg, &st, ch, time.Now(), 0,
		observability.NewRoundEventBuffer(), NewCircuitBreaker())

	if sr.terminalError != nil {
		t.Fatalf("unexpected error: %v", sr.terminalError)
	}
	if sr.thinkingText != "Let me think..." {
		t.Errorf("expected thinking, got: %s", sr.thinkingText)
	}
	if !sr.hadAnyDelta {
		t.Error("expected hadAnyDelta=true")
	}
}
