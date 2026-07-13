package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentgo/internal/hook"
)

// ---------------------------------------------------------------------------
// InitCheck
// ---------------------------------------------------------------------------

func TestInitCheck_MissingDir(t *testing.T) {
	hctx := &hook.HookContext{
		WorkspacePath: "/tmp/nonexistent_workspace_xyz123",
	}
	result := InitCheck(context.Background(), hctx)
	if result.Action != hook.Block {
		t.Errorf("expected Block for missing workspace, got %s", result.Action)
	}
}

func TestInitCheck_ExistingDir(t *testing.T) {
	dir := t.TempDir()
	hctx := &hook.HookContext{
		WorkspacePath: dir,
	}
	result := InitCheck(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for existing workspace, got %s: %s", result.Action, result.Reason)
	}
}

func TestInitCheck_EmptyPath(t *testing.T) {
	hctx := &hook.HookContext{
		WorkspacePath: "",
	}
	result := InitCheck(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for empty workspace path, got %s", result.Action)
	}
}

// ---------------------------------------------------------------------------
// SkillLoadingInjector — simplified: system prompt has full workflow.
// InitialGeneration → Allow, IterativeEdit → Warn with edit-mode.
// ---------------------------------------------------------------------------

func TestSkillLoadingInjector_InitialGeneration(t *testing.T) {
	hctx := &hook.HookContext{
		Stage: hook.StageInitialGeneration,
	}
	result := SkillLoadingInjector(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for initial generation, got %s", result.Action)
	}
}

func TestSkillLoadingInjector_IterativeEdit(t *testing.T) {
	hctx := &hook.HookContext{
		Stage: hook.StageIterativeEdit,
	}
	result := SkillLoadingInjector(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn for iterative edit, got %s", result.Action)
	}
	if !strings.Contains(result.Reason, "[edit-mode]") {
		t.Errorf("expected reason to contain [edit-mode], got: %s", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// FileTypeWhitelist
// ---------------------------------------------------------------------------

func TestFileTypeWhitelist_AllowedExtension(t *testing.T) {
	cfg := hook.DefaultConfig()
	hctx := &hook.HookContext{
		ToolName: "write_file",
		ToolArgs: map[string]any{"path": "test.html"},
		Config:   cfg,
	}

	result := FileTypeWhitelist(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for .html, got %s: %s", result.Action, result.Reason)
	}
}

func TestFileTypeWhitelist_BlockedExtension(t *testing.T) {
	cfg := hook.DefaultConfig()
	hctx := &hook.HookContext{
		ToolName: "write_file",
		ToolArgs: map[string]any{"path": "test.exe"},
		Config:   cfg,
	}

	result := FileTypeWhitelist(context.Background(), hctx)
	if result.Action != hook.Block {
		t.Errorf("expected Block for .exe, got %s", result.Action)
	}
}

func TestFileTypeWhitelist_NoExtension(t *testing.T) {
	cfg := hook.DefaultConfig()
	hctx := &hook.HookContext{
		ToolName: "write_file",
		ToolArgs: map[string]any{"path": "Makefile"},
		Config:   cfg,
	}

	result := FileTypeWhitelist(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn for no extension, got %s", result.Action)
	}
}

func TestFileTypeWhitelist_EmptyPath(t *testing.T) {
	cfg := hook.DefaultConfig()
	hctx := &hook.HookContext{
		ToolName: "write_file",
		ToolArgs: map[string]any{},
		Config:   cfg,
	}
	result := FileTypeWhitelist(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for empty path, got %s", result.Action)
	}
}

func TestFileTypeWhitelist_NoAllowedExtensions(t *testing.T) {
	cfg := &hook.Config{
		Settings: hook.Settings{
			AllowedExtensions: nil,
		},
	}
	hctx := &hook.HookContext{
		ToolName: "write_file",
		ToolArgs: map[string]any{"path": "anything.exe"},
		Config:   cfg,
	}
	result := FileTypeWhitelist(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when no extensions configured, got %s", result.Action)
	}
}

func TestFileTypeWhitelist_CustomMessage(t *testing.T) {
	cfg := &hook.Config{
		Settings: hook.Settings{
			AllowedExtensions: []string{".html"},
			Messages: hook.HookMessages{
				FileTypeBlock: "CUSTOM: cannot create %s, only %s",
			},
		},
	}
	hctx := &hook.HookContext{
		ToolName: "write_file",
		ToolArgs: map[string]any{"path": "test.exe"},
		Config:   cfg,
	}
	result := FileTypeWhitelist(context.Background(), hctx)
	if result.Action != hook.Block {
		t.Fatalf("expected Block, got %s", result.Action)
	}
	if result.Reason != "CUSTOM: cannot create .exe, only .html" {
		t.Errorf("expected custom message, got %q", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// ReadProofPreCheck
// ---------------------------------------------------------------------------

func TestReadProofPreCheck_FileNotRead(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	hctx := &hook.HookContext{
		ToolName:      "edit_file",
		ToolArgs:      map[string]any{"path": "index.html"},
		SessionState:  s,
		WorkspacePath: "/tmp/ws",
	}

	result := ReadProofPreCheck(context.Background(), hctx)
	if result.Action != hook.Block {
		t.Errorf("expected Block when file not read yet, got %s", result.Action)
	}
}

func TestReadProofPreCheck_FileWasRead(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	absPath, _ := hook.ResolveWorkspacePath("/tmp/ws", "index.html")
	s.FilesRead[absPath] = int64(1000)

	hctx := &hook.HookContext{
		ToolName:      "edit_file",
		ToolArgs:      map[string]any{"path": "index.html"},
		SessionState:  s,
		WorkspacePath: "/tmp/ws",
	}

	result := ReadProofPreCheck(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when file was read, got %s: %s", result.Action, result.Reason)
	}
}

func TestReadProofPreCheck_NoSessionState(t *testing.T) {
	hctx := &hook.HookContext{
		ToolName: "edit_file",
		ToolArgs: map[string]any{"path": "index.html"},
	}

	result := ReadProofPreCheck(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when no SessionState, got %s", result.Action)
	}
}

func TestReadProofPreCheck_StaleRead(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.html")
	os.WriteFile(testFile, []byte("<html></html>"), 0o644)

	s := hook.NewSessionState("sess1", dir, hook.StageInitialGeneration)
	absPath, _ := hook.ResolveWorkspacePath(dir, "test.html")
	s.FilesRead[absPath] = int64(1) // very old mtime

	hctx := &hook.HookContext{
		ToolName:      "edit_file",
		ToolArgs:      map[string]any{"path": "test.html"},
		SessionState:  s,
		WorkspacePath: dir,
	}

	result := ReadProofPreCheck(context.Background(), hctx)
	if result.Action != hook.Allow && result.Action != hook.Warn {
		t.Errorf("expected Allow or Warn, got %s", result.Action)
	}
}

func TestReadProofPreCheck_EmptyPath(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	hctx := &hook.HookContext{
		SessionState: s,
		ToolName:     "edit_file",
		ToolArgs:     map[string]any{},
	}
	result := ReadProofPreCheck(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when no path, got %s", result.Action)
	}
}

func TestReadProofPreCheck_PathNotString(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	hctx := &hook.HookContext{
		SessionState: s,
		ToolName:     "edit_file",
		ToolArgs:     map[string]any{"path": 42},
	}
	result := ReadProofPreCheck(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when path is not string, got %s", result.Action)
	}
}

func TestReadProofPreCheck_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	s := hook.NewSessionState("sess1", dir, hook.StageInitialGeneration)
	absPath, _ := hook.ResolveWorkspacePath(dir, "index.html")
	s.FilesRead[absPath] = int64(1000)

	hctx := &hook.HookContext{
		SessionState:  s,
		WorkspacePath: dir,
		ToolName:      "edit_file",
		ToolArgs:      map[string]any{"path": absPath},
	}
	result := ReadProofPreCheck(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for absolute path that was read, got %s: %s", result.Action, result.Reason)
	}
}

func TestReadProofPreCheck_CustomMessage(t *testing.T) {
	cfg := &hook.Config{
		Settings: hook.Settings{
			Messages: hook.HookMessages{
				ReadProofBlock: "CUSTOM: must read %q first",
			},
		},
	}
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	hctx := &hook.HookContext{
		ToolName:      "edit_file",
		ToolArgs:      map[string]any{"path": "index.html"},
		SessionState:  s,
		WorkspacePath: "/tmp/ws",
		Config:        cfg,
	}
	result := ReadProofPreCheck(context.Background(), hctx)
	if result.Action != hook.Block {
		t.Fatalf("expected Block, got %s", result.Action)
	}
	if result.Reason != `CUSTOM: must read "index.html" first` {
		t.Errorf("expected custom message, got %q", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// DesignSkillRequired
// ---------------------------------------------------------------------------

func TestDesignSkillRequired_NoSkill(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolArgs:     map[string]any{"path": "test.html"},
		SessionState: s,
		Stage:        hook.StageInitialGeneration,
	}
	result := DesignSkillRequired(context.Background(), hctx)
	if result.Action != hook.Block {
		t.Errorf("expected Block when skill not loaded, got %s", result.Action)
	}
}

func TestDesignSkillRequired_SkillLoaded(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	s.SkillsLoaded["grapesjs-html-compliance"] = true

	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolArgs:     map[string]any{"path": "test.html"},
		SessionState: s,
		Stage:        hook.StageInitialGeneration,
	}
	result := DesignSkillRequired(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when skill loaded, got %s: %s", result.Action, result.Reason)
	}
}

func TestDesignSkillRequired_IterativeEdit(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageIterativeEdit)
	// No skill loaded — stage is iterative_edit, so Allow.
	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolArgs:     map[string]any{"path": "test.html"},
		SessionState: s,
		Stage:        hook.StageIterativeEdit,
	}
	result := DesignSkillRequired(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for iterative edit stage, got %s", result.Action)
	}
}

func TestDesignSkillRequired_NilSessionState(t *testing.T) {
	hctx := &hook.HookContext{
		Stage: hook.StageInitialGeneration,
	}
	result := DesignSkillRequired(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when nil SessionState, got %s", result.Action)
	}
}

// ---------------------------------------------------------------------------
// ConsecutiveFailureDetector
// ---------------------------------------------------------------------------

func TestConsecutiveFailureDetector_BelowThreshold(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, true, nil)
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, true, nil)

	hctx := &hook.HookContext{
		ToolName:     "write_file",
		SessionState: s,
		ToolResult:   &hook.ToolResultInfo{IsError: true},
	}

	result := ConsecutiveFailureDetector(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when below threshold, got %s", result.Action)
	}
}

func TestConsecutiveFailureDetector_AtThreshold(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, true, nil)
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, true, nil)
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, true, nil)

	hctx := &hook.HookContext{
		ToolName:     "write_file",
		SessionState: s,
		ToolResult:   &hook.ToolResultInfo{IsError: true},
	}

	result := ConsecutiveFailureDetector(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn at threshold, got %s", result.Action)
	}
}

func TestConsecutiveFailureDetector_NonError(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	hctx := &hook.HookContext{
		SessionState: s,
		ToolResult:   &hook.ToolResultInfo{IsError: false},
	}
	result := ConsecutiveFailureDetector(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when no error, got %s", result.Action)
	}
}

func TestConsecutiveFailureDetector_NilToolResult(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	hctx := &hook.HookContext{
		SessionState: s,
	}
	result := ConsecutiveFailureDetector(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when nil ToolResult, got %s", result.Action)
	}
}

func TestConsecutiveFailureDetector_NilSessionState(t *testing.T) {
	hctx := &hook.HookContext{}
	result := ConsecutiveFailureDetector(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when nil SessionState, got %s", result.Action)
	}
}

func TestConsecutiveFailureDetector_CustomThreshold(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	s.MaxConsecutiveFailures = 2
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, true, nil)
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, true, nil)

	hctx := &hook.HookContext{
		ToolName:     "write_file",
		SessionState: s,
		ToolResult:   &hook.ToolResultInfo{IsError: true},
	}
	result := ConsecutiveFailureDetector(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn at custom threshold 2, got %s", result.Action)
	}
}

func TestConsecutiveFailureDetector_ZeroThreshold(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	s.MaxConsecutiveFailures = 0
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, true, nil)

	hctx := &hook.HookContext{
		ToolName:     "write_file",
		SessionState: s,
		ToolResult:   &hook.ToolResultInfo{IsError: true},
	}
	result := ConsecutiveFailureDetector(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow at failures=1 with default threshold, got %s", result.Action)
	}
}

func TestConsecutiveFailureDetector_CustomMessage(t *testing.T) {
	cfg := &hook.Config{
		Settings: hook.Settings{
			MaxConsecutiveFailures: 1,
			Messages: hook.HookMessages{
				ConsecutiveFailWarn: "CUSTOM: %q failed %d times",
			},
		},
	}
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	s.MaxConsecutiveFailures = 1
	s.RecordToolCall("bash", map[string]any{"cmd": "ls"}, true, nil)

	hctx := &hook.HookContext{
		ToolName:     "bash",
		SessionState: s,
		Config:       cfg,
		ToolResult:   &hook.ToolResultInfo{IsError: true},
	}
	result := ConsecutiveFailureDetector(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Fatalf("expected Warn, got %s", result.Action)
	}
	if result.Reason != `CUSTOM: "bash" failed 1 times` {
		t.Errorf("expected custom message, got %q", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// DuplicateCallDetector
// ---------------------------------------------------------------------------

func TestDuplicateCallDetector_AtThreshold(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	args := map[string]any{"path": "test.html", "content": "hello"}

	s.RecordToolCall("write_file", args, true, nil)
	s.RecordToolCall("write_file", args, true, nil)
	s.RecordToolCall("write_file", args, true, nil)

	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolArgs:     args,
		SessionState: s,
		ToolResult:   &hook.ToolResultInfo{IsError: true},
	}

	result := DuplicateCallDetector(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn at duplicate threshold, got %s", result.Action)
	}
}

func TestDuplicateCallDetector_BelowThreshold(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	args := map[string]any{"path": "test.html"}
	s.RecordToolCall("write_file", args, true, nil)
	s.RecordToolCall("write_file", args, true, nil)

	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolArgs:     args,
		SessionState: s,
		ToolResult:   &hook.ToolResultInfo{IsError: true},
	}
	result := DuplicateCallDetector(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow below threshold, got %s", result.Action)
	}
}

func TestDuplicateCallDetector_NonError(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	args := map[string]any{"path": "test.html"}
	s.RecordToolCall("write_file", args, true, nil)
	s.RecordToolCall("write_file", args, true, nil)
	s.RecordToolCall("write_file", args, true, nil)

	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolArgs:     args,
		SessionState: s,
		ToolResult:   &hook.ToolResultInfo{IsError: false},
	}
	result := DuplicateCallDetector(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for non-error duplicate, got %s", result.Action)
	}
}

func TestDuplicateCallDetector_NilSessionState(t *testing.T) {
	hctx := &hook.HookContext{ToolName: "write_file", ToolArgs: map[string]any{"path": "test.html"}}
	result := DuplicateCallDetector(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when nil SessionState, got %s", result.Action)
	}
}

func TestDuplicateCallDetector_NilToolResult(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	args := map[string]any{"path": "test.html"}
	s.RecordToolCall("write_file", args, true, nil)
	s.RecordToolCall("write_file", args, true, nil)
	s.RecordToolCall("write_file", args, true, nil)

	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolArgs:     args,
		SessionState: s,
		ToolResult:   nil,
	}
	result := DuplicateCallDetector(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when nil ToolResult, got %s", result.Action)
	}
}

func TestDuplicateCallDetector_CustomThreshold(t *testing.T) {
	cfg := &hook.Config{
		Settings: hook.Settings{
			DuplicateCallWarnCount: 2,
			Messages:               hook.DefaultConfig().Settings.Messages,
		},
	}
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	args := map[string]any{"path": "test.html"}
	s.RecordToolCall("write_file", args, true, nil)
	s.RecordToolCall("write_file", args, true, nil)

	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolArgs:     args,
		SessionState: s,
		Config:       cfg,
		ToolResult:   &hook.ToolResultInfo{IsError: true},
	}
	result := DuplicateCallDetector(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn at custom duplicate threshold 2, got %s", result.Action)
	}
}

// ---------------------------------------------------------------------------
// ComplianceReviewTrigger
// ---------------------------------------------------------------------------

func TestComplianceReviewTrigger_Success(t *testing.T) {
	hctx := &hook.HookContext{
		ToolResult: &hook.ToolResultInfo{IsError: false},
	}
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn after successful HTML write, got %s", result.Action)
	}
}

func TestComplianceReviewTrigger_Error(t *testing.T) {
	hctx := &hook.HookContext{
		ToolResult: &hook.ToolResultInfo{IsError: true},
	}
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when tool errored, got %s", result.Action)
	}
}

func TestComplianceReviewTrigger_NilToolResult(t *testing.T) {
	hctx := &hook.HookContext{}
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when nil ToolResult, got %s", result.Action)
	}
}

func TestComplianceReviewTrigger_SelfchecklistPublishFull_NoSessionState(t *testing.T) {
	// When SessionState is nil, count defaults to 0 → selfchecklist publish-full.
	hctx := &hook.HookContext{
		ToolResult: &hook.ToolResultInfo{IsError: false},
	}
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected selfchecklist publish-full when SessionState is nil, got %s", result.Action)
	}
	if len(result.Reason) < 50 {
		t.Errorf("expected full checklist (long message), got %d chars: %q", len(result.Reason), result.Reason)
	}
}

func TestComplianceReviewTrigger_SelfchecklistPublishFull_Count1(t *testing.T) {
	hctx := &hook.HookContext{
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
	}
	hctx.SessionState.HTMLPublishCount = 1
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected selfchecklist publish-full at count 1, got %s", result.Action)
	}
	if len(result.Reason) < 50 {
		t.Errorf("expected full checklist, got %d chars", len(result.Reason))
	}
}

func TestComplianceReviewTrigger_SelfchecklistPublishShort_Count2(t *testing.T) {
	hctx := &hook.HookContext{
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
	}
	hctx.SessionState.HTMLPublishCount = 2
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected selfchecklist publish-short at count 2, got %s", result.Action)
	}
	if len(result.Reason) < 100 {
		t.Errorf("expected full checklist, got %d bytes: %q", len(result.Reason), result.Reason)
	}
}

func TestComplianceReviewTrigger_SelfchecklistPublishShort_Count3(t *testing.T) {
	hctx := &hook.HookContext{
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
	}
	hctx.SessionState.HTMLPublishCount = 3
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected selfchecklist publish-short at count 3, got %s", result.Action)
	}
	if len(result.Reason) > 100 {
		t.Errorf("expected brief reminder, got %d bytes", len(result.Reason))
	}
}

func TestComplianceReviewTrigger_SelfchecklistPublishSilent(t *testing.T) {
	hctx := &hook.HookContext{
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
	}
	hctx.SessionState.HTMLPublishCount = 6
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected selfchecklist publish-silent at count 6, got %s", result.Action)
	}
}

func TestComplianceReviewTrigger_SelfchecklistPublishSilent_At10(t *testing.T) {
	hctx := &hook.HookContext{
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
	}
	hctx.SessionState.HTMLPublishCount = 10
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected selfchecklist publish-silent at count 10, got %s", result.Action)
	}
}

func TestComplianceReviewTrigger_HtmlcheckerViolation_OnPublish(t *testing.T) {
	hctx := &hook.HookContext{
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
		ToolArgs: map[string]any{
			"path":    "deck.html",
			"content": "<style>\n.slide { color: red; }\n}\n</style>",
		},
	}
	hctx.SessionState.HTMLPublishCount = 1
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn for htmlchecker violation, got %s", result.Action)
	}
	if len(result.Reason) < 30 {
		t.Errorf("expected htmlchecker violation message, got %q", result.Reason)
	}
}

func TestComplianceReviewTrigger_HtmlcheckerViolation_NotThrottledAtHighPublishCount(t *testing.T) {
	// Even at count 10, htmlchecker violations should still fire Warn.
	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
		ToolArgs: map[string]any{
			"path":    "deck.html",
			"content": "<style>\n.slide { /* unclosed comment\n</style>",
		},
	}
	hctx.SessionState.HTMLPublishCount = 10
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn for htmlchecker violation even at count 10, got %s", result.Action)
	}
}


func TestComplianceReviewTrigger_HtmlcheckerViolation_BeforeSelfchecklistSilent(t *testing.T) {
	// htmlchecker violation should fire even when selfchecklist degradation would go silent.
	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
		ToolArgs: map[string]any{
			"path":    "deck.html",
			"content": "<style>\n}\n</style>",
		},
	}
	hctx.SessionState.HTMLPublishCount = 5 // Would be silent for selfchecklist
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn (htmlchecker violation takes priority), got %s", result.Action)
	}
}

func TestComplianceReviewTrigger_SelfchecklistFullChecklist_NoHtmlChartReference(t *testing.T) {
	// The selfchecklist full checklist must NOT reference the removed html-chart system.
	hctx := &hook.HookContext{
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
	}
	hctx.SessionState.HTMLPublishCount = 1
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected selfchecklist publish-full at count 1, got %s", result.Action)
	}
	// Must NOT contain old html-chart references.
	for _, forbidden := range []string{
		`data-gjs-type="html-chart"`,
		"data-gjs-type=\\\"html-chart\\\"",
		"html-chart",
		"data-chart-data",
		"data-chart-kind",
	} {
		if strings.Contains(result.Reason, forbidden) {
			t.Errorf("selfchecklist full checklist must NOT contain %q, got: %s", forbidden, result.Reason)
		}
	}
}

func TestComplianceReviewTrigger_SelfchecklistFullChecklist_HasNewItems(t *testing.T) {
	// The selfchecklist full checklist must contain all 15 Part 3 items.
	hctx := &hook.HookContext{
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
	}
	hctx.SessionState.HTMLPublishCount = 1
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected selfchecklist publish-full at count 1, got %s", result.Action)
	}
	// Must contain all 15 checklist items.
	items := strings.Count(result.Reason, "- [ ]")
	if items != 15 {
		t.Errorf("selfchecklist full checklist must have 15 items, got %d", items)
	}
	// Must contain the new checklist items.
	if !strings.Contains(result.Reason, "No bare") || !strings.Contains(result.Reason, "<table>") {
		t.Errorf("selfchecklist full checklist must contain 'no bare <table>' item, got: %s", result.Reason)
	}
	if !strings.Contains(result.Reason, "First") || !strings.Contains(result.Reason, "active") {
		t.Errorf("selfchecklist full checklist must contain 'first .slide active' item, got: %s", result.Reason)
	}
}

func TestComplianceReviewTrigger_NonHTMLPath(t *testing.T) {
	// Non-HTML paths should not trigger compliance review.
	hctx := &hook.HookContext{
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration),
		ToolArgs: map[string]any{
			"path":    "styles.css",
			"content": ".foo { color: red; }",
		},
	}
	hctx.SessionState.HTMLPublishCount = 1
	result := ComplianceReviewTrigger(context.Background(), hctx)
	// For non-HTML, it falls through to compliance check (count=1 → full checklist)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn for non-HTML write at count 1, got %s", result.Action)
	}
}

// ---------------------------------------------------------------------------
// QualityInject
// ---------------------------------------------------------------------------

func TestQualityInject_NilSessionState(t *testing.T) {
	hctx := &hook.HookContext{}
	result := QualityInject(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when nil SessionState, got %s", result.Action)
	}
}

func TestQualityInject_EmptyWarnings(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	hctx := &hook.HookContext{SessionState: s}
	result := QualityInject(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when no pending warnings, got %s", result.Action)
	}
}

func TestQualityInject_HasWarnings(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageInitialGeneration)
	s.AddPendingWarnings([]string{"warning one", "warning two"})

	hctx := &hook.HookContext{SessionState: s}
	result := QualityInject(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn when pending warnings exist, got %s", result.Action)
	}
	// Verify warnings were drained.
	if len(s.DrainPendingWarnings()) != 0 {
		t.Errorf("expected warnings to be drained after QualityInject")
	}
}

// ---------------------------------------------------------------------------
// DesignSkillRequired — iterative_edit guard
// ---------------------------------------------------------------------------

func TestDesignSkillRequired_IterativeEdit_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644)

	s := hook.NewSessionState("sess1", dir, hook.StageIterativeEdit)
	hctx := &hook.HookContext{
		ToolName:      "write_file",
		ToolArgs:      map[string]any{"path": "index.html"},
		SessionState:  s,
		WorkspacePath: dir,
		Stage:         hook.StageIterativeEdit,
	}
	result := DesignSkillRequired(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn for write_file targeting existing file in iterative_edit, got %s", result.Action)
	}
	if !strings.Contains(result.Reason, "edit_file") {
		t.Errorf("expected warning to mention edit_file, got: %s", result.Reason)
	}
}

func TestDesignSkillRequired_IterativeEdit_NewFile(t *testing.T) {
	dir := t.TempDir()

	s := hook.NewSessionState("sess1", dir, hook.StageIterativeEdit)
	hctx := &hook.HookContext{
		ToolName:      "write_file",
		ToolArgs:      map[string]any{"path": "new-deck.html"},
		SessionState:  s,
		WorkspacePath: dir,
		Stage:         hook.StageIterativeEdit,
	}
	result := DesignSkillRequired(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for write_file targeting non-existing file, got %s: %s", result.Action, result.Reason)
	}
}

// ---------------------------------------------------------------------------
// ComplianceReviewTrigger — Publish/Patch separation
// ---------------------------------------------------------------------------

func TestComplianceReviewTrigger_Patch_ShortReminder(t *testing.T) {
	// Patch count 1 → short reminder, never full checklist.
	hctx := &hook.HookContext{
		ToolName:     "edit_file",
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageIterativeEdit),
	}
	hctx.SessionState.HTMLPatchCount = 1
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn for first Patch, got %s", result.Action)
	}
	// Must NOT contain full checklist items.
	if strings.Contains(result.Reason, "[ ] DOCTYPE") {
		t.Errorf("Patch warning must NOT contain full checklist, got: %s", result.Reason)
	}
}

func TestComplianceReviewTrigger_Patch_SilentAfterThreshold(t *testing.T) {
	// Patch count 2 with PatchWarnMax=1 → silent.
	hctx := &hook.HookContext{
		ToolName:     "edit_file",
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: hook.NewSessionState("s1", "/tmp", hook.StageIterativeEdit),
	}
	hctx.SessionState.HTMLPatchCount = 5
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow (silent) for Patch count exceeding threshold (5 > 3), got %s", result.Action)
	}
}

func TestComplianceReviewTrigger_Patch_HtmlcheckerWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "deck.html")
	// Valid HTML structure that passes S1-S10, but has position:fixed (S6 violation).
	os.WriteFile(testFile, []byte("<!DOCTYPE html>\n<html><head><style>.foo { position: fixed; }</style></head><body><div class=\"slide active\"></div></body></html>"), 0o644)

	hctx := &hook.HookContext{
		ToolName:      "edit_file",
		ToolResult:    &hook.ToolResultInfo{IsError: false},
		SessionState:  hook.NewSessionState("s1", dir, hook.StageIterativeEdit),
		ToolArgs:      map[string]any{"path": "deck.html"},
		WorkspacePath: dir,
	}
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn for htmlchecker violation on Patch, got %s", result.Action)
	}
	if !strings.Contains(result.Reason, "position: fixed") {
		t.Errorf("expected position:fixed error message, got: %s", result.Reason)
	}
}

func TestComplianceReviewTrigger_Publish_CustomThresholds(t *testing.T) {
	cfg := &hook.Config{
		Settings: hook.Settings{
			Compliance: hook.ComplianceSettings{
				PublishFullChecklistMax: 0,
				PublishWarnMax:          1,
				PatchWarnMax:            1,
				EnableCSSOnPatch:        true,
			},
			Messages: hook.DefaultConfig().Settings.Messages,
		},
	}
	// Publish count 1 with PublishWarnMax=1, FullChecklistMax=0 →
	// pubCount (1) > FullChecklistMax (0) → short reminder (not full checklist).
	s := hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration)
	s.HTMLPublishCount = 1
	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: s,
		Config:       cfg,
	}
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn at count 1 with custom thresholds, got %s", result.Action)
	}
	// Must be short reminder, not full checklist.
	if strings.Contains(result.Reason, "[ ] DOCTYPE") {
		t.Errorf("expected short reminder with FullChecklistMax=0, got full checklist: %s", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestResolveWorkspacePath_Absolute(t *testing.T) {
	result, err := hook.ResolveWorkspacePath("/tmp/ws", "/tmp/ws/subdir/file.html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "/tmp/ws/subdir/file.html" {
		t.Errorf("expected absolute path unchanged when within workspace, got %q", result)
	}
}

func TestResolveWorkspacePath_Relative(t *testing.T) {
	result, err := hook.ResolveWorkspacePath("/tmp/ws", "subdir/file.html")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "/tmp/ws/subdir/file.html"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestResolveWorkspacePath_Empty(t *testing.T) {
	_, err := hook.ResolveWorkspacePath("/tmp/ws", "")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestResolveWorkspacePath_OutsideWorkspace(t *testing.T) {
	_, err := hook.ResolveWorkspacePath("/tmp/ws", "../outside")
	if err == nil {
		t.Error("expected error for path outside workspace")
	}
}

func TestHashArgs_Empty(t *testing.T) {
	result := hashArgs(map[string]any{})
	if result != "0" {
		t.Errorf("expected '0' for empty args, got %q", result)
	}
}

func TestHashArgs_NonEmpty(t *testing.T) {
	result := hashArgs(map[string]any{"path": "test.html", "content": "hello"})
	if result == "" || result == "0" {
		t.Errorf("expected non-zero hash, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// RegisterBuiltins — hook count and smoke test
// ---------------------------------------------------------------------------

func TestRegisterBuiltins(t *testing.T) {
	cfg := hook.DefaultConfig()
	engine := hook.NewEngineWithConfig(cfg)
	RegisterBuiltins(engine)

	engine.InitState("sess1", "/tmp/ws", hook.StageInitialGeneration)

	// Verify all hook points can run without panic.
	points := map[hook.HookPoint]string{
		hook.PointUserPromptSubmit:   "user_prompt_submit",
		hook.PointPreContextAssemble: "pre_context_assemble",
		hook.PointPreToolUse:         "pre_tool_use",
		hook.PointPostToolUse:        "post_tool_use",
	}

	for point := range points {
		hctx := &hook.HookContext{
			SessionID:     "sess1",
			WorkspacePath: t.TempDir(),
			Stage:         hook.StageInitialGeneration,
			ToolName:      "write_file",
			ToolArgs:      map[string]any{"path": "test.html"},
			SessionState:  engine.State(),
			Config:        cfg,
		}
		engine.Run(context.Background(), point, hctx)
	}
}

// TestComplianceReviewTrigger_Patch_PatchWarnMaxZero verifies that when
// PatchWarnMax=0, no reminder fires even on the first patch.
func TestComplianceReviewTrigger_Patch_PatchWarnMaxZero(t *testing.T) {
	tmpDir := t.TempDir()
	htmlPath := filepath.Join(tmpDir, "index.html")
	os.WriteFile(htmlPath, []byte("<!DOCTYPE html>\n<html><head></head><body><div class=\"slide active\"></div></body></html>"), 0644)

	cfg := hook.DefaultConfig()
	cfg.Settings.Compliance.PatchWarnMax = 0

	engine := hook.NewEngineWithConfig(cfg)
	engine.InitState("sess1", tmpDir, hook.StageIterativeEdit)
	RegisterBuiltins(engine)

	// First patch with PatchWarnMax=0 → silent.
	hctx := &hook.HookContext{
		SessionID:     "sess1",
		WorkspacePath: tmpDir,
		Stage:         hook.StageIterativeEdit,
		ToolName:      "edit_file",
		ToolArgs:      map[string]any{"path": htmlPath, "old_string": "x", "new_string": "y"},
		ToolResult:    &hook.ToolResultInfo{IsError: false},
		SessionState:  engine.State(),
		Config:        cfg,
	}
	engine.State().RecordToolCall("edit_file", map[string]any{"path": htmlPath, "old_string": "x", "new_string": "y"}, false, nil)
	reasons, err := engine.Run(context.Background(), hook.PointPostToolUse, hctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reasons) > 0 {
		t.Errorf("expected no reminder when PatchWarnMax=0, got %d results", len(reasons))
	}
}

// ---------------------------------------------------------------------------
// validateHTMLCompliance — htmlchecker S1-S10 rules
// ---------------------------------------------------------------------------

func TestValidateHTMLCompliance_S1_NoDoctype(t *testing.T) {
	issues := validateHTMLCompliance("<html><head></head><body></body></html>")
	found := false
	for _, iss := range issues {
		if iss.Rule == "doctype" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected S1 doctype issue for HTML without DOCTYPE")
	}
}

func TestValidateHTMLCompliance_S1_HasDoctype(t *testing.T) {
	issues := validateHTMLCompliance("<!DOCTYPE html>\n<html><head></head><body></body></html>")
	for _, iss := range issues {
		if iss.Rule == "doctype" {
			t.Errorf("unexpected S1 doctype issue: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S2_NoHtmlTag(t *testing.T) {
	issues := validateHTMLCompliance("<!DOCTYPE html>\n<body></body>")
	found := false
	for _, iss := range issues {
		if iss.Rule == "no-html-tag" {
			found = true
		}
	}
	if !found {
		t.Error("expected S2 no-html-tag issue")
	}
}

func TestValidateHTMLCompliance_S2_NoBodyTag(t *testing.T) {
	issues := validateHTMLCompliance("<!DOCTYPE html>\n<html></html>")
	found := false
	for _, iss := range issues {
		if iss.Rule == "no-body-tag" {
			found = true
		}
	}
	if !found {
		t.Error("expected S2 no-body-tag issue")
	}
}

func TestValidateHTMLCompliance_S3_BareTable(t *testing.T) {
	issues := validateHTMLCompliance("<!DOCTYPE html>\n<html><body><table></table></body></html>")
	found := false
	for _, iss := range issues {
		if iss.Rule == "bare-table" {
			found = true
		}
	}
	if !found {
		t.Error("expected S3 bare-table issue")
	}
}

func TestValidateHTMLCompliance_S3_NoTable(t *testing.T) {
	issues := validateHTMLCompliance("<!DOCTYPE html>\n<html><body><div class=\"slide active\"></div></body></html>")
	for _, iss := range issues {
		if iss.Rule == "bare-table" {
			t.Errorf("unexpected S3 bare-table issue: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S4_OnClickHandler(t *testing.T) {
	issues := validateHTMLCompliance("<!DOCTYPE html>\n<html><body><div class=\"slide active\" onclick=\"foo()\"></div></body></html>")
	found := false
	for _, iss := range issues {
		if iss.Rule == "inline-handler" {
			found = true
		}
	}
	if !found {
		t.Error("expected S4 inline-handler issue for onclick")
	}
}

func TestValidateHTMLCompliance_S4_NoHandlers(t *testing.T) {
	issues := validateHTMLCompliance("<!DOCTYPE html>\n<html><body><div class=\"slide active\"></div></body></html>")
	for _, iss := range issues {
		if iss.Rule == "inline-handler" {
			t.Errorf("unexpected S4 inline-handler issue: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S5_ScriptInCustomCode_OK(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><div data-gjs-type="custom-code"><script>console.log(1)</script></div><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "script-outside-custom-code" {
			t.Errorf("unexpected S5 script-outside-custom-code issue: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S5_ScriptOutsideCustomCode(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><script>console.log(1)</script><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "script-outside-custom-code" {
			found = true
		}
	}
	if !found {
		t.Error("expected S5 script-outside-custom-code issue")
	}
}

func TestValidateHTMLCompliance_S6_PositionFixed(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><style>.sticky { position: fixed; }</style></head><body><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "position-fixed" {
			found = true
		}
	}
	if !found {
		t.Error("expected S6 position-fixed issue")
	}
}

func TestValidateHTMLCompliance_S6_NoPositionFixed(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><style>.sticky { position: sticky; }</style></head><body><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "position-fixed" {
			t.Errorf("unexpected S6 position-fixed issue: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S7_ExternalHref(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><link rel="stylesheet" href="https://cdn.example.com/style.css"></head><body><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "external-url" {
			found = true
		}
	}
	if !found {
		t.Error("expected S7 external-url issue for https href")
	}
}

func TestValidateHTMLCompliance_S7_ExternalSrc(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><img src="https://example.com/img.png"><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "external-url" {
			found = true
		}
	}
	if !found {
		t.Error("expected S7 external-url issue for https src")
	}
}

func TestValidateHTMLCompliance_S7_DataURIOK(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><img src="data:image/png;base64,iVBORw0KGgo="><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "external-url" {
			t.Errorf("unexpected S7 external-url for data: URI: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S7_RelativePathOK(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><img src="./images/logo.png"><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "external-url" {
			t.Errorf("unexpected S7 external-url for relative path: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S7_CSSImportExternal(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><style>@import url("https://fonts.googleapis.com/css2");</style></head><body><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "external-url" && strings.Contains(iss.Message, "@import") {
			found = true
		}
	}
	if !found {
		t.Error("expected S7 external-url for CSS @import")
	}
}

func TestValidateHTMLCompliance_S7_CSSUrlExternal(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><style>body { background: url(https://example.com/bg.png); }</style></head><body><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "external-url" && strings.Contains(iss.Message, "url()") {
			found = true
		}
	}
	if !found {
		t.Error("expected S7 external-url for CSS url() with https")
	}
}

func TestValidateHTMLCompliance_S7_BlobURLOK(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><img src="blob:abc123"><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "external-url" {
			t.Errorf("unexpected S7 external-url for blob: URI: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S7_SrcsetExternal(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><img srcset="https://example.com/img.png 2x, /local.png 1x"><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "external-url" && strings.Contains(iss.Message, "srcset") {
			found = true
		}
	}
	if !found {
		t.Error("expected S7 external-url for srcset with https URL")
	}
}

func TestValidateHTMLCompliance_S7_SrcsetDataURICommaOK(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><img srcset="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg'></svg> 1x, /local.png 2x"><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "external-url" {
			t.Errorf("unexpected S7 external-url for srcset with data: URI containing comma: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S7_FontsGoogleAPIsAllowed(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Roboto&display=swap"></head><body><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "external-url" {
			t.Errorf("unexpected S7 external-url for fonts.googleapis.com: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S7_FontsGstaticAllowed(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><link rel="preload" href="https://fonts.gstatic.com/s/roboto/v30/KFOmCnqEu92Fr1Mu4mxP.ttf" as="font"></head><body><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "external-url" {
			t.Errorf("unexpected S7 external-url for fonts.gstatic.com: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S7_OtherExternalStillBlocked(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><img src="https://example.com/photo.jpg"><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "external-url" {
			found = true
		}
	}
	if !found {
		t.Error("expected S7 external-url for non-whitelisted domain")
	}
}

func TestValidateHTMLCompliance_S8_UnclosedCSSComment(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><style>/* this comment is not closed</style></head><body><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "css-comment" {
			found = true
		}
	}
	if !found {
		t.Error("expected S8 css-comment issue for unclosed comment")
	}
}

func TestValidateHTMLCompliance_S8_ClosedCSSComment(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><style>/* properly closed */ .foo { color: red; }</style></head><body><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "css-comment" {
			t.Errorf("unexpected S8 css-comment issue: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S9_UnclosedDiv(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><div class="slide active"><div>open</body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "unclosed-tag" && strings.Contains(iss.Message, "<div>") {
			found = true
		}
	}
	if !found {
		t.Error("expected S9 unclosed-tag issue for div")
	}
}

func TestValidateHTMLCompliance_S9_AllTagsClosed(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><div class="slide active"><span>text</span><p>para</p><a>link</a><ul><li>item</li></ul></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "unclosed-tag" {
			t.Errorf("unexpected S9 unclosed-tag issue: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S10_FirstSlideMissingActive(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><div class="slide"></div></body></html>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "slide-no-active" {
			found = true
		}
	}
	if !found {
		t.Error("expected S10 slide-no-active issue")
	}
}

func TestValidateHTMLCompliance_S10_FirstSlideHasActive(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "slide-no-active" {
			t.Errorf("unexpected S10 slide-no-active issue: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_S10_NoSlides(t *testing.T) {
	// No .slide elements at all — no issue expected.
	html := `<!DOCTYPE html>
<html><body><div>no slides here</div></body></html>`
	issues := validateHTMLCompliance(html)
	for _, iss := range issues {
		if iss.Rule == "slide-no-active" {
			t.Errorf("unexpected S10 slide-no-active when no slides exist: %s", iss.Message)
		}
	}
}

func TestValidateHTMLCompliance_AllRulesPass(t *testing.T) {
	html := `<!DOCTYPE html>
<html><head><style>body { color: #000; }</style></head><body><div data-gjs-type="custom-code"><script>console.log(1)</script></div><div class="slide active"></div></body></html>`
	issues := validateHTMLCompliance(html)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for fully compliant HTML, got %d: %v", len(issues), issues)
	}
}

func TestValidateHTMLCompliance_MultipleIssues(t *testing.T) {
	// Missing DOCTYPE, bare table, and no body.
	html := `<html><head></head><table></table></html>`
	issues := validateHTMLCompliance(html)
	// Should find at least: S1 (doctype), S2 (no-body), S3 (bare-table)
	rules := make(map[string]bool)
	for _, iss := range issues {
		rules[iss.Rule] = true
	}
	if !rules["doctype"] {
		t.Error("expected S1 doctype issue")
	}
	if !rules["no-body-tag"] {
		t.Error("expected S2 no-body-tag issue")
	}
	if !rules["bare-table"] {
		t.Error("expected S3 bare-table issue")
	}
}

// ---------------------------------------------------------------------------
// splitSrcset
// ---------------------------------------------------------------------------

func TestSplitSrcset_Simple(t *testing.T) {
	urls := splitSrcset("/img1.png 1x, /img2.png 2x")
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d: %v", len(urls), urls)
	}
	if urls[0] != "/img1.png" {
		t.Errorf("expected first URL '/img1.png', got %q", urls[0])
	}
	if urls[1] != "/img2.png" {
		t.Errorf("expected second URL '/img2.png', got %q", urls[1])
	}
}

func TestSplitSrcset_DataURIWithComma(t *testing.T) {
	// data:image/svg+xml,... contains commas that should NOT be treated as separators.
	urls := splitSrcset("data:image/svg+xml,<svg></svg> 1x, /fallback.png 2x")
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs (data URI should not be split by its comma), got %d: %v", len(urls), urls)
	}
	if urls[0] != "data:image/svg+xml,<svg></svg>" {
		t.Errorf("expected data URI preserved, got %q", urls[0])
	}
	if urls[1] != "/fallback.png" {
		t.Errorf("expected fallback URL, got %q", urls[1])
	}
}

func TestSplitSrcset_SingleURL(t *testing.T) {
	urls := splitSrcset("/only.png")
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
	if urls[0] != "/only.png" {
		t.Errorf("expected '/only.png', got %q", urls[0])
	}
}

func TestSplitSrcset_DescriptorSuffix(t *testing.T) {
	urls := splitSrcset("/img.png 100w, /img2.png 2x")
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d: %v", len(urls), urls)
	}
	if urls[0] != "/img.png" {
		t.Errorf("expected '/img.png' without descriptor, got %q", urls[0])
	}
	if urls[1] != "/img2.png" {
		t.Errorf("expected '/img2.png' without descriptor, got %q", urls[1])
	}
}

// ---------------------------------------------------------------------------
// extractStyleBlocks
// ---------------------------------------------------------------------------

func TestExtractStyleBlocks_SingleBlock(t *testing.T) {
	html := `<html><head><style>.red { color: red; }</style></head></html>`
	blocks := extractStyleBlocks(html)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 style block, got %d", len(blocks))
	}
	if !strings.Contains(blocks[0], ".red") {
		t.Errorf("expected style content, got: %s", blocks[0])
	}
}

func TestExtractStyleBlocks_MultipleBlocks(t *testing.T) {
	html := `<html><head><style>.a { }</style><style>.b { }</style></head></html>`
	blocks := extractStyleBlocks(html)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 style blocks, got %d", len(blocks))
	}
}

func TestExtractStyleBlocks_None(t *testing.T) {
	blocks := extractStyleBlocks("<html><body>no style</body></html>")
	if len(blocks) != 0 {
		t.Errorf("expected 0 style blocks, got %d", len(blocks))
	}
}

func TestExtractStyleBlocks_InsideCustomCode(t *testing.T) {
	html := `<div data-gjs-type="custom-code"><style>.custom { }</style></div>`
	blocks := extractStyleBlocks(html)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 style block inside custom-code, got %d", len(blocks))
	}
	if !strings.Contains(blocks[0], ".custom") {
		t.Errorf("expected custom style content, got: %s", blocks[0])
	}
}

// ---------------------------------------------------------------------------
// checkScriptInCustomCode
// ---------------------------------------------------------------------------

func TestCheckScriptInCustomCode_OK(t *testing.T) {
	html := `<div data-gjs-type="custom-code"><script>console.log(1)</script></div>`
	issues := checkScriptInCustomCode(html)
	if len(issues) > 0 {
		t.Errorf("expected no issues for script inside custom-code, got: %v", issues)
	}
}

func TestCheckScriptInCustomCode_Violation(t *testing.T) {
	html := `<body><script>console.log(1)</script></body>`
	issues := checkScriptInCustomCode(html)
	if len(issues) == 0 {
		t.Fatal("expected issues for script outside custom-code")
	}
	if issues[0].Rule != "script-outside-custom-code" {
		t.Errorf("expected script-outside-custom-code rule, got %q", issues[0].Rule)
	}
}

func TestCheckScriptInCustomCode_NoScript(t *testing.T) {
	issues := checkScriptInCustomCode("<body><div>no script</div></body>")
	if len(issues) > 0 {
		t.Errorf("expected no issues when no script tags, got: %v", issues)
	}
}

// ---------------------------------------------------------------------------
// checkExternalURLs
// ---------------------------------------------------------------------------

func TestCheckExternalURLs_NoExternal(t *testing.T) {
	html := `<img src="./local.png" href="#anchor"><script src="app.js"></script>`
	issues := checkExternalURLs(html)
	if len(issues) > 0 {
		t.Errorf("expected no external URL issues, got: %v", issues)
	}
}

func TestCheckExternalURLs_HTTPImgSrc(t *testing.T) {
	html := `<img src="http://example.com/img.png">`
	issues := checkExternalURLs(html)
	if len(issues) == 0 {
		t.Fatal("expected external URL issue for http src")
	}
}

func TestCheckExternalURLs_CSSImport(t *testing.T) {
	html := `<style>@import url("https://fonts.googleapis.com/css2");</style>`
	issues := checkExternalURLs(html)
	found := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "@import") {
			found = true
		}
	}
	if !found {
		t.Error("expected external URL issue for CSS @import")
	}
}

func TestCheckExternalURLs_CSSUrlFunction(t *testing.T) {
	html := `<style>body { background: url(https://cdn.example.com/bg.jpg); }</style>`
	issues := checkExternalURLs(html)
	found := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "url()") {
			found = true
		}
	}
	if !found {
		t.Error("expected external URL issue for CSS url() with https")
	}
}

// ---------------------------------------------------------------------------
// PostRoundLoopGuard
// ---------------------------------------------------------------------------

func TestPostRoundLoopGuard_NilSessionState(t *testing.T) {
	result := PostRoundLoopGuard(context.Background(), &hook.HookContext{})
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for nil SessionState, got %s", result.Action)
	}
}

func TestPostRoundLoopGuard_BelowThreshold(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageIterativeEdit)
	s.ConsecutiveRoundsNoWrite = 2
	// Simulate read-only tool calls in last round.
	s.RecordToolStartCalled("read_file")
	if !s.AllLastRoundReadOnly() {
		t.Fatal("expected AllLastRoundReadOnly to be true")
	}
	result := PostRoundLoopGuard(context.Background(), &hook.HookContext{SessionState: s})
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when ConsecutiveRoundsNoWrite < 3, got %s: %s", result.Action, result.Reason)
	}
}

func TestPostRoundLoopGuard_TextOnlyRounds(t *testing.T) {
	// Empty LastRoundTools → AllLastRoundReadOnly returns false → no warning.
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageIterativeEdit)
	s.ConsecutiveRoundsNoWrite = 5
	// No tools called → LastRoundTools empty.
	if s.AllLastRoundReadOnly() {
		t.Fatal("expected AllLastRoundReadOnly to be false when LastRoundTools is empty")
	}
	result := PostRoundLoopGuard(context.Background(), &hook.HookContext{SessionState: s})
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for text-only rounds, got %s: %s", result.Action, result.Reason)
	}
}

func TestPostRoundLoopGuard_TriggersAtThreshold(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageIterativeEdit)
	s.ConsecutiveRoundsNoWrite = 3
	s.RecordToolStartCalled("read_file")
	s.RecordToolStartCalled("grep_search")
	if !s.AllLastRoundReadOnly() {
		t.Fatal("expected AllLastRoundReadOnly to be true with read-only tools")
	}
	result := PostRoundLoopGuard(context.Background(), &hook.HookContext{SessionState: s})
	if result.Action != hook.Warn {
		t.Errorf("expected Warn when 3+ consecutive no-write rounds with all read-only tools, got %s", result.Action)
	}
	if !strings.Contains(result.Reason, "loop-guard") {
		t.Errorf("expected loop_guard warning message, got: %s", result.Reason)
	}
}

func TestPostRoundLoopGuard_NonReadOnlyTools(t *testing.T) {
	s := hook.NewSessionState("sess1", "/tmp/ws", hook.StageIterativeEdit)
	s.ConsecutiveRoundsNoWrite = 3
	s.RecordToolStartCalled("read_file")
	s.RecordToolStartCalled("write_file") // destructive tool in round
	if s.AllLastRoundReadOnly() {
		t.Fatal("expected AllLastRoundReadOnly to be false with write_file in round")
	}
	result := PostRoundLoopGuard(context.Background(), &hook.HookContext{SessionState: s})
	if result.Action != hook.Allow {
		t.Errorf("expected Allow when last round had non-read-only tools, got %s", result.Action)
	}
}

// ---------------------------------------------------------------------------
// ComplianceReviewTrigger — htmlchecker strict checks fire regardless of threshold
// ---------------------------------------------------------------------------

func TestComplianceReviewTrigger_StrictCheckFiresEvenWhenSilent(t *testing.T) {
	// Even when Publish count exceeds PublishWarnMax, strict htmlchecker issues
	// must still fire (they are not silence-able).
	html := `<!DOCTYPE html>
<html><body><div class="slide active"></div></body></html>`
	s := hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration)
	s.HTMLPublishCount = 10 // well above PublishWarnMax default (5)
	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: s,
		ToolArgs: map[string]any{
			"path":    "deck.html",
			"content": html,
		},
	}
	// This valid HTML has no strict issues, so it should be Allow (not Warn)
	// since Publish count exceeds threshold and there are no strict violations.
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Allow {
		t.Errorf("expected Allow for clean HTML even when publish count exceeds threshold, got %s: %s", result.Action, result.Reason)
	}
}

func TestComplianceReviewTrigger_StrictCheckFiresWithBadHTML_SilentThreshold(t *testing.T) {
	// Strict htmlchecker fires even past the publish silence threshold.
	html := `<html><body><table></table></body></html>`
	s := hook.NewSessionState("s1", "/tmp", hook.StageInitialGeneration)
	s.HTMLPublishCount = 10 // well above PublishWarnMax default
	hctx := &hook.HookContext{
		ToolName:     "write_file",
		ToolResult:   &hook.ToolResultInfo{IsError: false},
		SessionState: s,
		ToolArgs: map[string]any{
			"path":    "deck.html",
			"content": html,
		},
	}
	result := ComplianceReviewTrigger(context.Background(), hctx)
	if result.Action != hook.Warn {
		t.Errorf("expected Warn for bad HTML even past silence threshold, got %s", result.Action)
	}
	if !strings.Contains(result.Reason, "htmlchecker") {
		t.Errorf("expected htmlchecker message, got: %s", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// ComplianceReviewTrigger — severity field
// ---------------------------------------------------------------------------

func TestComplianceIssue_SeverityMustFix(t *testing.T) {
	if SeverityMustFix != "must_fix" {
		t.Errorf("expected SeverityMustFix to be 'must_fix', got %q", SeverityMustFix)
	}
}

func TestComplianceIssue_SeverityNeedReview(t *testing.T) {
	if SeverityNeedReview != "need_review" {
		t.Errorf("expected SeverityNeedReview to be 'need_review', got %q", SeverityNeedReview)
	}
}
