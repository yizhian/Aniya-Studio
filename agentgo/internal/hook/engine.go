package hook

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"

	"agentgo/internal/model"
	"agentgo/internal/observability"
)

// Engine manages registered hooks, runs them at mount points, and owns SessionState.
type Engine struct {
	hooks  map[HookPoint][]*RegisteredHook
	state  *SessionState
	config *Config

	// overrides maps hook name → enabled flag from project config.
	overrides map[string]bool
	emitter   *observability.Emitter
}

// NewEngine creates a new hook engine with default config and empty hook registry.
func NewEngine(configPath string) *Engine {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		log.Printf("hook: load config %q: %v (using defaults)", configPath, err)
		cfg = DefaultConfig()
	}

	overrides := make(map[string]bool)
	for _, h := range cfg.Hooks {
		if h.Enabled != nil {
			overrides[h.Name] = *h.Enabled
		}
	}

	return &Engine{
		hooks:     make(map[HookPoint][]*RegisteredHook),
		config:    cfg,
		overrides: overrides,
	}
}

// NewEngineWithConfig creates an engine with a pre-loaded config (for testing).
func NewEngineWithConfig(cfg *Config) *Engine {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	overrides := make(map[string]bool)
	for _, h := range cfg.Hooks {
		if h.Enabled != nil {
			overrides[h.Name] = *h.Enabled
		}
	}
	return &Engine{
		hooks:     make(map[HookPoint][]*RegisteredHook),
		config:    cfg,
		overrides: overrides,
	}
}

// Register adds a hook to the engine. Hooks are sorted by Priority (ascending)
// within each mount point.
func (e *Engine) Register(h *RegisteredHook) {
	e.hooks[h.On] = append(e.hooks[h.On], h)
	// Keep hooks sorted by priority within each mount point.
	sort.SliceStable(e.hooks[h.On], func(i, j int) bool {
		return e.hooks[h.On][i].Priority < e.hooks[h.On][j].Priority
	})
}

// InitState creates a fresh SessionState bound to this engine.
func (e *Engine) InitState(sessionID, workspacePath string, stage Stage) {
	e.state = NewSessionState(sessionID, workspacePath, stage)
	e.state.MaxConsecutiveFailures = e.config.Settings.MaxConsecutiveFailures
}

// State returns the current SessionState (may be nil before InitState).
func (e *Engine) State() *SessionState {
	return e.state
}

// SetState injects a previously-saved SessionState (e.g. across rounds).
func (e *Engine) SetState(s *SessionState) {
	e.state = s
}

// SetEmitter sets an optional observability emitter for hook operation events.
func (e *Engine) SetEmitter(emitter *observability.Emitter) {
	e.emitter = emitter
}

// Run executes all registered hooks for the given mount point.
// It returns a slice of warning messages (Warn action) and an error for the
// first Block action encountered. Hooks that return Allow are not collected.
func (e *Engine) Run(ctx context.Context, point HookPoint, hctx *HookContext) ([]string, error) {
	hooks := e.match(point, hctx)

	var warnings []string
	for _, h := range hooks {
		// Check project overrides: if explicitly disabled, skip.
		if !h.Builtin {
			if enabled, ok := e.overrides[h.Name]; ok && !enabled {
				continue
			}
		}

		// Populate Config reference in hctx if not set.
		if hctx.Config == nil {
			hctx.Config = e.config
		}

		result := h.Fn(ctx, hctx)
		switch result.Action {
		case Block:
			observability.EmitOrLog(e.emitter, observability.AgentEvent{
				Type: observability.EventHookBlocked,
				Data: map[string]any{
					"hook_name":   h.Name,
					"mount_point": string(point),
					"reason":      result.Reason,
				},
			})
			return warnings, &BlockedError{
				HookName: h.Name,
				Reason:   result.Reason,
			}
		case Warn:
			warnings = append(warnings, formatWarning(h.Name, result.Reason))
		case Allow:
			// continue
		}
	}
	return warnings, nil
}

// match filters registered hooks for the given point and context.
func (e *Engine) match(point HookPoint, hctx *HookContext) []*RegisteredHook {
	candidates := e.hooks[point]
	var matched []*RegisteredHook
	for _, h := range candidates {
		// Stage filtering: empty or "always" matches everything.
		if h.Stage != "" && h.Stage != "always" && string(hctx.Stage) != "" && h.Stage != string(hctx.Stage) {
			continue
		}
		// Matcher filtering.
		if h.Matcher != nil && !h.Matcher.Match(hctx) {
			continue
		}
		matched = append(matched, h)
	}
	return matched
}

// RecordToolResult is the post_tool_use state updater. It calls
// SessionState.RecordToolCall and then runs PointPostToolUse hooks.
func (e *Engine) RecordToolResult(ctx context.Context, round int, toolName string, args map[string]any, isError bool, metadata map[string]any, content string) ([]string, error) {
	if e.state == nil {
		return nil, nil
	}
	e.state.RecordToolCall(toolName, args, isError, metadata)

	hctx := &HookContext{
		SessionID:     e.state.SessionID,
		WorkspacePath: e.state.WorkspacePath,
		Stage:         e.state.Stage,
		Round:         round,
		ToolName:      toolName,
		ToolArgs:      args,
		SessionState:  e.state,
		Config:        e.config,
		ToolResult: &ToolResultInfo{
			IsError:  isError,
			Content:  content,
			Metadata: metadata,
		},
	}

	return e.Run(ctx, PointPostToolUse, hctx)
}

// RecordRoundEnd runs end-of-round logic: TrackRoundEnd (per-round metrics) then
// PointPostRound hooks (e.g., loop_guard). TrackRoundEnd executes first so the
// counter reflects the just-completed round before hooks read it.
func (e *Engine) RecordRoundEnd(ctx context.Context, round int) []string {
	if e.state == nil {
		return nil
	}
	// 1. Update per-round metrics FIRST so PointPostRound sees correct counters.
	e.state.TrackRoundEnd()
	// 2. Run PointPostRound hooks (loop_guard, etc.) after counters are current.
	hctx := e.state.ToHookContext()
	hctx.Round = round
	warnings, _ := e.Run(ctx, PointPostRound, hctx)
	return warnings
}

// IsBlockedError checks if err is a BlockedError.
func IsBlockedError(err error) bool {
	var blocked *BlockedError
	return errors.As(err, &blocked)
}

// injectWarning appends a warning to the first system message when one exists,
// or prepends it as a new system message otherwise. This avoids creating
// multiple system messages (Anthropic only keeps the last one).
func injectWarning(messages []model.Message, warning string) []model.Message {
	if len(messages) == 0 {
		return messages
	}
	if messages[0].Role == "system" {
		out := make([]model.Message, len(messages))
		copy(out, messages)
		out[0].Content = messages[0].Content + "\n\n" + warning
		return out
	}
	warningMsg := model.Message{Role: "system", Content: warning}
	return append([]model.Message{warningMsg}, messages...)
}

// InjectWarnings inserts a collection of warning strings into the message list.
func InjectWarnings(messages []model.Message, warnings []string) []model.Message {
	for _, w := range warnings {
		if strings.TrimSpace(w) != "" {
			messages = injectWarning(messages, w)
		}
	}
	return messages
}