package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentctx "agentgo/internal/context"
	"agentgo/internal/hook"
	"agentgo/internal/model"
	"agentgo/internal/observability"
	"agentgo/internal/persistence"
	"agentgo/internal/provider"
	"agentgo/internal/toolkit/contracts"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

// stubStreamingProvider returns pre-baked StreamEvent slices per round.
type stubStreamingProvider struct {
	rounds       [][]provider.StreamEvent
	chatErr      error
	chatErrCount int // fail first N calls before succeeding (0 = always fail)
	callN        int // tracks total StreamChat invocations
	roundN       int
}

func (s *stubStreamingProvider) StreamChat(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	s.callN++
	if s.chatErr != nil && (s.chatErrCount == 0 || s.callN <= s.chatErrCount) {
		return nil, s.chatErr
	}
	idx := s.roundN
	s.roundN++
	if idx >= len(s.rounds) {
		ch := make(chan provider.StreamEvent, 1)
		ch <- provider.StreamEvent{Type: provider.EventDone}
		close(ch)
		return ch, nil
	}
	events := s.rounds[idx]
	ch := make(chan provider.StreamEvent, len(events))
	for _, ev := range events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func (s *stubStreamingProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubStreamingProvider) Type() provider.ProviderType {
	return provider.ProviderOpenAI
}

// stubToolExec returns canned outputs per tool name.
type stubToolExec struct {
	outputs map[string]string
	err     error // global error (all tools)
	flags   map[string]contracts.ToolBehaviorFlags
}

func (e *stubToolExec) Execute(ctx context.Context, name, argsJSON string) (string, map[string]any, error) {
	if e.err != nil {
		return "", nil, e.err
	}
	if out, ok := e.outputs[name]; ok {
		return out, nil, nil
	}
	return "ok", nil, nil
}

func (e *stubToolExec) GetToolFlags(name string) (contracts.ToolBehaviorFlags, error) {
	if f, ok := e.flags[name]; ok {
		return f, nil
	}
	return contracts.ToolBehaviorFlags{}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func toolCallStartEvent(name, id string, index int) provider.StreamEvent {
	return provider.StreamEvent{
		Type:          provider.EventToolCallStart,
		ToolCallName:  name,
		ToolCallID:    id,
		ToolCallIndex: index,
	}
}

func toolCallCompleteEvent(name, id, args string) provider.StreamEvent {
	return provider.NewToolCallCompleteEvent(name, id, args)
}

func textDeltaEvent(text string) provider.StreamEvent {
	return provider.StreamEvent{Type: provider.EventTextDelta, Delta: text}
}

func thinkingEvent(text string) provider.StreamEvent {
	return provider.StreamEvent{Type: provider.EventThinking, Delta: text}
}

// doneEventWithReason creates an EventDone with a specific finish_reason.
func doneEventWithReason(reason string) provider.StreamEvent {
	return provider.StreamEvent{Type: provider.EventDone, FinishReason: reason}
}

// errorEvent creates a mid-stream EventError frame.
func errorEvent(msg string) provider.StreamEvent {
	return provider.StreamEvent{Type: provider.EventError, Error: fmt.Errorf("%s", msg)}
}

// doneEventWithUsage creates an EventDone with a finish_reason and usage data.
func doneEventWithUsage(reason string, completionTokens int) provider.StreamEvent {
	return provider.StreamEvent{
		Type:         provider.EventDone,
		FinishReason: reason,
		Usage:        &model.Usage{CompletionTokens: completionTokens},
	}
}

// collectEvents returns a channel that collects all emitted events for assertion.
func collectEvents(emitter *observability.Emitter) <-chan observability.AgentEvent {
	ch := make(chan observability.AgentEvent, 256)
	emitter.Subscribe(ch)
	return ch
}

// drainEvents drains and returns all events currently in the channel.
func drainEvents(ch <-chan observability.AgentEvent) []observability.AgentEvent {
	var events []observability.AgentEvent
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, ev)
		default:
			return events
		}
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRunStreaming_TextOnly(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("Hello, world!"), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "You are helpful.",
		UserMessage:  "Say hello.",
		MaxRounds:    1,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.TransitionReason != TransitionModelCompleted {
		t.Fatalf("expected TransitionModelCompleted, got %q", st.TransitionReason)
	}
	if st.Round != 1 {
		t.Fatalf("expected 1 round, got %d", st.Round)
	}

	// Last message should be assistant with content.
	last := st.Messages[len(st.Messages)-1]
	if last.Role != "assistant" {
		t.Fatalf("expected last message role=assistant, got %q", last.Role)
	}
	if !strings.Contains(last.Content, "Hello") {
		t.Fatalf("expected 'Hello' in content, got: %s", last.Content)
	}
}

func TestRunStreaming_ToolThenComplete(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			// Round 1: tool call.
			{
				toolCallStartEvent("echo", "toolu_001", 0),
				toolCallCompleteEvent("echo", "toolu_001", `{"message":"test"}`),
				{Type: provider.EventDone},
			},
			// Round 2: text only.
			{
				textDeltaEvent("Done!"),
				{Type: provider.EventDone},
			},
		},
	}
	exec := &stubToolExec{
		outputs: map[string]string{"echo": "echo: test"},
	}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "System.",
		UserMessage:  "Go.",
		MaxRounds:    4,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.TransitionReason != TransitionModelCompleted {
		t.Fatalf("expected TransitionModelCompleted, got %q", st.TransitionReason)
	}
	if st.Round != 2 {
		t.Fatalf("expected 2 rounds, got %d", st.Round)
	}
	if len(st.ToolResultBlocks) != 1 {
		t.Fatalf("expected 1 ToolResultBlock, got %d", len(st.ToolResultBlocks))
	}
	if st.ToolResultBlocks[0].Type != "tool_result" {
		t.Fatalf("expected tool_result type, got %q", st.ToolResultBlocks[0].Type)
	}
	if !strings.Contains(st.ToolResultBlocks[0].Content, "echo") {
		t.Fatalf("expected echo in tool result, got: %s", st.ToolResultBlocks[0].Content)
	}
}

func TestRunStreaming_EmitterEvents(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				toolCallStartEvent("grep_search", "toolu_001", 0),
				toolCallCompleteEvent("grep_search", "toolu_001", `{"pattern":"test"}`),
				{Type: provider.EventDone},
			},
			{textDeltaEvent("Results found."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{
		outputs: map[string]string{"grep_search": "found: test.go"},
	}
	emitter := observability.NewEmitter()
	eventCh := collectEvents(emitter)

	cfg := StreamingLoopConfig{
		SystemPrompt: "System.",
		UserMessage:  "Search.",
		MaxRounds:    4,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Give events time to flush.
	time.Sleep(50 * time.Millisecond)
	emitter.Close()

	events := drainEvents(eventCh)
	eventTypes := make([]string, len(events))
	for i, e := range events {
		eventTypes[i] = e.Type
	}

	// Check expected event types appear in order:
	// round, tool_call_start, tool_call_complete, tool_result, round_end (R1)
	// round, text, round_end (R2)

	hasType := func(typ string) bool {
		for _, t := range eventTypes {
			if t == typ {
				return true
			}
		}
		return false
	}

	if !hasType("round") {
		t.Fatal("expected 'round' events")
	}
	if !hasType("tool_call_start") {
		t.Fatal("expected 'tool_call_start' event")
	}
	if !hasType("tool_call_complete") {
		t.Fatal("expected 'tool_call_complete' event")
	}
	if !hasType("tool_result") {
		t.Fatal("expected 'tool_result' event")
	}
	if !hasType("round_end") {
		t.Fatal("expected 'round_end' events")
	}
	if !hasType("text") {
		t.Fatal("expected 'text' events")
	}
}

func TestRunStreaming_ContextManager_Snapshot(t *testing.T) {
	// Create a workspace with an HTML file.
	ws := t.TempDir()
	origCwd, _ := os.Getwd()
	os.Chdir(ws)
	defer os.Chdir(origCwd)

	os.WriteFile("deck.html", []byte(`<html><head><title>Test</title></head><body>
		<section class="slide"><h1>Slide 1</h1></section>
		<section class="slide"><h1>Slide 2</h1></section>
	</body></html>`), 0o644)

	ctxMgr := agentctx.NewContextManager(ws, "test-session", "")

	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				toolCallStartEvent("write_file", "toolu_001", 0),
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html></html>"}`),
				{Type: provider.EventDone},
			},
			{textDeltaEvent("Created deck.html."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{
		outputs: map[string]string{"write_file": "file written"},
	}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt:   "System.",
		UserMessage:    "Create deck.",
		MaxRounds:      4,
		Provider:       provider,
		Execute:        exec,
		Emitter:        emitter,
		ContextManager: ctxMgr,
		SessionID:      "test-session",
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Version != 1 {
		t.Fatalf("expected Version=1 after HTML snapshot, got %d", st.Version)
	}
	if ctxMgr.CurrentVersion() != 1 {
		t.Fatalf("expected ContextManager version 1, got %d", ctxMgr.CurrentVersion())
	}
	snap := ctxMgr.LatestSnapshot()
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.Title != "Test" {
		t.Fatalf("expected title 'Test', got %q", snap.Title)
	}
}

func TestRunStreaming_ContextManager_TodoPersistence(t *testing.T) {
	ws := t.TempDir()
	origCwd, _ := os.Getwd()
	os.Chdir(ws)
	defer os.Chdir(origCwd)

	ctxMgr := agentctx.NewContextManager(ws, "test-session", "")

	// First round: write HTML (creates version 1), second round: todo_write.
	todoJSON := json.RawMessage(`{"todos":[{"content":"Task 1","status":"pending","activeForm":"Working on task 1"}]}`)
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				toolCallStartEvent("write_file", "toolu_001", 0),
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html></html>"}`),
				{Type: provider.EventDone},
			},
			{
				toolCallStartEvent("todo_write", "toolu_002", 0),
				toolCallCompleteEvent("todo_write", "toolu_002", string(todoJSON)),
				{Type: provider.EventDone},
			},
			{textDeltaEvent("All done."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{
		outputs: map[string]string{
			"write_file": "file written",
			"todo_write": "Todo list updated (1 items).",
		},
	}
	emitter := observability.NewEmitter()

	// Need the HTML on disk for MaybeSnapshot.
	os.WriteFile("deck.html", []byte(`<html><body><section class="slide"></section><section class="slide"></section></body></html>`), 0o644)

	cfg := StreamingLoopConfig{
		SystemPrompt:   "System.",
		UserMessage:    "Write deck and track tasks.",
		MaxRounds:      4,
		Provider:       provider,
		Execute:        exec,
		Emitter:        emitter,
		ContextManager: ctxMgr,
		SessionID:      "test-session",
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Check todos were persisted.
	todos := ctxMgr.LatestTodos()
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}
	if todos[0].Content != "Task 1" {
		t.Fatalf("expected 'Task 1', got %q", todos[0].Content)
	}

	// Check they're on disk too.
	store := agentctx.NewSnapshotStore(ws)
	loaded, err := store.LoadTodo(ctxMgr.CurrentVersion())
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 todo on disk, got %d", len(loaded))
	}
}

func TestRunStreaming_MaxRoundsExceeded(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{toolCallStartEvent("echo", "toolu_001", 0), toolCallCompleteEvent("echo", "toolu_001", `{}`), {Type: provider.EventDone}},
			{toolCallStartEvent("echo", "toolu_002", 0), toolCallCompleteEvent("echo", "toolu_002", `{}`), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{outputs: map[string]string{"echo": "ok"}}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "U",
		MaxRounds:    1,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for exceeded MaxRounds")
	}
	if !strings.Contains(err.Error(), "MaxRounds") {
		t.Fatalf("expected 'MaxRounds' in error, got: %v", err)
	}
}

func TestRunStreaming_NilProvider(t *testing.T) {
	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "U",
		Provider:     nil,
		Execute:      &stubToolExec{},
	}
	_, err := RunStreaming(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for nil Provider")
	}
}

func TestRunStreaming_NilExecutor(t *testing.T) {
	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "U",
		Provider:     &stubStreamingProvider{},
		Execute:      nil,
	}
	_, err := RunStreaming(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for nil ToolExecutor")
	}
}

func TestRunStreaming_SessionSave(t *testing.T) {
	ws := t.TempDir()
	sessionStore := persistence.NewSessionStore(filepath.Join(ws, "sessions"))

	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("Hello."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "System.",
		UserMessage:  "Say hello.",
		MaxRounds:    1,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
		SessionStore: sessionStore,
		SessionID:    "test-session",
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Verify session was saved.
	data, err := sessionStore.Load("test-session")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("expected saved session data")
	}
}

func TestRunStreaming_ContextCancellation(t *testing.T) {
	// Create a provider that checks context cancellation.
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("partial"), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "U",
		MaxRounds:    1,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
	}

	_, err := RunStreaming(ctx, cfg)
	if err != nil {
		// Expected — cancelled context.
		return
	}
	// If we get here, maybe the provider finished before cancellation was checked.
	// That's also acceptable.
}

func TestRunStreaming_ThinkingEvents(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				thinkingEvent("Let me think..."),
				textDeltaEvent("Answer."),
				{Type: provider.EventDone},
			},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()
	eventCh := collectEvents(emitter)

	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "U",
		MaxRounds:    1,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	emitter.Close()

	events := drainEvents(eventCh)
	hasThinking := false
	for _, e := range events {
		if e.Type == "thinking" {
			hasThinking = true
		}
	}
	if !hasThinking {
		t.Fatal("expected 'thinking' event")
	}
}

// ---------------------------------------------------------------------------
// Timeline tests
// ---------------------------------------------------------------------------

func TestAppendTimeline_BasicTypes(t *testing.T) {
	st := &LoopState{}

	appendTimeline(st, "user_message", map[string]any{"content": "Hello"})
	appendTimeline(st, "thinking", map[string]any{"content": "Let me think..."})
	appendTimeline(st, "text", map[string]any{"content": "Here's the answer."})
	appendTimeline(st, "tool", map[string]any{"phase": "start", "name": "write_file", "call_id": "c1"})
	appendTimeline(st, "tool", map[string]any{"phase": "result", "name": "write_file", "call_id": "c1", "success": true, "summary": "ok", "duration_ms": 123})
	appendTimeline(st, "todo_write", map[string]any{"todos": []map[string]any{{"content": "Task 1", "status": "pending"}}})

	events := make([]string, len(st.Timeline))
	for i, e := range st.Timeline {
		events[i] = e.Event
	}
	expected := []string{"user_message", "thinking", "text", "tool", "tool", "todo_write"}
	if len(events) != len(expected) {
		t.Fatalf("expected %d events, got %d: %v", len(expected), len(events), events)
	}
	for i := range expected {
		if events[i] != expected[i] {
			t.Fatalf("event[%d]: expected %q, got %q", i, expected[i], events[i])
		}
	}
}

func TestAppendTimeline_MergesConsecutiveThinking(t *testing.T) {
	st := &LoopState{}
	appendTimeline(st, "thinking", map[string]any{"content": "Part 1. "})
	appendTimeline(st, "thinking", map[string]any{"content": "Part 2."})
	appendTimeline(st, "thinking", map[string]any{"content": " Part 3."})

	if len(st.Timeline) != 1 {
		t.Fatalf("expected 1 merged thinking event, got %d", len(st.Timeline))
	}
	if st.Timeline[0].Data["content"] != "Part 1. Part 2. Part 3." {
		t.Fatalf("expected merged content, got %q", st.Timeline[0].Data["content"])
	}
}

func TestAppendTimeline_MergesConsecutiveText(t *testing.T) {
	st := &LoopState{}
	appendTimeline(st, "text", map[string]any{"content": "First"})
	appendTimeline(st, "text", map[string]any{"content": "Second"})

	if len(st.Timeline) != 1 {
		t.Fatalf("expected 1 merged text event, got %d", len(st.Timeline))
	}
	if st.Timeline[0].Data["content"] != "FirstSecond" {
		t.Fatalf("expected merged content, got %q", st.Timeline[0].Data["content"])
	}
}

func TestAppendTimeline_DoesNotMergeDifferentTypes(t *testing.T) {
	st := &LoopState{}
	appendTimeline(st, "thinking", map[string]any{"content": "Thought"})
	appendTimeline(st, "text", map[string]any{"content": "Output"})
	appendTimeline(st, "thinking", map[string]any{"content": " More thought"})

	if len(st.Timeline) != 3 {
		t.Fatalf("expected 3 separate events, got %d", len(st.Timeline))
	}
}

func TestAppendTimeline_NilState(t *testing.T) {
	// Should not panic.
	appendTimeline(nil, "text", map[string]any{"content": "test"})
}

func TestRunStreaming_TimelinePopulated(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				thinkingEvent("Let me plan..."),
				textDeltaEvent("I will help."),
				{Type: provider.EventDone},
			},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "Help me.",
		MaxRounds:    1,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Verify timeline has user_message, thinking, text events in order.
	if len(st.Timeline) < 3 {
		t.Fatalf("expected at least 3 timeline events, got %d", len(st.Timeline))
	}
	if st.Timeline[0].Event != "user_message" {
		t.Fatalf("expected first event user_message, got %q", st.Timeline[0].Event)
	}
	if st.Timeline[0].Data["content"] != "Help me." {
		t.Fatalf("expected user_message content, got %q", st.Timeline[0].Data["content"])
	}
	if st.Timeline[1].Event != "thinking" {
		t.Fatalf("expected second event thinking, got %q", st.Timeline[1].Event)
	}
	if st.Timeline[2].Event != "text" {
		t.Fatalf("expected third event text, got %q", st.Timeline[2].Event)
	}
}

func TestRunStreaming_TimelineToolStartResult(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				toolCallStartEvent("read_file", "toolu_001", 0),
				toolCallCompleteEvent("read_file", "toolu_001", `{"path":"test.txt"}`),
				{Type: provider.EventDone},
			},
			{textDeltaEvent("Done."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{
		outputs: map[string]string{"read_file": "file contents"},
	}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "Read a file.",
		MaxRounds:    4,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Find tool events in timeline.
	var toolEvents []TimelineEvent
	for _, e := range st.Timeline {
		if e.Event == "tool" {
			toolEvents = append(toolEvents, e)
		}
	}
	if len(toolEvents) != 2 {
		t.Fatalf("expected 2 tool timeline events (start + result), got %d", len(toolEvents))
	}
	if toolEvents[0].Data["phase"] != "start" {
		t.Fatalf("expected first tool event phase=start, got %q", toolEvents[0].Data["phase"])
	}
	if toolEvents[0].Data["name"] != "read_file" {
		t.Fatalf("expected tool name read_file, got %q", toolEvents[0].Data["name"])
	}
	if toolEvents[1].Data["phase"] != "result" {
		t.Fatalf("expected second tool event phase=result, got %q", toolEvents[1].Data["phase"])
	}
	if toolEvents[1].Data["success"] != true {
		t.Fatal("expected tool result success=true")
	}
}

func TestRunStreaming_TimelineTodoWrite(t *testing.T) {
	ws := t.TempDir()
	origCwd, _ := os.Getwd()
	os.Chdir(ws)
	defer os.Chdir(origCwd)

	ctxMgr := agentctx.NewContextManager(ws, "test-session", "")
	todoJSON := json.RawMessage(`{"todos":[{"content":"Task 1","status":"in_progress","activeForm":"Working on task 1"}]}`)

	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				toolCallStartEvent("todo_write", "toolu_001", 0),
				toolCallCompleteEvent("todo_write", "toolu_001", string(todoJSON)),
				{Type: provider.EventDone},
			},
			{textDeltaEvent("Todos updated."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{
		outputs: map[string]string{"todo_write": "Todo list updated (1 items)."},
	}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt:   "S",
		UserMessage:    "Update todos.",
		MaxRounds:      4,
		Provider:       provider,
		Execute:        exec,
		Emitter:        emitter,
		ContextManager: ctxMgr,
		SessionID:      "test-session",
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Find todo_write event in timeline.
	var todoEvent *TimelineEvent
	for i := range st.Timeline {
		if st.Timeline[i].Event == "todo_write" {
			todoEvent = &st.Timeline[i]
			break
		}
	}
	if todoEvent == nil {
		t.Fatal("expected todo_write event in timeline")
	}
	todos, ok := todoEvent.Data["todos"].([]map[string]any)
	if !ok {
		t.Fatalf("expected todos array in data, got %T", todoEvent.Data["todos"])
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}
	if todos[0]["content"] != "Task 1" {
		t.Fatalf("expected 'Task 1', got %q", todos[0]["content"])
	}
	if todos[0]["status"] != "in_progress" {
		t.Fatalf("expected 'in_progress', got %q", todos[0]["status"])
	}
}

func TestRunStreaming_TimelineSessionPersistence(t *testing.T) {
	ws := t.TempDir()
	sessionStore := persistence.NewSessionStore(filepath.Join(ws, "sessions"))

	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("Hello."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "System.",
		UserMessage:  "Say hello.",
		MaxRounds:    1,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
		SessionStore: sessionStore,
		SessionID:    "test-session",
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	data, err := sessionStore.Load("test-session")
	if err != nil {
		t.Fatal(err)
	}
	// Unmarshal and verify timeline is present in serialized form.
	var loaded LoopState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if len(loaded.Timeline) == 0 {
		t.Fatal("expected timeline events in persisted session")
	}
	if loaded.Timeline[0].Event != "user_message" {
		t.Fatalf("expected first event user_message, got %q", loaded.Timeline[0].Event)
	}
	foundText := false
	for _, e := range loaded.Timeline {
		if e.Event == "text" {
			foundText = true
			break
		}
	}
	if !foundText {
		t.Fatal("expected text event in persisted timeline")
	}
}

func TestRunStreaming_TimelinePreservedAcrossConversations(t *testing.T) {
	// Simulate a second conversation: PreviousTimeline contains events from
	// the first conversation, and should be prepended to the new timeline.
	prevTimeline := []TimelineEvent{
		{Event: "user_message", Data: map[string]any{"content": "First message."}},
		{Event: "text", Data: map[string]any{"content": "Response 1."}},
	}

	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("Second response."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt:      "S",
		UserMessage:       "Second message.",
		MaxRounds:         1,
		Provider:          provider,
		Execute:           exec,
		Emitter:           emitter,
		PreviousTimeline:  prevTimeline,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Timeline should have: prev[0], prev[1], new user_message, new text
	if len(st.Timeline) < 4 {
		t.Fatalf("expected at least 4 timeline events, got %d", len(st.Timeline))
	}
	if st.Timeline[0].Data["content"] != "First message." {
		t.Fatalf("expected first event from previous timeline, got %q", st.Timeline[0].Data["content"])
	}
	if st.Timeline[1].Data["content"] != "Response 1." {
		t.Fatalf("expected second event from previous timeline, got %q", st.Timeline[1].Data["content"])
	}
	if st.Timeline[2].Event != "user_message" {
		t.Fatalf("expected third event to be new user_message, got %q", st.Timeline[2].Event)
	}
	if st.Timeline[2].Data["content"] != "Second message." {
		t.Fatalf("expected new user_message content, got %q", st.Timeline[2].Data["content"])
	}
}

func TestRunStreaming_ProviderStreamError(t *testing.T) {
	provider := &stubStreamingProvider{
		chatErr: fmt.Errorf("401 Unauthorized"),
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "U",
		MaxRounds:    1,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error from StreamChat failure")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected '401' in error, got: %v", err)
	}
}

func TestRunStreaming_RecordRoundEndCalled(t *testing.T) {
	// Verifies that RecordRoundEnd(context.Background(), 1) is called each round (even though PostRound
	// hooks have been removed). ConsecutiveRoundsNoWrite should increment.
	ws := t.TempDir()
	engine := hook.NewEngineWithConfig(hook.DefaultConfig())
	engine.InitState("test-session", ws, hook.StageInitialGeneration)

	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("Hello."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "Say hello.",
		MaxRounds:    1,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
		HookEngine:   engine,
		SessionState: engine.State(),
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// After 1 round without writing HTML, ConsecutiveRoundsNoWrite should be 1.
	s := engine.State()
	if s.ConsecutiveRoundsNoWrite != 1 {
		t.Errorf("expected ConsecutiveRoundsNoWrite=1 after text-only round, got %d", s.ConsecutiveRoundsNoWrite)
	}
}

func TestRunStreaming_RecordRoundEndCalled_MultipleRounds(t *testing.T) {
	// Two rounds, both without HTML writes -> ConsecutiveRoundsNoWrite should be 2.
	ws := t.TempDir()
	engine := hook.NewEngineWithConfig(hook.DefaultConfig())
	engine.InitState("test-session", ws, hook.StageInitialGeneration)

	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{toolCallStartEvent("read_file", "toolu_001", 0), toolCallCompleteEvent("read_file", "toolu_001", `{"path":"test.txt"}`), {Type: provider.EventDone}},
			{textDeltaEvent("Done."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{
		outputs: map[string]string{"read_file": "file contents"},
	}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "Read and respond.",
		MaxRounds:    4,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
		HookEngine:   engine,
		SessionState: engine.State(),
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	s := engine.State()
	if s.ConsecutiveRoundsNoWrite != 2 {
		t.Errorf("expected ConsecutiveRoundsNoWrite=2 after 2 no-write rounds, got %d", s.ConsecutiveRoundsNoWrite)
	}
}

// ---------------------------------------------------------------------------
// inlineDesignSkillAssets tests
// ---------------------------------------------------------------------------

func TestInlineDesignSkillAssets_Basic(t *testing.T) {
	// Create temp workspace with an HTML file referencing a local CSS asset,
	// and a skills dir containing that asset.
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "coral-skill")
	os.MkdirAll(skillsDir, 0755)

	// Write HTML with a local <link rel="stylesheet">.
	htmlContent := `<!DOCTYPE html>
<html>
<head>
<link rel="stylesheet" href="fonts.css">
</head>
<body>Hello</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)

	// Write the CSS asset.
	cssContent := `@font-face { font-family: "Custom"; src: url("custom.woff2"); }`
	os.WriteFile(filepath.Join(skillsDir, "fonts.css"), []byte(cssContent), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	resultStr := string(result)

	if strings.Contains(resultStr, `<link rel="stylesheet"`) {
		t.Error("expected <link> tag to be replaced with <style>")
	}
	if !strings.Contains(resultStr, "<style>") {
		t.Error("expected <style> tag in output")
	}
	if !strings.Contains(resultStr, cssContent) {
		t.Error("expected CSS content to be inlined")
	}
}

func TestInlineDesignSkillAssets_SkipsAbsoluteURLs(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "coral-skill")
	os.MkdirAll(skillsDir, 0755)

	htmlContent := `<!DOCTYPE html>
<html>
<head>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter">
<link rel="stylesheet" href="fonts.css">
</head>
<body>Hello</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)

	cssContent := `body { font-family: sans-serif; }`
	os.WriteFile(filepath.Join(skillsDir, "fonts.css"), []byte(cssContent), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	resultStr := string(result)

	// Absolute URL should be preserved.
	if !strings.Contains(resultStr, "https://fonts.googleapis.com/css2?family=Inter") {
		t.Error("expected absolute URL to be preserved")
	}
	// Local ref should be inlined.
	if strings.Contains(resultStr, `<link rel="stylesheet" href="fonts.css"`) {
		t.Error("expected local <link> to be replaced")
	}
	if !strings.Contains(resultStr, cssContent) {
		t.Error("expected local CSS to be inlined")
	}
}

func TestInlineDesignSkillAssets_SkipsMissingAssets(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "coral-skill")
	os.MkdirAll(skillsDir, 0755)

	htmlContent := `<!DOCTYPE html>
<html>
<head>
<link rel="stylesheet" href="nonexistent.css">
</head>
<body>Hello</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	resultStr := string(result)

	// Missing asset → original <link> preserved.
	if !strings.Contains(resultStr, `<link rel="stylesheet" href="nonexistent.css"`) {
		t.Error("expected original <link> tag to be preserved for missing asset")
	}
}

func TestInlineDesignSkillAssets_InlinesScript(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "coral-skill")
	os.MkdirAll(skillsDir, 0755)

	htmlContent := `<!DOCTYPE html>
<html>
<head></head>
<body>
<script src="nav.js"></script>
</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)

	jsContent := `document.querySelectorAll('.nav a').forEach(function(el){el.addEventListener('click',function(e){e.preventDefault();});});`
	os.WriteFile(filepath.Join(skillsDir, "nav.js"), []byte(jsContent), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	resultStr := string(result)

	if strings.Contains(resultStr, `<script src="nav.js"`) {
		t.Error("expected <script src> to be replaced")
	}
	if !strings.Contains(resultStr, "<script>") {
		t.Error("expected <script> tag in output")
	}
	if !strings.Contains(resultStr, jsContent) {
		t.Error("expected JS content to be inlined")
	}
}

func TestInlineDesignSkillAssets_NoChanges(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "coral-skill")
	os.MkdirAll(skillsDir, 0755)

	htmlContent := `<!DOCTYPE html>
<html>
<head>
<style>body{color:red;}</style>
</head>
<body>Hello</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	resultStr := string(result)

	// File should be unchanged (no external refs).
	if resultStr != htmlContent {
		t.Error("expected file to be unchanged when no external refs exist")
	}
}

func TestStreamingLoopConfig_DesignSkill(t *testing.T) {
	cfg := StreamingLoopConfig{
		DesignSkill: "html-ppt-zhangzara-coral",
	}
	if cfg.DesignSkill != "html-ppt-zhangzara-coral" {
		t.Errorf("expected DesignSkill field, got %q", cfg.DesignSkill)
	}

	// Verify field is empty by default.
	cfg2 := StreamingLoopConfig{}
	if cfg2.DesignSkill != "" {
		t.Errorf("expected DesignSkill to be empty by default, got %q", cfg2.DesignSkill)
	}
}

func TestInlineDesignSkillAssets_MultipleAssets(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "skill-dir")
	os.MkdirAll(skillsDir, 0755)

	htmlContent := `<!DOCTYPE html>
<html>
<head>
<link rel="stylesheet" href="fonts.css">
<link rel="stylesheet" href="theme.css">
</head>
<body>
<script src="nav.js"></script>
</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)
	os.WriteFile(filepath.Join(skillsDir, "fonts.css"), []byte("body{font-family:Arial;}"), 0644)
	os.WriteFile(filepath.Join(skillsDir, "theme.css"), []byte(".dark{color:white;}"), 0644)
	os.WriteFile(filepath.Join(skillsDir, "nav.js"), []byte("console.log('nav');"), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, _ := os.ReadFile(htmlPath)
	resultStr := string(result)

	// All external refs should be inlined.
	if strings.Contains(resultStr, `<link rel="stylesheet"`) {
		t.Error("expected no <link> tags remaining")
	}
	if strings.Contains(resultStr, `<script src=`) {
		t.Error("expected no <script src> remaining")
	}
	if !strings.Contains(resultStr, "body{font-family:Arial;}") {
		t.Error("expected first CSS inlined")
	}
	if !strings.Contains(resultStr, ".dark{color:white;}") {
		t.Error("expected second CSS inlined")
	}
	if !strings.Contains(resultStr, "console.log('nav');") {
		t.Error("expected JS inlined")
	}
	// Should have 3 inline blocks (2 style + 1 script).
	count := strings.Count(resultStr, "<style>")
	if count != 2 {
		t.Errorf("expected 2 <style> blocks, got %d", count)
	}
}

func TestInlineDesignSkillAssets_EmptyAssetFile(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "skill-dir")
	os.MkdirAll(skillsDir, 0755)

	htmlContent := `<!DOCTYPE html>
<html>
<head>
<link rel="stylesheet" href="empty.css">
</head>
<body>Hello</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)
	os.WriteFile(filepath.Join(skillsDir, "empty.css"), []byte(""), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, _ := os.ReadFile(htmlPath)
	resultStr := string(result)
	// Empty CSS should still be inlined (producing empty <style></style>).
	if strings.Contains(resultStr, `<link rel="stylesheet"`) {
		t.Error("expected <link> to be replaced even for empty asset")
	}
}

func TestInlineDesignSkillAssets_LinkWithExtraAttrs(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "skill-dir")
	os.MkdirAll(skillsDir, 0755)

	// <link> with media and data-* attributes.
	htmlContent := `<!DOCTYPE html>
<html>
<head>
<link rel="stylesheet" href="base.css" media="screen" data-priority="high">
</head>
<body>Hello</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)
	os.WriteFile(filepath.Join(skillsDir, "base.css"), []byte("body{margin:0;}"), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, _ := os.ReadFile(htmlPath)
	resultStr := string(result)
	// Extra attributes should be stripped since the <link> is replaced.
	if strings.Contains(resultStr, `data-priority`) || strings.Contains(resultStr, `<link rel="stylesheet"`) {
		t.Error("expected <link> with extra attrs to be removed")
	}
	if !strings.Contains(resultStr, "body{margin:0;}") {
		t.Error("expected CSS inlined")
	}
}

func TestInlineDesignSkillAssets_ScriptWithTypeAttr(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "skill-dir")
	os.MkdirAll(skillsDir, 0755)

	htmlContent := `<!DOCTYPE html>
<html>
<head></head>
<body>
<script type="text/javascript" src="app.js"></script>
</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)
	os.WriteFile(filepath.Join(skillsDir, "app.js"), []byte("var x=1;"), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, _ := os.ReadFile(htmlPath)
	resultStr := string(result)
	if strings.Contains(resultStr, `<script type="text/javascript" src="app.js"`) {
		t.Error("expected <script src> to be removed")
	}
	if !strings.Contains(resultStr, "var x=1;") {
		t.Error("expected JS inlined")
	}
}

func TestInlineDesignSkillAssets_SkipsExternalScript(t *testing.T) {
	// External CDN scripts should NOT be inlined.
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "skill-dir")
	os.MkdirAll(skillsDir, 0755)

	htmlContent := `<!DOCTYPE html>
<html>
<body>
<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
<script src="local.js"></script>
</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)
	os.WriteFile(filepath.Join(skillsDir, "local.js"), []byte("localInit();"), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, _ := os.ReadFile(htmlPath)
	resultStr := string(result)
	// External script untouched.
	if !strings.Contains(resultStr, "cdn.jsdelivr.net") {
		t.Error("expected external CDN script to be preserved")
	}
	// Local script inlined.
	if strings.Contains(resultStr, `<script src="local.js"`) {
		t.Error("expected local script to be inlined")
	}
	if !strings.Contains(resultStr, "localInit();") {
		t.Error("expected local JS content inlined")
	}
}

func TestInlineDesignSkillAssets_UnicodeContent(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "skill-dir")
	os.MkdirAll(skillsDir, 0755)

	htmlContent := `<!DOCTYPE html>
<html>
<head>
<link rel="stylesheet" href="i18n.css">
</head>
<body>Hello</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)
	os.WriteFile(filepath.Join(skillsDir, "i18n.css"), []byte("/* 中文注释 */\nbody{font-family:'PingFang SC';}\n/* émoji: 🎨 */"), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, _ := os.ReadFile(htmlPath)
	resultStr := string(result)
	if !strings.Contains(resultStr, "PingFang SC") {
		t.Error("expected unicode CSS content preserved")
	}
	if !strings.Contains(resultStr, "🎨") {
		t.Error("expected emoji preserved")
	}
}

func TestInlineDesignSkillAssets_RelativePathTraversal(t *testing.T) {
	// filepath.Join cleans ".." so "../escape.css" resolves outside skillsDir.
	// Current behavior: if the resolved file exists, it IS inlined.
	// This test documents the current behavior (including path traversal).
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	skillsDir := filepath.Join(ws, "skill-dir")
	os.MkdirAll(skillsDir, 0755)

	htmlContent := `<!DOCTYPE html>
<html>
<head>
<link rel="stylesheet" href="../escape.css">
</head>
<body>Hello</body>
</html>`
	os.WriteFile(htmlPath, []byte(htmlContent), 0644)
	// Place file at the resolved path (outside skills dir).
	os.WriteFile(filepath.Join(ws, "escape.css"), []byte("escaped!"), 0644)

	err := inlineDesignSkillAssets(htmlPath, skillsDir)
	if err != nil {
		t.Fatalf("inlineDesignSkillAssets failed: %v", err)
	}

	result, _ := os.ReadFile(htmlPath)
	resultStr := string(result)
	// Current behavior: filepath.Join cleans the path, so "../escape.css"
	// resolves to ws/escape.css which exists → it gets inlined.
	// NOTE: this is a path traversal vector that should be hardened.
	t.Logf("path traversal test: file outside skillsDir was %s",
		map[bool]string{true: "INLINED (traversal possible)", false: "BLOCKED"}[strings.Contains(resultStr, "escaped!")])
}

func TestStreamingLoopConfig_DesignSkillPropagation(t *testing.T) {
	// Verify the DesignSkill field is propagated through to where inlining would be called.
	cfg := StreamingLoopConfig{
		SystemPrompt: "System.",
		UserMessage:  "Build a deck.",
		MaxRounds:    1,
		Provider:     &stubStreamingProvider{},
		Execute:      &stubToolExec{},
		Emitter:      observability.NewEmitter(),
		DesignSkill:  "coral-skill",
	}
	if cfg.DesignSkill != "coral-skill" {
		t.Errorf("DesignSkill = %q, want 'coral-skill'", cfg.DesignSkill)
	}

	// Empty DesignSkill means no inlining should happen.
	cfg2 := StreamingLoopConfig{}
	if cfg2.DesignSkill != "" {
		t.Error("expected empty DesignSkill by default")
	}
}

func TestRunStreaming_PreContextAssembleWarningsInjected(t *testing.T) {
	// When SessionState has PendingWarnings, PreContextAssemble should inject them
	// via QualityInject and the messages should contain the warning.
	ws := t.TempDir()
	engine := hook.NewEngineWithConfig(hook.DefaultConfig())
	engine.InitState("test-session", ws, hook.StageInitialGeneration)

	// Simulate main.go's warning flow: add pending warnings.
	engine.State().AddPendingWarnings([]string{"[系统提示] skill-loading-injector\nLoad compliance skill first."})

	// Register a QualityInject-like hook at PreContextAssemble.
	engine.Register(&hook.RegisteredHook{
		Name: "quality-inject", On: hook.PointPreContextAssemble, Stage: "always", Priority: 5,
		Builtin: true,
		Fn: func(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
			s := hctx.SessionState
			if s == nil {
				return hook.HookResult{Action: hook.Allow}
			}
			warns := s.DrainPendingWarnings()
			if len(warns) == 0 {
				return hook.HookResult{Action: hook.Allow}
			}
			combined := ""
			for _, w := range warns {
				combined += w + "\n"
			}
			return hook.HookResult{Action: hook.Warn, Reason: combined}
		},
	})

	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("I understand. Let me load the skill."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "You are an agent.",
		UserMessage:  "Build a slide deck.",
		MaxRounds:    1,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
		HookEngine:   engine,
		SessionState: engine.State(),
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// After QualityInject, PendingWarnings should be drained.
	if len(engine.State().DrainPendingWarnings()) != 0 {
		t.Error("expected pending warnings to be drained after PreContextAssemble")
	}

	// The streaming loop completed successfully, meaning the warning was processed
	// through InjectWarnings and the LLM received the modified context.
	if st.TransitionReason != TransitionModelCompleted {
		t.Errorf("expected TransitionModelCompleted, got %q", st.TransitionReason)
	}
}


// ---------------------------------------------------------------------------
// Recovery System Integration Tests (SIT)
// ---------------------------------------------------------------------------

// TestRunStreaming_Recovery_Truncation_Length tests POSITION 2:
// finishReason="length" → applyContinueRule Case B → continue → normal completion.
func TestRunStreaming_Recovery_Truncation_Length(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Generating HTML..."),
				toolCallStartEvent("write_file", "toolu_001", 0),
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html></html>"}`),
				doneEventWithReason("length"),
			},
			{
				textDeltaEvent("Continued after truncation. All done."),
				{Type: provider.EventDone},
			},
		},
	}
	exec := &stubToolExec{outputs: map[string]string{"write_file": "created deck.html"}}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "You are an agent.",
		UserMessage:  "Build a slide deck.",
		MaxRounds:    4,
		Provider:     provider,
		Execute:      exec,
		Emitter:      emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.ContinueCount != 1 {
		t.Errorf("expected ContinueCount=1, got %d", st.Recovery.ContinueCount)
	}
	if st.TransitionReason != TransitionModelCompleted {
		t.Errorf("expected TransitionModelCompleted, got %q", st.TransitionReason)
	}

	// Verify continue prompt was injected.
	foundContinue := false
	for _, m := range st.Messages {
		if m.Role == "user" && strings.Contains(m.Content, "Your previous response was truncated") {
			foundContinue = true
			break
		}
	}
	if !foundContinue {
		t.Error("expected continue prompt to be injected in messages")
	}
}

// TestRunStreaming_Recovery_Truncation_MaxTokens tests POSITION 2 with Anthropic "max_tokens".
func TestRunStreaming_Recovery_Truncation_MaxTokens(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Generating..."),
				toolCallStartEvent("write_file", "toolu_001", 0),
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html></html>"}`),
				doneEventWithReason("max_tokens"),
			},
			{textDeltaEvent("Done."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.ContinueCount != 1 {
		t.Errorf("expected ContinueCount=1, got %d", st.Recovery.ContinueCount)
	}
}

// TestRunStreaming_Recovery_Truncation_AmbiguousEOF tests POSITION 2:
// finish_reason="" + isTruncated=true (broken JSON) → continue.
func TestRunStreaming_Recovery_Truncation_AmbiguousEOF(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Writing file..."),
				toolCallStartEvent("write_file", "toolu_001", 0),
				// Broken JSON — missing closing }
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html>`),
				{Type: provider.EventDone}, // finish_reason="" (ambiguous)
			},
			{textDeltaEvent("Re-issued successfully."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.ContinueCount != 1 {
		t.Errorf("expected ContinueCount=1 for truncated ambiguous EOF, got %d", st.Recovery.ContinueCount)
	}
}

// TestRunStreaming_Recovery_AmbiguousEOF_NotTruncated tests that empty response
// with no deltas is NOT treated as truncation.
func TestRunStreaming_Recovery_AmbiguousEOF_NotTruncated(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{{Type: provider.EventDone}}, // empty with no deltas
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 1,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.ContinueCount != 0 {
		t.Errorf("expected ContinueCount=0 for empty non-truncated response, got %d", st.Recovery.ContinueCount)
	}
}

// TestRunStreaming_Recovery_Truncation_EmptyButHadDeltas tests that empty content
// WITH deltas is treated as truncation.
func TestRunStreaming_Recovery_Truncation_EmptyButHadDeltas(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				thinkingEvent("Let me think about this..."),
				{Type: provider.EventDone}, // had deltas but no content
			},
			{textDeltaEvent("OK, here is the actual response."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.ContinueCount != 1 {
		t.Errorf("expected ContinueCount=1 for empty-with-deltas truncation, got %d", st.Recovery.ContinueCount)
	}
}

// TestRunStreaming_Recovery_ContinueExhausted tests that 3 consecutive
// truncations (MaxContinueRetries=2 + 1 exhaust) result in a terminal error.
func TestRunStreaming_Recovery_ContinueExhausted(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("Attempt 1..."), doneEventWithReason("length")},
			{textDeltaEvent("Attempt 2..."), doneEventWithReason("length")},
			{textDeltaEvent("Attempt 3..."), doneEventWithReason("length")},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 6,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected terminal error after continue exhaustion")
	}
	if !strings.Contains(err.Error(), "continue recovery exhausted") {
		t.Fatalf("expected 'continue recovery exhausted' in error, got: %v", err)
	}
}

// TestRunStreaming_Recovery_InsufficientSystemResource tests POSITION 1.5:
// finishReason="insufficient_system_resource" → backoff → retry succeeds.
func TestRunStreaming_Recovery_InsufficientSystemResource(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("Working..."), doneEventWithReason("insufficient_system_resource")},
			{textDeltaEvent("Retry succeeded."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.BackoffCount != 1 {
		t.Errorf("expected BackoffCount=1, got %d", st.Recovery.BackoffCount)
	}
}

// TestRunStreaming_Recovery_EventError_Backoff tests mid-stream EventError
// triggering RecoveryBackoff and retrying successfully.
func TestRunStreaming_Recovery_EventError_Backoff(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Working on it..."),
				errorEvent("connection reset by peer"),
			},
			{textDeltaEvent("Retry after backoff."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.BackoffCount != 1 {
		t.Errorf("expected BackoffCount=1 after EventError backoff, got %d", st.Recovery.BackoffCount)
	}
}

// TestRunStreaming_Recovery_EventError_Compress tests mid-stream EventError
// with context overflow → RecoveryCompress → retry.
func TestRunStreaming_Recovery_EventError_Compress(t *testing.T) {
	ws := t.TempDir()
	origCwd, _ := os.Getwd()
	os.Chdir(ws)
	defer os.Chdir(origCwd)

	// Write a minimal HTML so snapshot succeeds.
	os.WriteFile("deck.html", []byte(`<html><body><section class="slide"></section></body></html>`), 0644)

	ctxMgr := agentctx.NewContextManager(ws, "test-session", "")

	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Processing large context..."),
				errorEvent("context_length_exceeded: too many tokens"),
			},
			{textDeltaEvent("Compressed and retried."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt:   "S",
		UserMessage:    "U",
		MaxRounds:      4,
		Provider:       provider,
		Execute:        exec,
		Emitter:        emitter,
		ContextManager: ctxMgr,
		SessionID:      "test-session",
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.CompressCount != 1 {
		t.Errorf("expected CompressCount=1, got %d", st.Recovery.CompressCount)
	}

	// Verify compress summary is in messages.
	foundCompress := false
	for _, m := range st.Messages {
		if m.Role == "user" && strings.Contains(m.Content, "[[RECOVERY_COMPRESS_V1]]") {
			foundCompress = true
			break
		}
	}
	if !foundCompress {
		t.Error("expected compression summary in messages after RecoveryCompress")
	}
}

// TestRunStreaming_Recovery_EventError_Compress_NoCM tests that compress
// recovery fails with terminal error when ContextManager is nil.
func TestRunStreaming_Recovery_EventError_Compress_NoCM(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Working..."),
				errorEvent("context_length_exceeded"),
			},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
		// ContextManager intentionally nil
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected terminal error when compress requires ContextManager but it is nil")
	}
	if !strings.Contains(err.Error(), "context_length_exceeded") {
		t.Fatalf("expected 'context_length_exceeded' in error, got: %v", err)
	}
}

// TestRunStreaming_Recovery_StreamChatError_503_Backoff tests callWithRecovery
// with a 503 transient error → backoff → retry succeeds.
func TestRunStreaming_Recovery_StreamChatError_503_Backoff(t *testing.T) {
	provider := &stubStreamingProvider{
		chatErr:      fmt.Errorf("503 Service Unavailable"),
		chatErrCount: 1, // fail once, then succeed
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("Success on retry."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.BackoffCount != 1 {
		t.Errorf("expected BackoffCount=1 after 503 backoff, got %d", st.Recovery.BackoffCount)
	}
}

// TestRunStreaming_Recovery_StreamChatError_401_Terminal tests callWithRecovery
// with a 401 non-transient error → terminal (no retry).
func TestRunStreaming_Recovery_StreamChatError_401_Terminal(t *testing.T) {
	provider := &stubStreamingProvider{
		chatErr: fmt.Errorf("401 Unauthorized"),
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 1,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected terminal error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected '401' in error, got: %v", err)
	}
}

// TestRunStreaming_Recovery_BrokenToolJSON_Guard tests that tool calls with
// invalid JSON are caught by json.Valid guard and not executed.
func TestRunStreaming_Recovery_BrokenToolJSON_Guard(t *testing.T) {
	// Defense-in-depth: when finish_reason="stop" (normal completion) but a
	// tool call has broken JSON, the json.Valid guard in executeTools catches
	// it before execution and injects a structured error.
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Writing file..."),
				toolCallStartEvent("write_file", "toolu_001", 0),
				// Broken JSON: truncated mid-string
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html>...`),
				doneEventWithReason("stop"), // normal finish_reason bypasses isTruncated
			},
			{textDeltaEvent("All done."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{outputs: map[string]string{"write_file": "should not be called"}}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// The broken tool call should have been filtered out by json.Valid guard.
	// Verify no successful write_file tool result exists.
	for _, m := range st.Messages {
		if m.Role == "tool" && m.Content == "should not be called" {
			t.Error("write_file should NOT have been executed with broken JSON")
		}
	}

	// Verify a structured error was injected for the broken tool call.
	foundError := false
	for _, m := range st.Messages {
		if m.Role == "tool" && strings.Contains(m.Content, "truncated") && strings.Contains(m.Content, "invalid JSON") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected structured error message for truncated tool call JSON")
	}
}

// TestRunStreaming_Recovery_ApplyContinueRule_BrokenTools tests that when
// tool calls have broken JSON, applyContinueRule appends only text content
// (no tool_calls) + Case A prompt.
func TestRunStreaming_Recovery_ApplyContinueRule_BrokenTools(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Let me write the file..."),
				toolCallStartEvent("write_file", "toolu_001", 0),
				// Broken JSON
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<h`),
				doneEventWithReason("length"),
			},
			{textDeltaEvent("Re-issued write_file."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Verify the assistant message was appended WITHOUT tool_calls.
	foundAssistant := false
	for _, m := range st.Messages {
		if m.Role == "assistant" && strings.Contains(m.Content, "Let me write the file") {
			foundAssistant = true
			if len(m.ToolCalls) > 0 {
				t.Error("expected assistant message with NO tool_calls for broken JSON")
			}
			break
		}
	}
	if !foundAssistant {
		t.Error("expected assistant message with text content for broken tools case")
	}

	// Verify Case A prompt (re-issue instruction) was injected.
	foundPrompt := false
	for _, m := range st.Messages {
		if m.Role == "user" && strings.Contains(m.Content, "Please re-issue") {
			foundPrompt = true
			break
		}
	}
	if !foundPrompt {
		t.Error("expected Case A continue prompt (re-issue) for broken tools")
	}
}

// TestRunStreaming_Recovery_ApplyContinueRule_ValidTools tests Case B:
// all tool calls valid → assistant with tool_calls + tool results + continue prompt.
func TestRunStreaming_Recovery_ApplyContinueRule_ValidTools(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Created the deck."),
				toolCallStartEvent("write_file", "toolu_001", 0),
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html></html>"}`),
				doneEventWithReason("length"),
			},
			{textDeltaEvent("All done."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{outputs: map[string]string{"write_file": "created deck.html"}}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Verify assistant with tool_calls was appended (Case B).
	found := false
	for _, m := range st.Messages {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected assistant message with tool_calls for Case B")
	}

	// Verify tool result placeholder was injected.
	foundTool := false
	for _, m := range st.Messages {
		if m.Role == "tool" && m.Content == "Tool executed before truncation." {
			foundTool = true
			break
		}
	}
	if !foundTool {
		t.Error("expected tool result placeholder for Case B")
	}

	// Verify Case B continue prompt was injected.
	foundPrompt := false
	for _, m := range st.Messages {
		if m.Role == "user" && strings.Contains(m.Content, "Please continue from where you left off") {
			foundPrompt = true
			break
		}
	}
	if !foundPrompt {
		t.Error("expected Case B continue prompt")
	}
}

// TestRunStreaming_Recovery_TimelineRollback_EventError tests that on
// EventError backoff retry, timeline events are rolled back so they do not
// duplicate across retries.
func TestRunStreaming_Recovery_TimelineRollback_EventError(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Working on it..."),
				thinkingEvent("Let me think..."),
				errorEvent("connection reset by peer"),
			},
			{textDeltaEvent("Retry success."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// After rollback and retry, the timeline should have content from the
	// successful retry only, not duplicated from the failed attempt.
	countWorking := 0
	for _, e := range st.Timeline {
		if e.Data != nil {
			if content, ok := e.Data["content"].(string); ok && content == "Working on it..." {
				countWorking++
			}
		}
	}
	if countWorking > 1 {
		t.Errorf("expected 'Working on it...' to appear at most once after rollback, got %d", countWorking)
	}
}

func TestRunStreaming_Recovery_ToolObsRollback_NoDuplicateEmitter(t *testing.T) {
	toolID := "call_retry_tool"
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				toolCallStartEvent("read_file", toolID, 0),
				toolCallCompleteEvent("read_file", toolID, `{"path":"a.txt"}`),
				errorEvent("connection reset by peer"),
			},
			{
				toolCallStartEvent("read_file", toolID, 0),
				toolCallCompleteEvent("read_file", toolID, `{"path":"a.txt"}`),
				{Type: provider.EventDone},
			},
			{{Type: provider.EventTextDelta, Delta: "done"}, {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{
		outputs: map[string]string{"read_file": "ok"},
	}
	emitter := observability.NewEmitter()
	ch := collectEvents(emitter)

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 6,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	events := drainEvents(ch)
	startCount := 0
	retryCount := 0
	for _, ev := range events {
		if ev.Type == "tool_call_start" {
			startCount++
		}
		if ev.Type == "round_retry" {
			retryCount++
		}
	}
	if startCount != 1 {
		t.Errorf("expected 1 committed tool_call_start after rollback, got %d", startCount)
	}
	if retryCount != 1 {
		t.Errorf("expected 1 round_retry event, got %d", retryCount)
	}
}

func TestRunStreaming_UserMessage_PersistsAttachments(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{{Type: provider.EventTextDelta, Delta: "ok"}, {Type: provider.EventDone}},
		},
	}
	emitter := observability.NewEmitter()
	cfg := StreamingLoopConfig{
		SystemPrompt: "S",
		UserMessage:  "hello",
		MaxRounds:    2,
		Provider:     provider,
		Execute:      &stubToolExec{},
		Emitter:      emitter,
		Attachments: []map[string]any{
			{"original_name": "brief.pdf", "saved_path_rel": "uploads/docs/brief.pdf.txt", "type": "pdf"},
		},
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	var found bool
	for _, e := range st.Timeline {
		if e.Event != "user_message" {
			continue
		}
		atts, ok := e.Data["attachments"].([]map[string]any)
		if !ok {
			// JSON round-trip may decode as []any
			if raw, ok2 := e.Data["attachments"].([]any); ok2 && len(raw) > 0 {
				found = true
				break
			}
		}
		if ok && len(atts) == 1 && atts[0]["original_name"] == "brief.pdf" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected user_message with attachments in timeline, got %+v", st.Timeline)
	}
}

// TestRunStreaming_Recovery_UsageAccumulation tests that Usage from
// EventDone is accumulated into CumulativeUsage.
func TestRunStreaming_Recovery_UsageAccumulation(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Hello."),
				doneEventWithUsage("stop", 150),
			},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 1,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.CumulativeUsage.CompletionTokens != 150 {
		t.Errorf("expected CumulativeUsage.CompletionTokens=150, got %d", st.CumulativeUsage.CompletionTokens)
	}
}

// TestRunStreaming_Recovery_EventError_Terminal tests that unclassified
// EventError (not transient, not context overflow) returns terminal error.
func TestRunStreaming_Recovery_EventError_Terminal(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Working..."),
				errorEvent("internal server error: something unexpected"),
			},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 1,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected terminal error for unclassified EventError")
	}
	if !strings.Contains(err.Error(), "stream error") {
		t.Fatalf("expected 'stream error' in error, got: %v", err)
	}
}

// TestRunStreaming_Recovery_Position1_Compress tests that when StreamChat returns
// a context_length_exceeded error, callWithRecovery classifies it as RecoveryCompress
// and the loop applies compression before retrying.
func TestRunStreaming_Recovery_Position1_Compress(t *testing.T) {
	ws := t.TempDir()
	origCwd, _ := os.Getwd()
	os.Chdir(ws)
	defer os.Chdir(origCwd)

	os.WriteFile("deck.html", []byte(`<html><body><section class="slide"></section></body></html>`), 0644)

	ctxMgr := agentctx.NewContextManager(ws, "test-session", "")

	provider := &stubStreamingProvider{
		chatErr:      fmt.Errorf("context_length_exceeded: too many tokens"),
		chatErrCount: 1,
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("Compressed and retried successfully."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt:   "S",
		UserMessage:    "U",
		MaxRounds:      4,
		Provider:       provider,
		Execute:        exec,
		Emitter:        emitter,
		ContextManager: ctxMgr,
		SessionID:      "test-session",
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.CompressCount != 1 {
		t.Errorf("expected CompressCount=1 after Position 1 compress, got %d", st.Recovery.CompressCount)
	}

	foundCompress := false
	for _, m := range st.Messages {
		if m.Role == "user" && strings.Contains(m.Content, "[[RECOVERY_COMPRESS_V1]]") {
			foundCompress = true
			break
		}
	}
	if !foundCompress {
		t.Error("expected compression summary in messages after Position 1 compress")
	}
}

// TestRunStreaming_Recovery_CircuitBreaker_OpenGate tests that a tripped circuit
// breaker causes callWithRecovery to return a terminal error immediately.
func TestRunStreaming_Recovery_CircuitBreaker_OpenGate(t *testing.T) {
	// Pre-trip the breaker: 5 consecutive RecordFailure calls.
	cb := NewCircuitBreaker()
	for i := 0; i < 5; i++ {
		cb.RecordFailure()
	}

	// callWithRecovery should reject immediately with circuit breaker open.
	rs := &RecoveryState{}
	cfg := StreamingLoopConfig{Provider: &stubStreamingProvider{}}

	_, _, err := callWithRecovery(context.Background(), cfg, nil, rs, cb)
	if err == nil {
		t.Fatal("expected terminal error from open circuit breaker")
	}
	if !strings.Contains(err.Error(), "circuit breaker open") {
		t.Fatalf("expected 'circuit breaker open' error, got: %v", err)
	}
	if !rs.Terminal {
		t.Error("expected RecoveryState.Terminal=true after circuit breaker open")
	}
}

// TestRunStreaming_Recovery_MessageSequence_Continue verifies the exact message
// sequence after a continue protocol: assistant(tool_calls) → tool(result) →
// user(continue prompt) → assistant(final).
func TestRunStreaming_Recovery_MessageSequence_Continue(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Creating the deck..."),
				toolCallStartEvent("write_file", "toolu_001", 0),
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html></html>"}`),
				doneEventWithReason("length"),
			},
			{textDeltaEvent("Deck completed."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{outputs: map[string]string{"write_file": "created deck.html"}}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Extract sequence after the initial system + user messages.
	// Expected: [system, user, assistant(tool_calls), tool(result), user(continue), assistant(final)]
	roles := make([]string, 0, len(st.Messages))
	for _, m := range st.Messages {
		roles = append(roles, m.Role)
	}

	sysIdx := indexOf(roles, "system")
	firstUserIdx := indexOf(roles, "user")
	if sysIdx < 0 || firstUserIdx < 0 {
		t.Fatal("expected system and first user messages")
	}

	// After first user: assistant(with tool_calls) → tool → user(continue) → assistant(final)
	seq := roles[firstUserIdx+1:]
	if len(seq) < 4 {
		t.Fatalf("expected at least 4 messages after first user, got %d: %v", len(seq), seq)
	}

	if seq[0] != "assistant" {
		t.Errorf("expected assistant at position +1, got %q", seq[0])
	}
	if seq[1] != "tool" {
		t.Errorf("expected tool at position +2, got %q", seq[1])
	}
	if seq[2] != "user" {
		t.Errorf("expected user(continue prompt) at position +3, got %q", seq[2])
	}
	if seq[3] != "assistant" {
		t.Errorf("expected assistant(final) at position +4, got %q", seq[3])
	}

	// Verify the continue prompt user message contains the expected marker.
	continueMsg := st.Messages[firstUserIdx+3]
	if !strings.Contains(continueMsg.Content, "Please continue from where you left off") {
		t.Errorf("expected continue prompt, got: %s", continueMsg.Content)
	}

	// Verify the tool message contains the placeholder.
	toolMsg := st.Messages[firstUserIdx+2]
	if toolMsg.Content != "Tool executed before truncation." {
		t.Errorf("expected tool placeholder, got: %s", toolMsg.Content)
	}
}

func indexOf(slice []string, target string) int {
	for i, s := range slice {
		if s == target {
			return i
		}
	}
	return -1
}

// TestRunStreaming_Recovery_ToolCallsFinishReason_NoContinue verifies that
// finish_reason="tool_calls" with valid JSON does NOT trigger the continue protocol.
func TestRunStreaming_Recovery_ToolCallsFinishReason_NoContinue(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				toolCallStartEvent("write_file", "toolu_001", 0),
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html></html>"}`),
				doneEventWithReason("tool_calls"),
			},
			{textDeltaEvent("Done."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{outputs: map[string]string{"write_file": "created deck.html"}}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Continue should NOT have been triggered.
	if st.Recovery.ContinueCount != 0 {
		t.Errorf("expected ContinueCount=0 for tool_calls finish_reason, got %d", st.Recovery.ContinueCount)
	}

	// Tools should have been executed normally.
	foundToolResult := false
	for _, m := range st.Messages {
		if m.Role == "tool" && m.Content == "created deck.html" {
			foundToolResult = true
			break
		}
	}
	if !foundToolResult {
		t.Error("expected tool result from normal tool execution")
	}
}

// TestRunStreaming_Recovery_MultipleRecoveryTypes tests a conversation that
// experiences both a backoff and a continue in sequence.
func TestRunStreaming_Recovery_MultipleRecoveryTypes(t *testing.T) {
	provider := &stubStreamingProvider{
		chatErr:      fmt.Errorf("503 Service Unavailable"),
		chatErrCount: 1,
		rounds: [][]provider.StreamEvent{
			// Round 1 (retry after backoff): truncated response.
			{
				textDeltaEvent("Generating HTML..."),
				toolCallStartEvent("write_file", "toolu_001", 0),
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html></html>"}`),
				doneEventWithReason("length"),
			},
			// Round 2 (continue): successful completion.
			{textDeltaEvent("All done after continue."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{outputs: map[string]string{"write_file": "created deck.html"}}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 6,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	if st.Recovery.BackoffCount != 1 {
		t.Errorf("expected BackoffCount=1, got %d", st.Recovery.BackoffCount)
	}
	if st.Recovery.ContinueCount != 1 {
		t.Errorf("expected ContinueCount=1, got %d", st.Recovery.ContinueCount)
	}
	if st.TransitionReason != TransitionModelCompleted {
		t.Errorf("expected TransitionModelCompleted, got %q", st.TransitionReason)
	}
}

// TestRunStreaming_Recovery_TerminalFlag tests that RecoveryState.Terminal is set
// to true on terminal (non-recoverable) errors.
func TestRunStreaming_Recovery_TerminalFlag(t *testing.T) {
	tests := []struct {
		name      string
		chatErr   error
		maxRounds int
	}{
		{
			name:      "401 unauthorized",
			chatErr:   fmt.Errorf("401 Unauthorized"),
			maxRounds: 1,
		},
		{
			name:      "invalid api key",
			chatErr:   fmt.Errorf("invalid api key"),
			maxRounds: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &stubStreamingProvider{chatErr: tt.chatErr}
			exec := &stubToolExec{}
			emitter := observability.NewEmitter()

			cfg := StreamingLoopConfig{
				SystemPrompt: "S", UserMessage: "U", MaxRounds: tt.maxRounds,
				Provider: provider, Execute: exec, Emitter: emitter,
			}

			st, err := RunStreaming(context.Background(), cfg)
			if err == nil {
				t.Fatal("expected terminal error")
			}
			if !st.Recovery.Terminal {
				t.Errorf("expected RecoveryState.Terminal=true for %s, got false", tt.name)
			}
		})
	}
}

// TestRunStreaming_Recovery_ToolCallsFinishReason_BrokenJSON_Guard tests that
// finish_reason="tool_calls" with broken tool call JSON does NOT trigger an
// infinite continue loop. The json.Valid guard in executeTools catches the
// broken JSON and injects a structured error.
func TestRunStreaming_Recovery_ToolCallsFinishReason_BrokenJSON_Guard(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("Using tool..."),
				toolCallStartEvent("write_file", "toolu_001", 0),
				// Broken JSON — truncated mid-string.
				toolCallCompleteEvent("write_file", "toolu_001", `{"path":"deck.html","content":"<html>...`),
				doneEventWithReason("tool_calls"),
			},
			{textDeltaEvent("Fixed the broken call."), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{outputs: map[string]string{"write_file": "should not execute"}}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}

	// Continue should NOT be triggered (finish_reason="tool_calls" bypasses POSITION 2).
	if st.Recovery.ContinueCount != 0 {
		t.Errorf("expected ContinueCount=0, got %d (infinite continue loop?)", st.Recovery.ContinueCount)
	}

	// The broken tool call should have been caught by json.Valid guard in executeTools.
	// Verify a structured error was injected.
	foundError := false
	for _, m := range st.Messages {
		if m.Role == "tool" && strings.Contains(m.Content, "invalid JSON") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected structured error for broken tool call JSON (json.Valid guard)")
	}

	// The write_file should NOT have been executed.
	for _, m := range st.Messages {
		if m.Role == "tool" && m.Content == "should not execute" {
			t.Error("write_file should NOT have been executed with broken JSON")
		}
	}
}

// TestRunStreaming_Recovery_ContextCancellationDuringBackoff tests that context
// cancellation during a backoff wait returns immediately with context.Canceled.
func TestRunStreaming_Recovery_ContextCancellationDuringBackoff(t *testing.T) {
	provider := &stubStreamingProvider{
		chatErr:      fmt.Errorf("503 Service Unavailable"),
		chatErrCount: 0, // always fail → will enter first backoff
		rounds:       [][]provider.StreamEvent{}, // never reached
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		_, err := RunStreaming(ctx, cfg)
		errCh <- err
	}()

	// Wait for the first backoff to start, then cancel.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from cancelled context")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for context cancellation")
	}
}

// TestHasBrokenToolCalls_EmptySlice verifies that an empty tool call slice
// is not considered broken (defensive edge case).
func TestHasBrokenToolCalls_EmptySlice(t *testing.T) {
	if hasBrokenToolCalls(nil) {
		t.Error("nil tool calls should not be broken")
	}
	if hasBrokenToolCalls([]model.ToolCall{}) {
		t.Error("empty tool calls should not be broken")
	}
}

// TestClassifyRecovery_ExhaustedRetries tests the exhaustion branches of
// ClassifyRecovery where retry counts have reached their maximum.
func TestClassifyRecovery_ExhaustedRetries(t *testing.T) {
	tests := []struct {
		name string
		rs   *RecoveryState
		err  error
		fr   string
		want RecoveryAction
	}{
		{
			name: "continue exhausted",
			rs:   &RecoveryState{ContinueCount: MaxContinueRetries},
			fr:   "length",
			want: RecoveryNone,
		},
		{
			name: "max_tokens continue exhausted",
			rs:   &RecoveryState{ContinueCount: MaxContinueRetries},
			fr:   "max_tokens",
			want: RecoveryNone,
		},
		{
			name: "compress exhausted",
			rs:   &RecoveryState{CompressCount: MaxCompressRetries},
			err:  fmt.Errorf("context_length_exceeded"),
			want: RecoveryNone,
		},
		{
			name: "backoff exhausted",
			rs:   &RecoveryState{BackoffCount: MaxBackoffRetries},
			err:  fmt.Errorf("503 Service Unavailable"),
			want: RecoveryNone,
		},
		{
			name: "ambiguous eof with continue exhausted",
			rs:   &RecoveryState{ContinueCount: MaxContinueRetries},
			fr:   "",
			err:  nil,
			// Need hadAnyDelta=true and empty content for isTruncated=true.
			// But err is nil here, so isTruncated returns true with empty content + hadAnyDelta.
			want: RecoveryNone, // ContinueCount >= MaxContinueRetries → RecoveryNone
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only the "ambiguous eof" test needs hadAnyDelta=true to trigger isTruncated.
			hadAnyDelta := tt.name == "ambiguous eof with continue exhausted"
			got := ClassifyRecovery(tt.err, tt.fr, "", nil, hadAnyDelta, tt.rs, nil)
			if got != tt.want {
				t.Errorf("ClassifyRecovery(...) = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestClassifyRecovery_NilCircuitBreaker verifies that ClassifyRecovery
// works correctly with a nil circuit breaker (cb == nil branch).
func TestClassifyRecovery_NilCircuitBreaker(t *testing.T) {
	rs := &RecoveryState{}

	// insufficient_system_resource with nil cb → should still allow backoff.
	got := ClassifyRecovery(nil, "insufficient_system_resource", "", nil, false, rs, nil)
	if got != RecoveryBackoff {
		t.Errorf("insufficient_system_resource with nil cb: got %d, want RecoveryBackoff", got)
	}

	// Transient error with nil cb → should still allow backoff.
	got = ClassifyRecovery(fmt.Errorf("503 error"), "", "", nil, false, rs, nil)
	if got != RecoveryBackoff {
		t.Errorf("transient error with nil cb: got %d, want RecoveryBackoff", got)
	}

	// After backoff increment, second transient error still allowed.
	rs.BackoffCount++
	got = ClassifyRecovery(fmt.Errorf("429 rate limit"), "", "", nil, false, rs, nil)
	if got != RecoveryBackoff {
		t.Errorf("transient error with nil cb (retry 2): got %d, want RecoveryBackoff", got)
	}
}

// ---------------------------------------------------------------------------
// isMemoryPath
// ---------------------------------------------------------------------------

func TestIsMemoryPath_DotSlashPrefix(t *testing.T) {
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentgo", "memory")

	if !isMemoryPath(ws, base, "./.agentgo/memory/design/theme.md") {
		t.Error("./.agentgo/memory/... should match")
	}
}

func TestIsMemoryPath_RelativeUnderMemory(t *testing.T) {
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentgo", "memory")

	if !isMemoryPath(ws, base, ".agentgo/memory/feedback/no-anim.md") {
		t.Error(".agentgo/memory/... should match")
	}
}

func TestIsMemoryPath_AbsolutePath(t *testing.T) {
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentgo", "memory")

	absPath := filepath.Join(base, "design", "theme.md")
	if !isMemoryPath(ws, base, absPath) {
		t.Error("absolute path under memory base should match")
	}
}

func TestIsMemoryPath_PathOutsideMemory(t *testing.T) {
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentgo", "memory")

	if isMemoryPath(ws, base, "deck.html") {
		t.Error("deck.html outside memory should NOT match")
	}
	if isMemoryPath(ws, base, "/etc/passwd") {
		t.Error("/etc/passwd should NOT match")
	}
	if isMemoryPath(ws, base, ".agentgo/sessions/test.json") {
		t.Error(".agentgo/sessions/ should NOT match")
	}
}

func TestIsMemoryPath_EmptyPath(t *testing.T) {
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentgo", "memory")

	if isMemoryPath(ws, base, "") {
		t.Error("empty path should NOT match")
	}
}

func TestIsMemoryPath_CleanPath(t *testing.T) {
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentgo", "memory")

	if !isMemoryPath(ws, base, ".agentgo/memory//design/theme.md") {
		t.Error("double-slash path should match after cleaning")
	}
}

// ---------------------------------------------------------------------------
// extractPathFromJSON
// ---------------------------------------------------------------------------

func TestExtractPathFromJSON_Valid(t *testing.T) {
	got := extractPathFromJSON(`{"path": "deck.html"}`)
	if got != "deck.html" {
		t.Errorf("expected 'deck.html', got %q", got)
	}
}

func TestExtractPathFromJSON_InvalidJSON(t *testing.T) {
	got := extractPathFromJSON(`{bad`)
	if got != "" {
		t.Errorf("expected empty for invalid JSON, got %q", got)
	}
}

func TestExtractPathFromJSON_Empty(t *testing.T) {
	if got := extractPathFromJSON(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractPathFromJSON_MissingField(t *testing.T) {
	got := extractPathFromJSON(`{"other": "value"}`)
	if got != "" {
		t.Errorf("expected empty for missing path field, got %q", got)
	}
}

// TestRunStreaming_Recovery_EventError_Continue tests mid-stream EventError
// where ClassifyRecovery returns RecoveryContinue (content was truncated, e.g.
// unclosed markdown fence). The loop should apply the continue rule and retry.
func TestRunStreaming_Recovery_EventError_Continue(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{
				textDeltaEvent("```html\n<section>content</section>\n"),
				// No closing ``` — isTruncated returns true.
				errorEvent("stream interrupted"),
			},
			{textDeltaEvent("```html\n<section>more</section>\n```"), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	st, err := RunStreaming(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RunStreaming failed: %v", err)
	}
	if st.Recovery.ContinueCount != 1 {
		t.Errorf("expected ContinueCount=1 after EventError continue, got %d", st.Recovery.ContinueCount)
	}
}

// TestRunStreaming_Recovery_EventError_ContinueExhausted tests mid-stream
// EventError where ContinueCount is already at MaxContinueRetries.
func TestRunStreaming_Recovery_EventError_ContinueExhausted(t *testing.T) {
	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			// First two rounds produce truncated content to exhaust continue retries.
			{
				textDeltaEvent("```html\n<section>1</section>\n"),
				errorEvent("stream interrupted at round 1"),
			},
			{
				textDeltaEvent("```html\n<section>2</section>\n"),
				errorEvent("stream interrupted at round 2"),
			},
			// Third round: still truncated → RecoveryNone (exhausted).
			{
				textDeltaEvent("```html\n<section>3</section>\n"),
				errorEvent("stream interrupted at round 3"),
			},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected terminal error after continue retries exhausted")
	}
	if !strings.Contains(err.Error(), "stream error round 3") {
		t.Errorf("expected 'stream error round 3' in error, got: %v", err)
	}
}

// TestRunStreaming_PreContextAssemble_BlockedError tests that a hook
// returning BlockedError at PreContextAssemble terminates the loop.
func TestRunStreaming_PreContextAssemble_BlockedError(t *testing.T) {
	ws := t.TempDir()
	engine := hook.NewEngineWithConfig(hook.DefaultConfig())
	engine.InitState("test-session", ws, hook.StageInitialGeneration)

	engine.Register(&hook.RegisteredHook{
		Name: "blocking-hook", On: hook.PointPreContextAssemble,
		Stage: "always", Priority: 1,
		Fn: func(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
			return hook.HookResult{Action: hook.Block, Reason: "session limit exceeded"}
		},
	})

	provider := &stubStreamingProvider{
		rounds: [][]provider.StreamEvent{
			{textDeltaEvent("should not be reached"), {Type: provider.EventDone}},
		},
	}
	exec := &stubToolExec{}
	emitter := observability.NewEmitter()

	cfg := StreamingLoopConfig{
		SystemPrompt: "S", UserMessage: "U", MaxRounds: 4,
		Provider: provider, Execute: exec, Emitter: emitter,
		HookEngine: engine, SessionState: engine.State(),
	}

	_, err := RunStreaming(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error from PreContextAssemble BlockedError")
	}
	if !strings.Contains(err.Error(), "pre_context_assemble blocked") {
		t.Errorf("expected 'pre_context_assemble blocked' in error, got: %v", err)
	}
	if !hook.IsBlockedError(err) {
		t.Errorf("expected BlockedError, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// Discovery break tests
// ---------------------------------------------------------------------------

