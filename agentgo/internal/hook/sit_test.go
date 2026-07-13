package hook

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentgo/internal/model"
)

// ============================================================================
// SIT: Hook engine + safepath integration
// ============================================================================

func TestSIT_SafePath_ResolveWorkspacePath(t *testing.T) {
	tmpDir := t.TempDir()
	workspacePath := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(workspacePath, 0755)

	// Create a file inside workspace.
	testFile := filepath.Join(workspacePath, "test.html")
	os.WriteFile(testFile, []byte("<html></html>"), 0644)

	// Resolve within workspace.
	resolved, err := ResolveWorkspacePath(workspacePath, "test.html")
	if err != nil {
		t.Fatalf("ResolveWorkspacePath failed: %v", err)
	}
	if resolved != testFile {
		t.Errorf("expected %q, got %q", testFile, resolved)
	}

	// Resolve absolute path within workspace.
	resolvedAbs, err := ResolveWorkspacePath(workspacePath, testFile)
	if err != nil {
		t.Fatalf("ResolveWorkspacePath with abs path failed: %v", err)
	}
	if resolvedAbs != testFile {
		t.Errorf("expected %q, got %q", testFile, resolvedAbs)
	}
}

func TestSIT_SafePath_EscapeRejected(t *testing.T) {
	tmpDir := t.TempDir()
	workspacePath := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(workspacePath, 0755)

	// Try to escape via relative path.
	_, err := ResolveWorkspacePath(workspacePath, "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path escape")
	}
	if !strings.Contains(err.Error(), "outside the workspace") {
		t.Errorf("expected 'outside the workspace' error, got: %v", err)
	}

	// Try to escape via absolute path.
	_, err = ResolveWorkspacePath(workspacePath, "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for absolute path outside workspace")
	}
	if !strings.Contains(err.Error(), "outside the workspace") {
		t.Errorf("expected 'outside the workspace' error, got: %v", err)
	}
}

func TestSIT_SafePath_EmptyPath(t *testing.T) {
	_, err := ResolveWorkspacePath("/tmp/ws", "")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if !strings.Contains(err.Error(), "path is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSIT_SafePath_EmptyWorkspace(t *testing.T) {
	// Empty workspace should resolve to absolute path.
	tmpDir := t.TempDir()
	absFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(absFile, []byte("hello"), 0644)

	resolved, err := ResolveWorkspacePath("", absFile)
	if err != nil {
		t.Fatalf("ResolveWorkspacePath with empty workspace failed: %v", err)
	}
	if resolved != absFile {
		t.Errorf("expected %q, got %q", absFile, resolved)
	}
}

func TestSIT_IsPathSafe_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	ws := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(ws, 0755)

	// File inside workspace.
	f := filepath.Join(ws, "index.html")
	os.WriteFile(f, []byte("hello"), 0644)

	if !IsPathSafe(ws, f) {
		t.Error("expected path inside workspace to be safe")
	}
}

func TestSIT_IsPathSafe_RejectsOutside(t *testing.T) {
	tmpDir := t.TempDir()
	ws := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(ws, 0755)

	if IsPathSafe(ws, "/etc/hosts") {
		t.Error("expected /etc/hosts to be unsafe")
	}
	if IsPathSafe(ws, "../secret.txt") {
		t.Error("expected relative escape to be unsafe")
	}
}

func TestSIT_IsPathSafe_EmptyRoot(t *testing.T) {
	if IsPathSafe("", "/tmp/test.txt") {
		t.Error("expected false for empty workspace root")
	}
}

func TestSIT_IsPathSafe_SymlinkRejection(t *testing.T) {
	tmpDir := t.TempDir()
	ws := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(ws, 0755)

	// Create a real file.
	realFile := filepath.Join(ws, "real.txt")
	os.WriteFile(realFile, []byte("real"), 0644)

	// Create a symlink.
	symlink := filepath.Join(ws, "link.txt")
	os.Symlink(realFile, symlink)

	if IsPathSafe(ws, symlink) {
		t.Error("expected symlink to be rejected by IsPathSafe")
	}
}

func TestSIT_ReadFileWithCap_Normal(t *testing.T) {
	tmpDir := t.TempDir()
	ws := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(ws, 0755)

	f := filepath.Join(ws, "test.txt")
	os.WriteFile(f, []byte("Hello, World!"), 0644)

	content := ReadFileWithCap(f, 100, ws)
	if content != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", content)
	}
}

func TestSIT_ReadFileWithCap_ExceedsLimit(t *testing.T) {
	tmpDir := t.TempDir()
	ws := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(ws, 0755)

	f := filepath.Join(ws, "large.txt")
	os.WriteFile(f, []byte(strings.Repeat("x", 1000)), 0644)

	content := ReadFileWithCap(f, 100, ws)
	if content != "" {
		t.Error("expected empty string when file exceeds cap")
	}
}

func TestSIT_ReadFileWithCap_OutsideWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	ws := filepath.Join(tmpDir, "workspace")
	os.MkdirAll(ws, 0755)

	content := ReadFileWithCap("/etc/hosts", 1000, ws)
	if content != "" {
		t.Error("expected empty string for file outside workspace")
	}
}

// ============================================================================
// SIT: Engine + Config + Run integration
// ============================================================================

func TestSIT_Engine_RunWithNoHooks(t *testing.T) {
	engine := NewEngineWithConfig(DefaultConfig())
	engine.InitState("sess-1", "/tmp/ws", "deck")

	hctx := &HookContext{
		SessionID:     "sess-1",
		WorkspacePath: "/tmp/ws",
		Stage:         "deck",
		Config:        DefaultConfig(),
	}
	warnings, err := engine.Run(context.Background(), PointPreToolUse, hctx)
	if err != nil {
		t.Fatalf("unexpected error with no hooks: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d", len(warnings))
	}
}

func TestSIT_Engine_BlockAction(t *testing.T) {
	engine := NewEngineWithConfig(DefaultConfig())

	engine.Register(&RegisteredHook{
		Name:     "blocker",
		On:       PointPreToolUse,
		Stage:    "always",
		Priority: 1,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			return HookResult{Action: Block, Reason: "not allowed"}
		},
	})

	engine.InitState("sess-1", "/tmp/ws", "deck")
	hctx := &HookContext{
		SessionID:     "sess-1",
		WorkspacePath: "/tmp/ws",
		Stage:         "deck",
		Config:        DefaultConfig(),
	}

	_, err := engine.Run(context.Background(), PointPreToolUse, hctx)
	if err == nil {
		t.Fatal("expected Block action to return error")
	}
	if !IsBlockedError(err) {
		t.Errorf("expected BlockedError, got %T: %v", err, err)
	}
	blocked := err.(*BlockedError)
	if blocked.HookName != "blocker" {
		t.Errorf("expected hook name 'blocker', got %q", blocked.HookName)
	}
	if !strings.Contains(blocked.Reason, "not allowed") {
		t.Errorf("expected reason to contain 'not allowed', got %q", blocked.Reason)
	}
}

func TestSIT_Engine_WarnAction(t *testing.T) {
	engine := NewEngineWithConfig(DefaultConfig())

	engine.Register(&RegisteredHook{
		Name:     "warner",
		On:       PointPreToolUse,
		Stage:    "always",
		Priority: 1,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			return HookResult{Action: Warn, Reason: "be careful"}
		},
	})

	engine.InitState("sess-1", "/tmp/ws", "deck")
	hctx := &HookContext{
		SessionID:     "sess-1",
		WorkspacePath: "/tmp/ws",
		Stage:         "deck",
		Config:        DefaultConfig(),
	}

	warnings, err := engine.Run(context.Background(), PointPreToolUse, hctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "be careful") {
		t.Errorf("unexpected warning: %s", warnings[0])
	}
}

func TestSIT_Engine_BlockBeforeWarn(t *testing.T) {
	engine := NewEngineWithConfig(DefaultConfig())

	// Register warn first (lower priority = runs first).
	engine.Register(&RegisteredHook{
		Name:     "warner",
		On:       PointPreToolUse,
		Stage:    "always",
		Priority: 1,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			return HookResult{Action: Warn, Reason: "warning"}
		},
	})
	// Register block after.
	engine.Register(&RegisteredHook{
		Name:     "blocker",
		On:       PointPreToolUse,
		Stage:    "always",
		Priority: 2,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			return HookResult{Action: Block, Reason: "blocked"}
		},
	})

	engine.InitState("sess-1", "/tmp/ws", "deck")
	hctx := &HookContext{
		SessionID:     "sess-1",
		WorkspacePath: "/tmp/ws",
		Stage:         "deck",
		Config:        DefaultConfig(),
	}

	warnings, err := engine.Run(context.Background(), PointPreToolUse, hctx)
	if err == nil {
		t.Fatal("expected error from block hook")
	}
	// The warn hook runs first and collects the warning before block stops execution.
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning from warn hook before block, got %d", len(warnings))
	}
}

func TestSIT_Engine_FilterByToolName(t *testing.T) {
	engine := NewEngineWithConfig(DefaultConfig())

	engine.Register(&RegisteredHook{
		Name:     "write-only-check",
		On:       PointPreToolUse,
		Stage:    "always",
		Priority: 10,
		Matcher:  &Matcher{ToolNames: []string{"write_file"}},
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			return HookResult{Action: Block, Reason: "write blocked"}
		},
	})

	engine.InitState("sess-1", "/tmp/ws", "deck")

	// Read should be allowed (no match).
	readCtx := &HookContext{
		SessionID:     "sess-1",
		WorkspacePath: "/tmp/ws",
		Stage:         "deck",
		ToolName:      "read_file",
		Config:        DefaultConfig(),
	}
	_, err := engine.Run(context.Background(), PointPreToolUse, readCtx)
	if err != nil {
		t.Fatalf("read should not be blocked by write-only hook: %v", err)
	}

	// Write should be blocked.
	writeCtx := &HookContext{
		SessionID:     "sess-1",
		WorkspacePath: "/tmp/ws",
		Stage:         "deck",
		ToolName:      "write_file",
		Config:        DefaultConfig(),
	}
	_, err = engine.Run(context.Background(), PointPreToolUse, writeCtx)
	if err == nil {
		t.Fatal("expected write to be blocked")
	}
}

func TestSIT_Engine_MultipleMountPoints(t *testing.T) {
	engine := NewEngineWithConfig(DefaultConfig())

	preSubmitCalled := false
	preToolCalled := false

	engine.Register(&RegisteredHook{
		Name: "pre-submit-hook", On: PointUserPromptSubmit, Stage: "always", Priority: 1,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			preSubmitCalled = true
			return HookResult{Action: Allow}
		},
	})
	engine.Register(&RegisteredHook{
		Name: "pre-tool-hook", On: PointPreToolUse, Stage: "always", Priority: 1,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			preToolCalled = true
			return HookResult{Action: Allow}
		},
	})

	engine.InitState("sess-1", "/tmp/ws", "deck")

	// Run pre-submit.
	hctx := &HookContext{SessionID: "sess-1", WorkspacePath: "/tmp/ws", Stage: "deck", Config: DefaultConfig()}
	engine.Run(context.Background(), PointUserPromptSubmit, hctx)
	if !preSubmitCalled {
		t.Error("pre-submit hook should have been called")
	}
	if preToolCalled {
		t.Error("pre-tool hook should NOT have been called")
	}

	// Run pre-tool.
	engine.Run(context.Background(), PointPreToolUse, hctx)
	if !preToolCalled {
		t.Error("pre-tool hook should have been called")
	}
}

func TestSIT_Engine_StageFiltering(t *testing.T) {
	engine := NewEngineWithConfig(DefaultConfig())

	deckOnlyCalled := false

	engine.Register(&RegisteredHook{
		Name:     "deck-only",
		On:       PointPreToolUse,
		Stage:    "deck",
		Priority: 1,
		Fn: func(ctx context.Context, hctx *HookContext) HookResult {
			deckOnlyCalled = true
			return HookResult{Action: Allow}
		},
	})

	engine.InitState("sess-1", "/tmp/ws", "landing")
	cfg := DefaultConfig()

	// Run with "landing" stage — deck-only hook should NOT fire.
	hctx := &HookContext{SessionID: "sess-1", WorkspacePath: "/tmp/ws", Stage: "landing", Config: cfg}
	engine.Run(context.Background(), PointPreToolUse, hctx)
	if deckOnlyCalled {
		t.Error("deck-only hook should NOT fire for landing stage")
	}

	// Run with "deck" stage — deck-only hook SHOULD fire.
	engine.InitState("sess-1", "/tmp/ws", "deck")
	hctx.Stage = "deck"
	deckOnlyCalled = false
	engine.Run(context.Background(), PointPreToolUse, hctx)
	if !deckOnlyCalled {
		t.Error("deck-only hook SHOULD fire for deck stage")
	}
}

func TestSIT_InjectWarnings_IntoMessages(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Hello"},
	}

	warnings := []string{"Warning: file too large", "Note: use section not div"}
	result := InjectWarnings(messages, warnings)

	// Now appended to system message, not inserted as separate messages.
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Error("expected system message first")
	}
	if result[1].Role != "user" || result[1].Content != "Hello" {
		t.Error("expected original user message last")
	}
	// Both warnings should be present in the system content.
	hasFileWarning := strings.Contains(result[0].Content, "file too large")
	hasSectionWarning := strings.Contains(result[0].Content, "section not div")
	if !hasFileWarning || !hasSectionWarning {
		t.Errorf("expected both warnings in system content, got: %s", result[0].Content)
	}
}

func TestSIT_InjectWarnings_NoSystemMessage(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Hello"},
	}
	warnings := []string{"Watch out"}
	result := InjectWarnings(messages, warnings)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("expected warning as system message, got role=%s", result[0].Role)
	}
	if !strings.Contains(result[0].Content, "Watch out") {
		t.Errorf("unexpected first message: %s", result[0].Content)
	}
	if result[1].Role != "user" || result[1].Content != "Hello" {
		t.Errorf("unexpected second message: %s", result[1].Content)
	}
}

func TestSIT_InjectWarnings_EmptyWarning(t *testing.T) {
	messages := []model.Message{
		{Role: "user", Content: "Hello"},
	}
	warnings := []string{"", "  ", "Real warning"}
	result := InjectWarnings(messages, warnings)

	if len(result) != 2 {
		t.Fatalf("expected 2 messages (empty warnings skipped), got %d", len(result))
	}
}

func TestSIT_InjectWarnings_EmptyMessages(t *testing.T) {
	result := InjectWarnings(nil, []string{"warning"})
	if len(result) != 0 {
		t.Errorf("expected 0 messages for nil input, got %d", len(result))
	}
}

func TestSIT_IsBlockedError(t *testing.T) {
	if IsBlockedError(nil) {
		t.Error("nil is not a BlockedError")
	}
	blocked := &BlockedError{HookName: "test", Reason: "nope"}
	if !IsBlockedError(blocked) {
		t.Error("BlockedError should be detected")
	}
}
