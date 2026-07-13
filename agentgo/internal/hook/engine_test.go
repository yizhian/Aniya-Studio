package hook

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agentgo/internal/model"
)

func TestEngineRegisterAndMatch(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)

	called := false
	engine.Register(&RegisteredHook{
		Name:     "test-hook",
		On:       PointPreToolUse,
		Stage:    "always",
		Priority: 10,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			called = true
			return HookResult{Action: Allow}
		},
	})

	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)
	hctx := &HookContext{
		SessionID:     "sess1",
		WorkspacePath: "/tmp/ws",
		Stage:         StageInitialGeneration,
		ToolName:      "write_file",
		ToolArgs:      map[string]any{"path": "test.html"},
	}

	warnings, err := engine.Run(context.Background(), PointPreToolUse, hctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %d", len(warnings))
	}
	if !called {
		t.Error("hook was not called")
	}
}

func TestEngineBlockAction(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)

	engine.Register(&RegisteredHook{
		Name:     "block-hook",
		On:       PointPreToolUse,
		Stage:    "always",
		Priority: 10,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			return HookResult{Action: Block, Reason: "not allowed"}
		},
	})

	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)
	hctx := &HookContext{
		SessionID:     "sess1",
		WorkspacePath: "/tmp/ws",
		Stage:         StageInitialGeneration,
		ToolName:      "write_file",
		ToolArgs:      map[string]any{"path": "test.html"},
	}

	_, err := engine.Run(context.Background(), PointPreToolUse, hctx)
	if err == nil {
		t.Fatal("expected BlockedError, got nil")
	}
	var blocked *BlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected BlockedError, got %T: %v", err, err)
	}
	if blocked.HookName != "block-hook" {
		t.Errorf("expected hook name 'block-hook', got %q", blocked.HookName)
	}
}

func TestEngineWarnAction(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)

	engine.Register(&RegisteredHook{
		Name:     "warn-hook",
		On:       PointPreToolUse,
		Stage:    "always",
		Priority: 10,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			return HookResult{Action: Warn, Reason: "be careful"}
		},
	})

	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)
	hctx := &HookContext{
		SessionID:     "sess1",
		WorkspacePath: "/tmp/ws",
		Stage:         StageInitialGeneration,
		ToolName:      "write_file",
	}

	warnings, err := engine.Run(context.Background(), PointPreToolUse, hctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0] == "" {
		t.Error("expected non-empty warning message")
	}
}

func TestEngineStageFiltering(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)

	called := false
	engine.Register(&RegisteredHook{
		Name:     "init-only",
		On:       PointPreToolUse,
		Stage:    string(StageInitialGeneration),
		Priority: 10,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			called = true
			return HookResult{Action: Allow}
		},
	})

	engine.InitState("sess1", "/tmp/ws", StageIterativeEdit)
	hctx := &HookContext{
		SessionID:     "sess1",
		WorkspacePath: "/tmp/ws",
		Stage:         StageIterativeEdit,
		ToolName:      "write_file",
	}

	engine.Run(context.Background(), PointPreToolUse, hctx)
	if called {
		t.Error("hook should not have been called for iterative_edit stage")
	}
}

func TestEngineMatcherFiltering(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)

	called := false
	engine.Register(&RegisteredHook{
		Name:     "html-only",
		On:       PointPreToolUse,
		Stage:    "always",
		Priority: 10,
		Matcher: &Matcher{
			ToolNames:    []string{"write_file"},
			PathPatterns: []string{"*.html"},
		},
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			called = true
			return HookResult{Action: Allow}
		},
	})

	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)

	// Should not match: wrong file extension
	hctx := &HookContext{
		SessionID:     "sess1",
		WorkspacePath: "/tmp/ws",
		Stage:         StageInitialGeneration,
		ToolName:      "write_file",
		ToolArgs:      map[string]any{"path": "test.css"},
	}
	engine.Run(context.Background(), PointPreToolUse, hctx)
	if called {
		t.Error("hook should not match .css file")
	}

	// Should match: correct file extension
	called = false
	hctx.ToolArgs = map[string]any{"path": "test.html"}
	engine.Run(context.Background(), PointPreToolUse, hctx)
	if !called {
		t.Error("hook should match .html file")
	}
}

func TestEnginePriorityOrder(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)

	order := make([]int, 0, 3)
	engine.Register(&RegisteredHook{
		Name: "third", On: PointPreToolUse, Stage: "always", Priority: 30,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			order = append(order, 3)
			return HookResult{Action: Allow}
		},
	})
	engine.Register(&RegisteredHook{
		Name: "first", On: PointPreToolUse, Stage: "always", Priority: 10,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			order = append(order, 1)
			return HookResult{Action: Allow}
		},
	})
	engine.Register(&RegisteredHook{
		Name: "second", On: PointPreToolUse, Stage: "always", Priority: 20,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			order = append(order, 2)
			return HookResult{Action: Allow}
		},
	})

	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)
	engine.Run(context.Background(), PointPreToolUse, &HookContext{
		SessionID: "sess1", WorkspacePath: "/tmp/ws", Stage: StageInitialGeneration,
	})

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("expected order [1,2,3], got %v", order)
	}
}

func TestEngineHookOverrideDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Hooks = []HookOverrides{
		{Name: "my-hook", Enabled: boolPtr(false)},
	}
	engine := NewEngineWithConfig(cfg)

	called := false
	engine.Register(&RegisteredHook{
		Name: "my-hook", On: PointPreToolUse, Stage: "always", Priority: 10,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			called = true
			return HookResult{Action: Allow}
		},
	})

	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)
	engine.Run(context.Background(), PointPreToolUse, &HookContext{
		SessionID: "sess1", WorkspacePath: "/tmp/ws", Stage: StageInitialGeneration,
	})

	if called {
		t.Error("hook should be disabled by override")
	}
}

func TestEngineBuiltInCannotBeDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Hooks = []HookOverrides{
		{Name: "builtin-hook", Enabled: boolPtr(false)},
	}
	engine := NewEngineWithConfig(cfg)

	called := false
	engine.Register(&RegisteredHook{
		Name: "builtin-hook", On: PointPreToolUse, Stage: "always", Priority: 10, Builtin: true,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			called = true
			return HookResult{Action: Allow}
		},
	})

	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)
	engine.Run(context.Background(), PointPreToolUse, &HookContext{
		SessionID: "sess1", WorkspacePath: "/tmp/ws", Stage: StageInitialGeneration,
	})

	if !called {
		t.Error("builtin hook should not be disabled by override")
	}
}

func TestIsBlockedError(t *testing.T) {
	blocked := &BlockedError{HookName: "test", Reason: "no"}
	if !IsBlockedError(blocked) {
		t.Error("IsBlockedError should return true for BlockedError")
	}
	if IsBlockedError(errors.New("plain error")) {
		t.Error("IsBlockedError should return false for plain error")
	}
	if IsBlockedError(nil) {
		t.Error("IsBlockedError should return false for nil")
	}
}

func TestMatcherEmptyPath(t *testing.T) {
	m := &Matcher{PathPatterns: []string{"*.html"}}
	hctx := &HookContext{ToolName: "write_file", ToolArgs: map[string]any{}}
	if m.Match(hctx) {
		t.Error("matcher should not match when path is missing")
	}
}

func TestMatcherGlobFullPath(t *testing.T) {
	m := &Matcher{PathPatterns: []string{"*.html"}}
	hctx := &HookContext{ToolName: "write_file", ToolArgs: map[string]any{"path": "src/components/foo.html"}}
	if !m.Match(hctx) {
		t.Error("matcher should match *.html against full path with dirs")
	}
}

func TestNewEngineWithConfig_NilConfig(t *testing.T) {
	engine := NewEngineWithConfig(nil)
	if engine == nil {
		t.Fatal("expected non-nil engine even with nil config")
	}
	if engine.config == nil {
		t.Fatal("expected engine to use default config when nil passed")
	}
	if engine.config.Version != "1" {
		t.Errorf("expected default config version '1', got %q", engine.config.Version)
	}
}

func TestEngineStateAndSetState(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)

	// Before InitState, State() should return nil.
	if s := engine.State(); s != nil {
		t.Error("expected nil state before InitState")
	}

	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)
	s1 := engine.State()
	if s1 == nil {
		t.Fatal("expected non-nil state after InitState")
	}
	if s1.SessionID != "sess1" {
		t.Errorf("expected session ID 'sess1', got %q", s1.SessionID)
	}

	// SetState to replace.
	s2 := NewSessionState("sess2", "/tmp/ws2", StageIterativeEdit)
	engine.SetState(s2)
	if engine.State().SessionID != "sess2" {
		t.Errorf("expected 'sess2' after SetState, got %q", engine.State().SessionID)
	}
}

func TestRecordToolResult(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)
	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)

	// Register a post_tool_use hook to verify it runs.
	called := false
	engine.Register(&RegisteredHook{
		Name: "test-post", On: PointPostToolUse, Stage: "always", Priority: 10,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			called = true
			return HookResult{Action: Allow}
		},
	})

	warnings, err := engine.RecordToolResult(context.Background(), 1,
		"write_file", map[string]any{"path": "test.html"}, false, map[string]any{"count": 3}, "tool output content")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("post_tool_use hook was not called")
	}
	_ = warnings

	// Verify state was updated.
	s := engine.State()
	if s.ToolsCalled["write_file"] != 1 {
		t.Errorf("expected write_file called once, got %d", s.ToolsCalled["write_file"])
	}
}

func TestRecordToolResult_NilState(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)
	// Don't call InitState — state is nil.

	warnings, err := engine.RecordToolResult(context.Background(), 1,
		"write_file", map[string]any{"path": "test.html"}, false, nil, "tool output content")

	if err != nil {
		t.Fatalf("unexpected error with nil state: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
}

func TestRecordRoundEnd(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)
	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)

	// Initially ConsecutiveRoundsNoWrite should be 0.
	s := engine.State()
	s.TrackRoundEnd() // first no-write round
	if s.ConsecutiveRoundsNoWrite != 1 {
		t.Errorf("expected 1 after first no-write round, got %d", s.ConsecutiveRoundsNoWrite)
	}

	// Call RecordRoundEnd through engine.
	engine.RecordRoundEnd(context.Background(), 1)
	if s.ConsecutiveRoundsNoWrite != 2 {
		t.Errorf("expected 2 after second no-write round, got %d", s.ConsecutiveRoundsNoWrite)
	}
}

func TestRecordRoundEnd_NilState(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)
	// No InitState — should not panic.
	engine.RecordRoundEnd(context.Background(), 1)
}

func TestInjectWarnings(t *testing.T) {
	// Test with system prompt first message (common case).
	messages := []model.Message{
		{Role: "system", Content: "You are an agent."},
		{Role: "user", Content: "Build a slide."},
	}
	warnings := []string{"[System] warn-hook\nBe careful."}

	result := InjectWarnings(messages, warnings)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("expected system message first, got %s", result[0].Role)
	}
	if !strings.Contains(result[0].Content, warnings[0]) {
		t.Errorf("expected system message to contain warning, got: %s", result[0].Content)
	}
	if result[1].Role != "user" || result[1].Content != "Build a slide." {
		t.Error("expected original user message second")
	}
}

func TestInjectWarnings_NoSystemPrompt(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Build a slide."},
	}
	warnings := []string{"warning text"}

	result := InjectWarnings(messages, warnings)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("expected warning as system message, got role=%s", result[0].Role)
	}
	if result[0].Content != "warning text" {
		t.Errorf("expected warning first, got %q", result[0].Content)
	}
}

func TestInjectWarnings_EmptyMessages(t *testing.T) {
	result := InjectWarnings(nil, []string{"warning"})
	if len(result) != 0 {
		t.Errorf("expected 0 messages for nil input, got %d", len(result))
	}
}

func TestInjectWarnings_EmptyWarningString(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "hello"},
	}
	// Whitespace-only warning should be skipped.
	warnings := []string{"   ", "valid warning"}

	result := InjectWarnings(messages, warnings)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages (one skipped), got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("expected warning as system message, got role=%s", result[0].Role)
	}
	if result[0].Content != "valid warning" {
		t.Errorf("expected valid warning, got %q", result[0].Content)
	}
}

func TestInjectWarnings_MultipleWarnings(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "hello"},
	}
	warnings := []string{"warn1", "warn2"}

	result := InjectWarnings(messages, warnings)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Error("expected system message first")
	}
	if !strings.Contains(result[0].Content, "warn1") || !strings.Contains(result[0].Content, "warn2") {
		t.Errorf("expected both warnings in system content, got: %s", result[0].Content)
	}
}

func TestBlockedError_Error(t *testing.T) {
	blocked := &BlockedError{HookName: "test-hook", Reason: "not allowed"}
	msg := blocked.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
	if !containsStr(msg, "test-hook") {
		t.Error("error message should contain hook name")
	}
	if !containsStr(msg, "not allowed") {
		t.Error("error message should contain reason")
	}
}

func TestNewEngine_WithInvalidPath(t *testing.T) {
	// NewEngine loads config from disk; with invalid path, should fall back to defaults.
	engine := NewEngine("/tmp/nonexistent/hooks.yaml")
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if engine.config == nil {
		t.Fatal("expected non-nil config (defaults)")
	}
}

func TestEngineRun_NilConfigInHctx(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)

	called := false
	engine.Register(&RegisteredHook{
		Name: "check-config", On: PointPreToolUse, Stage: "always", Priority: 10,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			if hctx.Config == nil {
				t.Error("expected Config to be populated in hctx")
			}
			called = true
			return HookResult{Action: Allow}
		},
	})

	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)
	hctx := &HookContext{
		SessionID: "sess1", WorkspacePath: "/tmp/ws", Stage: StageInitialGeneration,
		// Config is nil — engine should populate it.
	}

	engine.Run(context.Background(), PointPreToolUse, hctx)
	if !called {
		t.Error("hook should have been called")
	}
}

func TestMatcher_Match_BothFilters(t *testing.T) {
	m := &Matcher{
		ToolNames:    []string{"write_file"},
		PathPatterns: []string{"*.html"},
	}
	hctx := &HookContext{
		ToolName: "write_file",
		ToolArgs: map[string]any{"path": "index.html"},
	}
	if !m.Match(hctx) {
		t.Error("should match when both tool name and path pattern match")
	}

	// Wrong tool name
	hctx2 := &HookContext{
		ToolName: "read_file",
		ToolArgs: map[string]any{"path": "index.html"},
	}
	if m.Match(hctx2) {
		t.Error("should not match when tool name differs")
	}
}

func TestMatcher_Match_OnlyToolNames(t *testing.T) {
	m := &Matcher{ToolNames: []string{"write_file", "edit_file"}}
	hctx := &HookContext{ToolName: "edit_file"}
	if !m.Match(hctx) {
		t.Error("should match edit_file")
	}
	hctx2 := &HookContext{ToolName: "read_file"}
	if m.Match(hctx2) {
		t.Error("should not match read_file")
	}
}

func TestContains_EdgeCases(t *testing.T) {
	if contains(nil, "test") {
		t.Error("nil slice should not contain anything")
	}
	if contains([]string{}, "test") {
		t.Error("empty slice should not contain anything")
	}
	if !contains([]string{"a", "b", "c"}, "b") {
		t.Error("should contain 'b'")
	}
}

func TestMatchesAnyGlob_NoMatch(t *testing.T) {
	if matchesAnyGlob("readme.txt", []string{"*.html", "*.css"}) {
		t.Error("should not match readme.txt with html/css patterns")
	}
}

func TestMatchesAnyGlob_MatchFullPath(t *testing.T) {
	if !matchesAnyGlob("src/components/foo.html", []string{"*.html"}) {
		t.Error("should match *.html against full path")
	}
}

func TestMatchesAnyGlob_NilPatterns(t *testing.T) {
	if matchesAnyGlob("test.html", nil) {
		t.Error("should not match with nil patterns")
	}
}

func TestMatchesAnyGlob_MatchBasename(t *testing.T) {
	if !matchesAnyGlob("deep/nested/path/file.css", []string{"*.css"}) {
		t.Error("should match *.css against basename")
	}
}

func TestMatch_WithHctxPathInArgs(t *testing.T) {
	m := &Matcher{PathPatterns: []string{"*.svg"}}
	hctx := &HookContext{
		ToolName: "write_file",
		ToolArgs: map[string]any{"path": "icon.svg"},
	}
	if !m.Match(hctx) {
		t.Error("should match svg pattern")
	}

	// Path is not a string
	hctx2 := &HookContext{
		ToolName: "write_file",
		ToolArgs: map[string]any{"path": 123},
	}
	if m.Match(hctx2) {
		t.Error("should not match when path is not a string")
	}
}

func TestHashToolArgs_ErrorPath(t *testing.T) {
	// Hashing should handle complex types gracefully.
	// This covers the json.Marshal path.
	result := HashToolArgs(map[string]any{"key": "value", "nested": map[string]any{"a": 1}})
	if result == "" || result == "0" {
		t.Errorf("expected non-zero hash for non-empty args, got %q", result)
	}
}

func TestMatch_StageEdgeCases(t *testing.T) {
	// Stage empty string should match everything (not caught by the filter)
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)

	called := false
	engine.Register(&RegisteredHook{
		Name: "empty-stage", On: PointPreToolUse, Stage: "", Priority: 10,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			called = true
			return HookResult{Action: Allow}
		},
	})

	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)
	engine.Run(context.Background(), PointPreToolUse, &HookContext{
		SessionID: "sess1", WorkspacePath: "/tmp/ws", Stage: StageInitialGeneration,
	})

	if !called {
		t.Error("hook with empty Stage should match everything")
	}
}

func TestEngine_WarningFlow_UserPromptSubmitToInject(t *testing.T) {
	// Simulates the full warning flow:
	// 1. UserPromptSubmit hooks produce Warnings
	// 2. Warnings go into PendingWarnings via AddPendingWarnings
	// 3. QualityInject drains them and injects into messages

	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)
	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)

	// Step 1: Run UserPromptSubmit with a Warn hook (like skill-loading-injector).
	engine.Register(&RegisteredHook{
		Name: "test-injector", On: PointUserPromptSubmit, Stage: "always", Priority: 15,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			return HookResult{Action: Warn, Reason: "load grapesjs-html-compliance first"}
		},
	})

	hctx := &HookContext{
		SessionID:     "sess1",
		WorkspacePath: "/tmp/ws",
		Stage:         StageInitialGeneration,
		Round:         0,
		SessionState:  engine.State(),
		Config:        cfg,
	}

	warnings, err := engine.Run(context.Background(), PointUserPromptSubmit, hctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected at least 1 warning from UserPromptSubmit")
	}

	// Step 2: Simulate main.go storing warnings into PendingWarnings.
	if len(warnings) > 0 && engine.State() != nil {
		engine.State().AddPendingWarnings(warnings)
	}

	// Step 3: Simulate PreContextAssemble with QualityInject.
	engine.Register(&RegisteredHook{
		Name: "quality-inject", On: PointPreContextAssemble, Stage: "always", Priority: 5,
		Builtin: true,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			s := hctx.SessionState
			if s == nil {
				return HookResult{Action: Allow}
			}
			allWarnings := s.DrainPendingWarnings()
			if len(allWarnings) == 0 {
				return HookResult{Action: Allow}
			}
			combined := ""
			for _, w := range allWarnings {
				combined += w + "\n"
			}
			return HookResult{Action: Warn, Reason: combined}
		},
	})

	qctx := &HookContext{
		SessionID:    "sess1",
		SessionState: engine.State(),
		Config:       cfg,
	}

	qWarnings, err := engine.Run(context.Background(), PointPreContextAssemble, qctx)
	if err != nil {
		t.Fatalf("unexpected error from QualityInject: %v", err)
	}
	if len(qWarnings) == 0 {
		t.Fatal("expected QualityInject to return warnings")
	}

	// Step 4: Verify PendingWarnings are drained after QualityInject.
	if len(engine.State().DrainPendingWarnings()) != 0 {
		t.Error("expected PendingWarnings to be empty after QualityInject")
	}
}

func TestEngine_WarningFlow_NoWarnings(t *testing.T) {
	// When no hooks produce warnings, flow should be clean.
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)
	engine.InitState("sess1", "/tmp/ws", StageInitialGeneration)

	engine.Register(&RegisteredHook{
		Name: "allow-only", On: PointUserPromptSubmit, Stage: "always", Priority: 10,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			return HookResult{Action: Allow}
		},
	})

	hctx := &HookContext{
		SessionID:     "sess1",
		WorkspacePath: "/tmp/ws",
		Stage:         StageInitialGeneration,
		SessionState:  engine.State(),
		Config:        cfg,
	}

	warnings, err := engine.Run(context.Background(), PointUserPromptSubmit, hctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
	if len(engine.State().DrainPendingWarnings()) != 0 {
		t.Error("expected no pending warnings")
	}
}

func TestEngine_WarningFormat(t *testing.T) {
	// formatWarning wraps hook name and reason.
	result := formatWarning("my-hook", "this is a warning")
	if result == "" {
		t.Fatal("expected non-empty formatted warning")
	}
	if !containsStr(result, "[System]") {
		t.Error("expected [System] prefix")
	}
	if !containsStr(result, "my-hook") {
		t.Error("expected hook name in warning")
	}
	if !containsStr(result, "this is a warning") {
		t.Error("expected reason in warning")
	}
}

func TestMatchesAnyGlob_EmptyPatterns(t *testing.T) {
	if matchesAnyGlob("test.html", []string{}) {
		t.Error("should not match with empty patterns")
	}
}

func TestHashToolArgs_NilArgs(t *testing.T) {
	result := HashToolArgs(nil)
	if result != "0" {
		t.Errorf("expected '0' for nil args, got %q", result)
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func boolPtr(b bool) *bool { return &b }

// ---------------------------------------------------------------------------
// RecordRoundEnd integration — TrackRoundEnd ordering + loop_guard hook
// ---------------------------------------------------------------------------

// TestRecordRoundEnd_Ordering verifies that TrackRoundEnd (counter update)
// runs BEFORE PointPostRound hooks, so loop_guard sees the correct counter
// for the just-completed round.
func TestRecordRoundEnd_Ordering(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)
	engine.InitState("sess1", "/tmp/ws", StageIterativeEdit)

	// Register a hook that captures the counter value at PointPostRound.
	var observedCounter int
	engine.Register(&RegisteredHook{
		Name:     "test-ordering-hook",
		On:       PointPostRound,
		Stage:    "always",
		Priority: 10,
		Fn: func(_ context.Context, hctx *HookContext) HookResult {
			if hctx.SessionState != nil {
				observedCounter = hctx.SessionState.ConsecutiveRoundsNoWrite
			}
			return HookResult{Action: Allow}
		},
	})

	s := engine.State()

	// Round 1: no HTML write. Counter should be 1 after TrackRoundEnd,
	// and the hook should observe 1 (not 0).
	s.RecordRoundStart()
	engine.RecordRoundEnd(context.Background(), 1)
	if observedCounter != 1 {
		t.Errorf("round 1: hook observed counter %d, expected 1 (TrackRoundEnd must run BEFORE PointPostRound)", observedCounter)
	}
	if s.ConsecutiveRoundsNoWrite != 1 {
		t.Errorf("round 1: counter is %d, expected 1", s.ConsecutiveRoundsNoWrite)
	}

	// Round 2: no HTML write.
	s.RecordRoundStart()
	engine.RecordRoundEnd(context.Background(), 2)
	if observedCounter != 2 {
		t.Errorf("round 2: hook observed counter %d, expected 2", observedCounter)
	}

	// Round 3: no HTML write.
	s.RecordRoundStart()
	engine.RecordRoundEnd(context.Background(), 3)
	if observedCounter != 3 {
		t.Errorf("round 3: hook observed counter %d, expected 3", observedCounter)
	}
}

// TestRecordRoundEnd_LoopGuardIntegration verifies the full loop_guard flow
// through the engine: after 3 rounds of read-only tools with no HTML writes,
// the point_post_round hook should fire a warning.
func TestRecordRoundEnd_LoopGuardIntegration(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)
	engine.InitState("sess1", "/tmp/ws", StageIterativeEdit)

	// Register post_round_loop_guard (same logic as builtin).
	engine.Register(&RegisteredHook{
		Name:     "loop-guard-test",
		On:       PointPostRound,
		Stage:    "always",
		Priority: 10,
		Fn: func(_ context.Context, hctx *HookContext) HookResult {
			if hctx.SessionState == nil {
				return HookResult{Action: Allow}
			}
			if hctx.SessionState.ConsecutiveRoundsNoWrite >= 3 &&
				hctx.SessionState.AllLastRoundReadOnly() {
				return HookResult{Action: Warn, Reason: "loop_guard: read-only spiral detected"}
			}
			return HookResult{Action: Allow}
		},
	})

	s := engine.State()

	// Simulate 3 rounds, each with only read-only tools and no HTML writes.
	for round := 1; round <= 3; round++ {
		s.RecordRoundStart()
		// Simulate read-only tool calls.
		s.RecordToolStartCalled("read_file")
		s.RecordToolStartCalled("grep_search")
		warnings := engine.RecordRoundEnd(context.Background(), round)
		if round < 3 {
			if len(warnings) > 0 {
				t.Errorf("round %d: expected no warnings, got %v", round, warnings)
			}
		} else {
			if len(warnings) == 0 {
				t.Error("round 3: expected loop_guard warning")
			}
		}
	}
}

// TestRecordRoundEnd_ResetAfterHTMLWrite verifies that ConsecutiveRoundsNoWrite
// resets to 0 after a round with an HTML write.
func TestRecordRoundEnd_ResetAfterHTMLWrite(t *testing.T) {
	cfg := DefaultConfig()
	engine := NewEngineWithConfig(cfg)
	engine.InitState("sess1", "/tmp/ws", StageIterativeEdit)

	s := engine.State()

	// 2 rounds with no writes.
	for round := 1; round <= 2; round++ {
		s.RecordRoundStart()
		s.RecordToolStartCalled("read_file")
		engine.RecordRoundEnd(context.Background(), round)
	}
	if s.ConsecutiveRoundsNoWrite != 2 {
		t.Fatalf("expected counter=2 after 2 no-write rounds, got %d", s.ConsecutiveRoundsNoWrite)
	}

	// Round 3: writes HTML.
	s.RecordRoundStart()
	s.RecordToolStartCalled("edit_file")
	// Simulate HTML write via RecordToolCall.
	s.RecordToolCall("write_file", map[string]any{"path": "deck.html"}, false, nil)
	engine.RecordRoundEnd(context.Background(), 3)
	if s.ConsecutiveRoundsNoWrite != 0 {
		t.Errorf("expected counter=0 after round with HTML write, got %d", s.ConsecutiveRoundsNoWrite)
	}
}
