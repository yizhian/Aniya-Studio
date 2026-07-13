// Package hook provides a Hook framework that enforces engineering discipline
// at the code level rather than relying solely on system prompt instructions.
// Hooks intercept the agent loop at five mount points and can block (Block),
// warn (Warn), or allow (Allow) operations based on configurable rules.
package hook

import (
	"context"
	"path/filepath"
	"strings"

	"agentgo/internal/model"
)

// HookPoint identifies the mount point in the agent loop where a hook is invoked.
type HookPoint string

const (
	PointUserPromptSubmit   HookPoint = "user_prompt_submit"
	PointPreContextAssemble HookPoint = "pre_context_assemble"
	PointPreToolUse         HookPoint = "pre_tool_use"
	PointPostToolUse        HookPoint = "post_tool_use"
	PointPostRound          HookPoint = "post_round"
)

// Stage classifies the current phase of a conversation.
// Type alias to model.Stage so packages that need Stage but not the full hook
// framework can depend on the lighter model package.
type Stage = model.Stage

const (
	StageInitialGeneration = model.StageInitialGeneration
	StageIterativeEdit     = model.StageIterativeEdit
)

// HookAction determines what the engine does after a hook fires.
type HookAction string

const (
	Allow HookAction = "allow"
	Warn  HookAction = "warn"
	Block HookAction = "block"
)

// HookResult is returned by a hook function and tells the engine what to do.
type HookResult struct {
	Action HookAction
	Reason string // human-readable explanation (required for Block and Warn)
}

// HookContext carries all the data a hook function can inspect.
type HookContext struct {
	// Session identity
	SessionID     string
	WorkspacePath string

	// Stage tracking
	Stage Stage
	Round int

	// Design skill (set by handleChat from project.json manifest)
	SelectedDesignSkill string

	// Tool call context (set only for pre_tool_use and post_tool_use)
	ToolName  string
	ToolArgs  map[string]any
	ToolIsDestructive bool

	// Tool execution result (set only for post_tool_use)
	ToolResult *ToolResultInfo

	// Context stats (set for pre_context_assemble)
	MessageCount int

	// Session-level state (read-only — mutations go through SessionState.RecordToolCall)
	SessionState *SessionState

	// Config snapshot
	Config *Config
}

// ToolResultInfo carries the outcome of a tool execution for post_tool_use hooks.
type ToolResultInfo struct {
	IsError    bool
	Content    string
	DurationMs int64
	Metadata   map[string]any
}

// Matcher filters which tool calls a hook applies to.
// All fields are ANDed — a nil/empty field means "match anything".
type Matcher struct {
	ToolNames    []string // exact tool name match
	PathPatterns []string // glob patterns for file path (e.g. "*.html")
}

// Match reports whether hctx satisfies this matcher.
func (m *Matcher) Match(hctx *HookContext) bool {
	if m == nil {
		return true
	}
	if len(m.ToolNames) > 0 && !contains(m.ToolNames, hctx.ToolName) {
		return false
	}
	if len(m.PathPatterns) > 0 {
		path, _ := hctx.ToolArgs["path"].(string)
		if path == "" {
			return false
		}
		if !matchesAnyGlob(path, m.PathPatterns) {
			return false
		}
	}
	return true
}

// HookFn is the function signature for a hook implementation.
type HookFn func(ctx context.Context, hctx *HookContext) HookResult

// RegisteredHook is a hook registered with the engine.
type RegisteredHook struct {
	Name     string
	On       HookPoint
	Stage    string // "always", "initial_generation", "iterative_edit", or "" (=always)
	Priority int    // lower values run first
	Matcher  *Matcher
	Fn       HookFn
	Builtin  bool // true if this hook cannot be disabled via config
}

func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func matchesAnyGlob(path string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
		// Also try matching against the full path (relative).
		matched, err = filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// IsHTMLPath checks if a file path has an .html or .htm extension (case-insensitive).
func IsHTMLPath(p string) bool {
	lower := strings.ToLower(p)
	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm")
}
