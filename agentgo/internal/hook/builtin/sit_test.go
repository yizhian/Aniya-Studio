package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentgo/internal/hook"
)

// ============================================================================
// SIT: Builtin hook registration and execution pipeline
// ============================================================================

func TestSIT_RegisterBuiltins_AllHooksRegistered(t *testing.T) {
	engine := hook.NewEngine("")

	RegisterBuiltins(engine)

	// Smoke: verify engine was populated by running hooks at each mount point.
	ctx := &hook.HookContext{
		Stage:         hook.StageInitialGeneration,
		WorkspacePath: t.TempDir(),
		SessionState:  hook.NewSessionState("sess1", "/ws", hook.StageInitialGeneration),
		Config:        hook.DefaultConfig(),
		ToolName:      "write_file",
		ToolArgs:      map[string]any{"path": "test.html"},
	}

	// User prompt submit — init-check and skill-loading-injector should fire.
	_, err := engine.Run(context.Background(), hook.PointUserPromptSubmit, ctx)
	if err != nil {
		t.Fatalf("unexpected block at user_prompt_submit: %v", err)
	}

	// Pre-tool-use — file-type-whitelist should block .exe.
	ctx.ToolArgs = map[string]any{"path": "malware.exe"}
	_, err = engine.Run(context.Background(), hook.PointPreToolUse, ctx)
	if err == nil {
		t.Error("expected file-type-whitelist to block .exe")
	}
}

func TestSIT_HookPipeline_WriteFileFlow(t *testing.T) {
	dir := t.TempDir()
	engine := hook.NewEngine("")
	RegisterBuiltins(engine)

	ctx := &hook.HookContext{
		Stage:         hook.StageInitialGeneration,
		WorkspacePath: dir,
		SessionState:  hook.NewSessionState("sess1", dir, hook.StageInitialGeneration),
		Config:        hook.DefaultConfig(),
		ToolName:      "write_file",
	}

	// Stage 1: Pre-tool-use for HTML write without skill — blocks.
	ctx.ToolArgs = map[string]any{"path": "slide.html"}
	_, err := engine.Run(context.Background(), hook.PointPreToolUse, ctx)
	if err == nil {
		t.Error("expected design-skill-required to block HTML write without skill")
	} else if !strings.Contains(err.Error(), "grapesjs-html-compliance") {
		t.Errorf("expected block reason to mention grapesjs-html-compliance, got: %v", err)
	}

	// Stage 2: Load compliance skill — write allowed.
	ctx.SessionState.SkillsLoaded = map[string]bool{"grapesjs-html-compliance": true}
	_, err = engine.Run(context.Background(), hook.PointPreToolUse, ctx)
	if err != nil {
		t.Errorf("unexpected block after skill loaded: %v", err)
	}

	// Stage 3: Post-tool-use — compliance-review-trigger checks HTML.
	ctx.ToolResult = &hook.ToolResultInfo{IsError: false}
	ctx.ToolArgs = map[string]any{
		"path":    "slide.html",
		"content": "<html><body><table></table></body></html>",
	}
	postWarnings, err := engine.Run(context.Background(), hook.PointPostToolUse, ctx)
	if err != nil {
		t.Errorf("unexpected block at post_tool_use: %v", err)
	}
	foundCompliance := false
	for _, w := range postWarnings {
		if strings.Contains(w, "[htmlchecker]") {
			foundCompliance = true
		}
	}
	if !foundCompliance {
		t.Error("expected compliance warning for invalid HTML")
	}
}

func TestSIT_HookPipeline_EditFile_ReadProof(t *testing.T) {
	dir := t.TempDir()
	engine := hook.NewEngine("")
	RegisterBuiltins(engine)

	targetFile := filepath.Join(dir, "deck.html")
	os.WriteFile(targetFile, []byte("<html></html>"), 0644)

	ctx := &hook.HookContext{
		Stage:         hook.StageIterativeEdit,
		WorkspacePath: dir,
		SessionState:  hook.NewSessionState("sess1", dir, hook.StageIterativeEdit),
		Config:        hook.DefaultConfig(),
		ToolName:      "edit_file",
		ToolArgs:      map[string]any{"path": "deck.html"},
	}

	// Should block because file hasn't been read.
	_, err := engine.Run(context.Background(), hook.PointPreToolUse, ctx)
	if err == nil {
		t.Error("expected read-proof-pre-check to block edit of unread file")
	}

	// Mark file as read — should allow.
	ctx.SessionState.FilesRead = map[string]int64{
		targetFile: 1000,
	}
	_, err = engine.Run(context.Background(), hook.PointPreToolUse, ctx)
	if err != nil {
		t.Errorf("unexpected block after file read: %v", err)
	}
}

func TestSIT_ComplianceTrigger_InvalidHTML_Warns(t *testing.T) {
	dir := t.TempDir()
	engine := hook.NewEngine("")
	RegisterBuiltins(engine)

	ctx := &hook.HookContext{
		Stage:         hook.StageInitialGeneration,
		WorkspacePath: dir,
		SessionState: &hook.SessionState{
			Stage:        hook.StageInitialGeneration,
			SkillsLoaded: map[string]bool{"grapesjs-html-compliance": true},
		},
		Config:   hook.DefaultConfig(),
		ToolName: "write_file",
		ToolArgs: map[string]any{
			"path":    "slide.html",
			"content": "<html><body><table></table></body></html>",
		},
		ToolResult: &hook.ToolResultInfo{IsError: false},
	}

	warnings, err := engine.Run(context.Background(), hook.PointPostToolUse, ctx)
	if err != nil {
		t.Fatalf("unexpected block: %v", err)
	}

	foundS1, foundS3 := false, false
	for _, w := range warnings {
		if strings.Contains(w, "S1:") {
			foundS1 = true
		}
		if strings.Contains(w, "S3:") {
			foundS3 = true
		}
	}
	if !foundS1 {
		t.Error("expected compliance warning to mention S1 (DOCTYPE)")
	}
	if !foundS3 {
		t.Error("expected compliance warning to mention S3 (bare table)")
	}
}

func TestSIT_HookPipeline_ConsecutiveFailureWarning(t *testing.T) {
	dir := t.TempDir()
	engine := hook.NewEngine("")
	RegisterBuiltins(engine)

	ctx := &hook.HookContext{
		Stage:         hook.StageInitialGeneration,
		WorkspacePath: dir,
		SessionState: &hook.SessionState{
			Stage:                  hook.StageInitialGeneration,
			ToolsFailed:            map[string]int{"write_file": 3},
			MaxConsecutiveFailures: 2,
		},
		Config:     hook.DefaultConfig(),
		ToolName:   "write_file",
		ToolResult: &hook.ToolResultInfo{IsError: true},
	}

	warnings, err := engine.Run(context.Background(), hook.PointPostToolUse, ctx)
	if err != nil {
		t.Fatalf("unexpected block: %v", err)
	}

	foundFailureWarn := false
	for _, w := range warnings {
		if strings.Contains(w, "write_file") {
			foundFailureWarn = true
		}
	}
	if !foundFailureWarn {
		t.Error("expected consecutive-failure-detector to warn after 3 failures")
	}
}

func TestSIT_QualityInject_DrainsWarnings(t *testing.T) {
	engine := hook.NewEngine("")
	RegisterBuiltins(engine)

	ctx := &hook.HookContext{
		SessionState: &hook.SessionState{
			PendingWarnings: []string{"warning 1", "warning 2", ""},
		},
		Config: hook.DefaultConfig(),
	}

	warnings, err := engine.Run(context.Background(), hook.PointPreContextAssemble, ctx)
	if err != nil {
		t.Fatalf("unexpected block: %v", err)
	}

	hasWarnings := false
	for _, w := range warnings {
		if strings.Contains(w, "warning 1") {
			hasWarnings = true
		}
	}
	if !hasWarnings {
		t.Error("expected quality-inject to warn with pending warnings")
	}

	if len(ctx.SessionState.PendingWarnings) > 0 {
		t.Errorf("expected drained warnings, got %d remaining", len(ctx.SessionState.PendingWarnings))
	}
}

func TestSIT_HookPipeline_NoBlockOnNonHTMLWrite(t *testing.T) {
	dir := t.TempDir()
	engine := hook.NewEngine("")
	RegisterBuiltins(engine)

	ctx := &hook.HookContext{
		Stage:         hook.StageInitialGeneration,
		WorkspacePath: dir,
		SessionState:  hook.NewSessionState("sess1", dir, hook.StageInitialGeneration),
		Config:        hook.DefaultConfig(),
		ToolName:      "write_file",
		ToolArgs:      map[string]any{"path": "notes.txt"},
	}

	_, err := engine.Run(context.Background(), hook.PointPreToolUse, ctx)
	if err != nil {
		t.Errorf("expected no block for .txt write, got: %v", err)
	}
}

func TestSIT_SkillLoadingInjector_StageBehavior(t *testing.T) {
	engine := hook.NewEngine("")
	RegisterBuiltins(engine)

	dir := t.TempDir()
	cfg := hook.DefaultConfig()

	// Initial generation: should NOT have edit-mode warning.
	ctx := &hook.HookContext{
		Stage:         hook.StageInitialGeneration,
		WorkspacePath: dir,
		Config:        cfg,
	}
	warnings, err := engine.Run(context.Background(), hook.PointUserPromptSubmit, ctx)
	if err != nil {
		t.Fatalf("unexpected block: %v", err)
	}
	for _, w := range warnings {
		if strings.Contains(w, "[edit-mode]") {
			t.Error("initial generation should not have edit-mode warning")
		}
	}

	// Iterative edit: should warn with [edit-mode].
	ctx.Stage = hook.StageIterativeEdit
	warnings, err = engine.Run(context.Background(), hook.PointUserPromptSubmit, ctx)
	if err != nil {
		t.Fatalf("unexpected block: %v", err)
	}
	foundEditMode := false
	for _, w := range warnings {
		if strings.Contains(w, "[edit-mode]") {
			foundEditMode = true
		}
	}
	if !foundEditMode {
		t.Error("iterative edit should have [edit-mode] warning")
	}
}
