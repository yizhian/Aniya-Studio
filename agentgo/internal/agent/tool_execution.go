package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"agentgo/internal/hook"
	"agentgo/internal/model"
	"agentgo/internal/observability"
)

type toolResult struct {
	toolCallID string
	name       string
	content    string
	message    model.Message
	isError    bool
	durationMs int64
	metadata   map[string]any
}

// executeTools runs tool calls with pre/post hook checks. Read-only tools run concurrently.
func executeTools(ctx context.Context, cfg StreamingLoopConfig, round int, calls []model.ToolCall, toolRoundBuf *observability.RoundEventBuffer) []toolResult {
	results := make([]toolResult, 0, len(calls))

	var validCalls []model.ToolCall
	for _, tc := range calls {
		if json.Valid([]byte(tc.Function.Arguments)) {
			validCalls = append(validCalls, tc)
		} else {
			errMsg := "Tool call arguments were truncated (invalid JSON). The file was NOT written. Re-issue with smaller content or use edit_file for incremental changes."
			msg := model.Message{Role: "tool", ToolCallID: tc.ID, Content: errMsg}
			results = append(results, toolResult{
				toolCallID: tc.ID,
				name:       tc.Function.Name,
				content:    errMsg,
				message:    msg,
				isError:    true,
			})
		}
	}
	calls = validCalls

	var readOnly, destructive []model.ToolCall
	for _, tc := range calls {
		if isReadOnlyTool(cfg, tc.Function.Name) {
			readOnly = append(readOnly, tc)
		} else {
			destructive = append(destructive, tc)
		}
	}

	if len(readOnly) > 0 {
		const maxParallelReadOnly = 6
		sem := make(chan struct{}, maxParallelReadOnly)
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, tc := range readOnly {
			tc := tc
			args := parseToolArgsJSON(tc.Function.Arguments)
			if blocked := checkPreToolUse(ctx, preToolUseParams{cfg, round, tc, args, false}); blocked != nil {
				results = append(results, *blocked)
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				r := runOneTool(ctx, cfg, round, tc, toolRoundBuf)
				runPostToolUse(ctx, postToolUseParams{cfg, round, tc.Function.Name, args, r.isError, r.metadata, r.content})
				mu.Lock()
				results = append(results, r)
				mu.Unlock()
			}()
		}
		wg.Wait()
	}

	if len(destructive) > 0 && cfg.SessionState != nil {
		cfg.SessionState.DestructiveInProgress = true
	}
	for _, tc := range destructive {
		args := parseToolArgsJSON(tc.Function.Arguments)
		if blocked := checkPreToolUse(ctx, preToolUseParams{cfg, round, tc, args, true}); blocked != nil {
			results = append(results, *blocked)
			continue
		}
		r := runOneTool(ctx, cfg, round, tc, toolRoundBuf)
		runPostToolUse(ctx, postToolUseParams{cfg, round, tc.Function.Name, args, r.isError, r.metadata, r.content})
		results = append(results, r)
	}
	if len(destructive) > 0 && cfg.SessionState != nil {
		cfg.SessionState.DestructiveInProgress = false
	}

	return results
}

type preToolUseParams struct {
	cfg           StreamingLoopConfig
	round         int
	tc            model.ToolCall
	args          map[string]any
	isDestructive bool
}

func checkPreToolUse(ctx context.Context, p preToolUseParams) *toolResult {
	if p.cfg.HookEngine == nil || p.cfg.SessionState == nil {
		return nil
	}

	p.cfg.SessionState.RecordToolStartCalled(p.tc.Function.Name)

	hctx := &hook.HookContext{
		SessionID:         p.cfg.SessionState.SessionID,
		WorkspacePath:     p.cfg.SessionState.WorkspacePath,
		Stage:             p.cfg.SessionState.Stage,
		Round:             p.round,
		ToolName:          p.tc.Function.Name,
		ToolArgs:          p.args,
		ToolIsDestructive: p.isDestructive,
		SessionState:      p.cfg.SessionState,
		Config:            nil,
	}

	warnings, err := p.cfg.HookEngine.Run(ctx, hook.PointPreToolUse, hctx)
	if err != nil {
		if blocked, ok := err.(*hook.BlockedError); ok {
			msg := model.Message{
				Role:       "tool",
				ToolCallID: p.tc.ID,
				Content:    blocked.Error(),
			}
			return &toolResult{
				toolCallID: p.tc.ID,
				name:       p.tc.Function.Name,
				content:    blocked.Error(),
				message:    msg,
				isError:    true,
			}
		}
		log.Printf("hook error (pre_tool_use %s): %v", p.tc.Function.Name, err)
	}

	if len(warnings) > 0 && p.cfg.SessionState != nil {
		p.cfg.SessionState.AddPendingWarnings(warnings)
		if p.cfg.Emitter != nil {
			for _, w := range warnings {
				p.cfg.Emitter.Emit(observability.AgentEvent{
					Type:  "hook:warn",
					Round: p.round,
					Data:  map[string]any{"message": w},
				})
			}
		}
	}

	return nil
}

type postToolUseParams struct {
	cfg      StreamingLoopConfig
	round    int
	toolName string
	args     map[string]any
	isError  bool
	metadata map[string]any
	content  string
}

func runPostToolUse(ctx context.Context, p postToolUseParams) {
	if p.cfg.HookEngine == nil {
		return
	}
	warnings, err := p.cfg.HookEngine.RecordToolResult(ctx, p.round, p.toolName, p.args, p.isError, p.metadata, p.content)
	if err != nil {
		if _, ok := err.(*hook.BlockedError); !ok {
			log.Printf("hook error (post_tool_use %s): %v", p.toolName, err)
		}
	}
	if len(warnings) > 0 && p.cfg.SessionState != nil {
		p.cfg.SessionState.AddPendingWarnings(warnings)
		if p.cfg.Emitter != nil {
			for _, w := range warnings {
				p.cfg.Emitter.Emit(observability.AgentEvent{
					Type:  "hook:warn",
					Round: p.round,
					Data:  map[string]any{"message": w},
				})
			}
		}
	}
}

func parseToolArgsJSON(argsJSON string) map[string]any {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil
	}
	return args
}

// runOneTool executes a single tool call and emits observability events.
func runOneTool(ctx context.Context, cfg StreamingLoopConfig, round int, tc model.ToolCall, toolRoundBuf *observability.RoundEventBuffer) toolResult {
	if tc.Type != "" && tc.Type != "function" {
		return toolResult{toolCallID: tc.ID, content: "skipped: non-function tool type"}
	}

	start := time.Now()
	out, metadata, err := cfg.Execute.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
	durationMs := time.Since(start).Milliseconds()
	isError := err != nil
	if err != nil {
		out = fmt.Sprintf("error executing tool %q: %v", tc.Function.Name, err)
	}

	msg := model.Message{
		Role:       "tool",
		ToolCallID: tc.ID,
		Content:    out,
	}

	if cfg.Emitter != nil {
		data := map[string]any{
			"name":        tc.Function.Name,
			"content":     out,
			"id":          tc.ID,
			"duration_ms": durationMs,
		}
		if args := parseToolArgsJSON(tc.Function.Arguments); args != nil {
			if p, ok := args["path"].(string); ok {
				data["path"] = p
			}
		}
		if err != nil {
			data["is_error"] = true
		}
		bufferToolObs(cfg.Emitter, toolRoundBuf, observability.AgentEvent{
			Type:  "tool_result",
			Round: round,
			Data:  data,
		})

		if isError {
			bufferToolObs(cfg.Emitter, toolRoundBuf, observability.AgentEvent{
				Type:  "error",
				Round: round,
				Data: map[string]any{
					"message":   fmt.Sprintf("tool %q failed: %s", tc.Function.Name, out),
					"tool_name": tc.Function.Name,
				},
			})
		}
	}

	return toolResult{
		toolCallID: tc.ID,
		name:       tc.Function.Name,
		content:    out,
		message:    msg,
		isError:    isError,
		durationMs: durationMs,
		metadata:   metadata,
	}
}

func isReadOnlyTool(cfg StreamingLoopConfig, name string) bool {
	if fp, ok := cfg.Execute.(ToolFlagProvider); ok {
		flags, err := fp.GetToolFlags(name)
		if err == nil {
			return flags.ReadOnly && flags.ConcurrencySafe
		}
	}
	switch name {
	case "read_file", "list_files", "grep_search", "web_fetch", "tool_search", "skill":
		return true
	}
	return false
}

func hasBrokenToolCalls(toolCalls []model.ToolCall) bool {
	for _, tc := range toolCalls {
		if !json.Valid([]byte(tc.Function.Arguments)) {
			return true
		}
		if !hasMinimumFields(tc.Function.Name, tc.Function.Arguments) {
			return true
		}
	}
	return false
}
