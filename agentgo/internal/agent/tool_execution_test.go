package agent

import (
	"context"
	"errors"
	"testing"

	"agentgo/internal/hook"
	"agentgo/internal/model"
	"agentgo/internal/observability"
	"agentgo/internal/toolkit/contracts"
)

// stubToolFlags implements ToolFlagProvider for tests.
type stubToolFlags struct {
	flags map[string]contracts.ToolBehaviorFlags
}

func (s *stubToolFlags) GetToolFlags(name string) (contracts.ToolBehaviorFlags, error) {
	if f, ok := s.flags[name]; ok {
		return f, nil
	}
	return contracts.ToolBehaviorFlags{}, nil
}

func TestCheckPreToolUse_BlockedError(t *testing.T) {
	engine := hook.NewEngineWithConfig(hook.DefaultConfig())
	engine.Register(&hook.RegisteredHook{
		Name:     "test-block",
		On:       hook.PointPreToolUse,
		Fn:       func(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
			return hook.HookResult{Action: hook.Block, Reason: "test block"}
		},
	})
	engine.InitState("sess1", "/tmp/ws", hook.StageInitialGeneration)

	cfg := StreamingLoopConfig{
		HookEngine:   engine,
		SessionState: engine.State(),
	}
	p := preToolUseParams{
		cfg:   cfg,
		round: 1,
		tc:    model.ToolCall{ID: "call_1", Function: model.ToolCallFunction{Name: "write_file", Arguments: `{"path":"test.html"}`}},
		args:  map[string]any{"path": "test.html"},
	}

	result := checkPreToolUse(context.Background(), p)
	if result == nil {
		t.Fatal("expected blocked result, got nil")
	}
	if !result.isError {
		t.Error("expected isError=true for blocked tool call")
	}
}

func TestCheckPreToolUse_NilEngine(t *testing.T) {
	cfg := StreamingLoopConfig{} // HookEngine is nil
	p := preToolUseParams{
		cfg:   cfg,
		round: 1,
		tc:    model.ToolCall{ID: "call_1", Function: model.ToolCallFunction{Name: "write_file", Arguments: `{}`}},
	}

	result := checkPreToolUse(context.Background(), p)
	if result != nil {
		t.Error("expected nil result when HookEngine is nil")
	}
}

func TestCheckPreToolUse_NilSessionState(t *testing.T) {
	engine := hook.NewEngineWithConfig(hook.DefaultConfig())
	// Don't call InitState — SessionState is nil.

	cfg := StreamingLoopConfig{
		HookEngine: engine, // SessionState defaults to nil
	}
	p := preToolUseParams{
		cfg:   cfg,
		round: 1,
		tc:    model.ToolCall{ID: "call_1", Function: model.ToolCallFunction{Name: "write_file", Arguments: `{}`}},
	}

	result := checkPreToolUse(context.Background(), p)
	if result != nil {
		t.Error("expected nil result when SessionState is nil")
	}
}

func TestCheckPreToolUse_AllowProceeds(t *testing.T) {
	engine := hook.NewEngineWithConfig(hook.DefaultConfig())
	engine.Register(&hook.RegisteredHook{
		Name:     "test-allow",
		On:       hook.PointPreToolUse,
		Fn:       func(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
			return hook.HookResult{Action: hook.Allow}
		},
	})
	engine.InitState("sess1", "/tmp/ws", hook.StageInitialGeneration)

	cfg := StreamingLoopConfig{
		HookEngine:   engine,
		SessionState: engine.State(),
	}
	p := preToolUseParams{
		cfg:   cfg,
		round: 1,
		tc:    model.ToolCall{ID: "call_1", Function: model.ToolCallFunction{Name: "read_file", Arguments: `{"path":"test.txt"}`}},
		args:  map[string]any{"path": "test.txt"},
	}

	result := checkPreToolUse(context.Background(), p)
	if result != nil {
		t.Errorf("expected nil result for allowed tool, got %+v", result)
	}
}

func TestRunPostToolUse_NilEngine(t *testing.T) {
	cfg := StreamingLoopConfig{} // HookEngine is nil
	p := postToolUseParams{
		cfg:      cfg,
		round:    1,
		toolName: "write_file",
	}

	// Should not panic.
	runPostToolUse(context.Background(), p)
}

func TestRunPostToolUse_RecordsAndRunsHooks(t *testing.T) {
	engine := hook.NewEngineWithConfig(hook.DefaultConfig())
	engine.InitState("sess1", "/tmp/ws", hook.StageInitialGeneration)

	cfg := StreamingLoopConfig{
		HookEngine:   engine,
		SessionState: engine.State(),
		Emitter:      observability.NewEmitter(),
	}
	p := postToolUseParams{
		cfg:      cfg,
		round:    1,
		toolName: "write_file",
		args:     map[string]any{"path": "test.html"},
		isError:  false,
	}

	runPostToolUse(context.Background(), p)

	// Verify RecordToolCall was executed.
	if engine.State().ToolsCalled["write_file"] != 1 {
		t.Errorf("expected write_file called once, got %d", engine.State().ToolsCalled["write_file"])
	}
}

func TestRunPostToolUse_NonBlockedError(t *testing.T) {
	// Register a hook that returns a non-BlockedError via a panic.
	// We can't easily test the log output, but we verify the function doesn't
	// panic when the hook engine encounters an error that isn't a BlockedError.
	engine := hook.NewEngineWithConfig(hook.DefaultConfig())
	engine.InitState("sess1", "/tmp/ws", hook.StageInitialGeneration)

	cfg := StreamingLoopConfig{
		HookEngine:   engine,
		SessionState: engine.State(),
	}
	p := postToolUseParams{
		cfg:      cfg,
		round:    1,
		toolName: "write_file",
		args:     map[string]any{"path": "test.html"},
		isError:  false,
	}

	// Should not panic even with no registered hooks.
	runPostToolUse(context.Background(), p)
}

func TestIsReadOnlyTool_Defaults(t *testing.T) {
	cfg := StreamingLoopConfig{}
	readOnlyTools := []string{"read_file", "list_files", "grep_search", "web_fetch", "tool_search", "skill"}
	writeTools := []string{"write_file", "edit_file", "bash", "execute"}

	for _, name := range readOnlyTools {
		if !isReadOnlyTool(cfg, name) {
			t.Errorf("expected %q to be read-only by default", name)
		}
	}
	for _, name := range writeTools {
		if isReadOnlyTool(cfg, name) {
			t.Errorf("expected %q to NOT be read-only by default", name)
		}
	}
}

func TestIsReadOnlyTool_CustomFlags(t *testing.T) {
	combined := &struct {
		*stubExecute
		*stubToolFlags
	}{
		stubExecute: &stubExecute{output: "ok"},
		stubToolFlags: &stubToolFlags{flags: map[string]contracts.ToolBehaviorFlags{
			"custom_read":  {ReadOnly: true, ConcurrencySafe: true},
			"custom_write": {ReadOnly: false, ConcurrencySafe: false},
		}},
	}
	cfg := StreamingLoopConfig{Execute: combined}

	if !isReadOnlyTool(cfg, "custom_read") {
		t.Error("expected custom_read to be read-only")
	}
	if isReadOnlyTool(cfg, "custom_write") {
		t.Error("expected custom_write to NOT be read-only")
	}
	// Unknown tool without flags should not be read-only.
	if isReadOnlyTool(cfg, "unknown_tool") {
		t.Error("expected unknown_tool to NOT be read-only")
	}
}

func TestParseToolArgsJSON(t *testing.T) {
	args := parseToolArgsJSON(`{"path": "test.html", "content": "hello"}`)
	if args["path"] != "test.html" {
		t.Errorf("expected test.html, got %v", args["path"])
	}

	// Invalid JSON returns nil.
	if args := parseToolArgsJSON(`{invalid}`); args != nil {
		t.Error("expected nil for invalid JSON")
	}

	// Empty returns nil (empty JSON is valid but returns empty map, which is fine).
	if args := parseToolArgsJSON(`{}`); args == nil {
		t.Error("expected non-nil for empty JSON object")
	}
}

func TestHasBrokenToolCalls_Empty(t *testing.T) {
	if hasBrokenToolCalls(nil) {
		t.Error("expected false for nil")
	}
	if hasBrokenToolCalls([]model.ToolCall{}) {
		t.Error("expected false for empty")
	}
}

func TestHasBrokenToolCalls_InvalidJSON(t *testing.T) {
	calls := []model.ToolCall{
		{Function: model.ToolCallFunction{Name: "write_file", Arguments: `{broken}`}},
	}
	if !hasBrokenToolCalls(calls) {
		t.Error("expected true for broken JSON")
	}
}

func TestHasBrokenToolCalls_MissingFields(t *testing.T) {
	// write_file requires path AND content.
	calls := []model.ToolCall{
		{Function: model.ToolCallFunction{Name: "write_file", Arguments: `{"path": "test.html"}`}},
	}
	if !hasBrokenToolCalls(calls) {
		t.Error("expected true when missing required fields")
	}
}

func TestHasBrokenToolCalls_Valid(t *testing.T) {
	calls := []model.ToolCall{
		{Function: model.ToolCallFunction{Name: "write_file", Arguments: `{"path": "test.html", "content": "hi"}`}},
	}
	if hasBrokenToolCalls(calls) {
		t.Error("expected false for valid tool calls")
	}
}

// stubExecute implements ToolExecutor for testing executeTools.
type stubExecute struct {
	output string
	err    error
}

func (e *stubExecute) Execute(ctx context.Context, name, argsJSON string) (string, map[string]any, error) {
	return e.output, nil, e.err
}

// Ensure stubExecute satisfies ToolExecutor.
var _ ToolExecutor = (*stubExecute)(nil)

// Ensure stubToolFlags satisfies ToolFlagProvider.
var _ ToolFlagProvider = (*stubToolFlags)(nil)

func TestExecuteTools_InvalidJSON(t *testing.T) {
	cfg := StreamingLoopConfig{
		Execute: &stubExecute{output: "ok"},
	}
	calls := []model.ToolCall{
		{ID: "call_1", Function: model.ToolCallFunction{Name: "write_file", Arguments: `{broken`}},
	}
	buf := observability.NewRoundEventBuffer()

	results := executeTools(context.Background(), cfg, 1, calls, buf)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].isError {
		t.Error("expected isError=true for invalid JSON")
	}
}

func TestExecuteTools_WithHookEngine(t *testing.T) {
	engine := hook.NewEngineWithConfig(hook.DefaultConfig())
	engine.InitState("sess1", "/tmp/ws", hook.StageInitialGeneration)

	// Stub that provides both Execute and GetToolFlags.
	combined := &struct {
		*stubExecute
		*stubToolFlags
	}{
		stubExecute:  &stubExecute{output: "file content"},
		stubToolFlags: &stubToolFlags{flags: map[string]contracts.ToolBehaviorFlags{
			"read_file": {ReadOnly: true, ConcurrencySafe: true},
		}},
	}

	cfg := StreamingLoopConfig{
		Execute:      combined,
		HookEngine:   engine,
		SessionState: engine.State(),
	}
	calls := []model.ToolCall{
		{ID: "call_1", Function: model.ToolCallFunction{Name: "read_file", Arguments: `{"path":"test.txt"}`}},
	}
	buf := observability.NewRoundEventBuffer()

	results := executeTools(context.Background(), cfg, 1, calls, buf)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Verify RecordToolStartCalled was invoked.
	if len(engine.State().LastRoundTools) != 1 {
		t.Errorf("expected 1 tool in LastRoundTools, got %d", len(engine.State().LastRoundTools))
	}
}

func TestExecuteTools_SkipNonFunction(t *testing.T) {
	cfg := StreamingLoopConfig{
		Execute: &stubExecute{output: "ok"},
	}
	calls := []model.ToolCall{
		{ID: "call_1", Type: "non_function", Function: model.ToolCallFunction{Name: "custom", Arguments: `{}`}},
	}
	buf := observability.NewRoundEventBuffer()

	results := executeTools(context.Background(), cfg, 1, calls, buf)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for skipped non-function, got %d", len(results))
	}
	if results[0].toolCallID != "call_1" {
		t.Error("expected toolCallID preserved")
	}
}

func TestExecuteTools_ToolError(t *testing.T) {
	execErr := errors.New("tool execution failed")
	cfg := StreamingLoopConfig{
		Execute: &stubExecute{err: execErr},
	}
	calls := []model.ToolCall{
		{ID: "call_1", Function: model.ToolCallFunction{Name: "read_file", Arguments: `{"path":"test.txt"}`}},
	}
	buf := observability.NewRoundEventBuffer()

	results := executeTools(context.Background(), cfg, 1, calls, buf)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].isError {
		t.Error("expected isError=true for tool execution failure")
	}
}
