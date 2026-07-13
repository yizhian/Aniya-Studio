package agent

import (
	"path/filepath"

	agentctx "agentgo/internal/context"
	"agentgo/internal/model"
	"agentgo/internal/observability"
	"agentgo/internal/persistence"
)

// processContextOperations handles post-tool-execution operations: HTML modification
// detection, design skill asset inlining, todo persistence, and memory index rebuild.
// Returns true if HTML was modified during this round.
func processContextOperations(
	cfg StreamingLoopConfig,
	st *LoopState,
	toolCalls []model.ToolCall,
	results []toolResult,
	hadModification bool,
) bool {
	if cfg.ContextManager == nil {
		return hadModification
	}

	// Build tool execution summaries for HTML detection.
	execSummaries := make([]agentctx.ToolExecSummary, len(results))
	for i, r := range results {
		execSummaries[i] = agentctx.ToolExecSummary{
			ToolCallID: r.toolCallID,
			Success:    !r.isError,
		}
	}

	// Track HTML modifications (no version creation yet).
	if cfg.ContextManager.DetectHTMLModification(toolCalls, execSummaries) {
		hadModification = true
		// Inline design skill assets into generated HTML.
		if cfg.DesignSkill != "" {
			htmlPath := cfg.ContextManager.HTMLFilePath()
			if htmlPath != "" {
				skillsDir := filepath.Join(cfg.WorkspacePath, "skills", cfg.DesignSkill)
				if err := inlineDesignSkillAssets(htmlPath, skillsDir); err != nil && cfg.Emitter != nil {
					cfg.Emitter.Emit(observability.AgentEvent{
						Type:  "hook:warn",
						Round: st.Round,
						Data:  map[string]any{"message": "skill asset inline: " + err.Error()},
					})
				}
			}
		}
	}

	// Persist todo list if todo_write was called this round.
	for _, tc := range toolCalls {
		if tc.Function.Name == "todo_write" {
			if todos, err := agentctx.ParseTodoArgs(tc.Function.Arguments); err == nil {
				if err := cfg.ContextManager.UpdateTodos(todos); err != nil && cfg.Emitter != nil {
					cfg.Emitter.Emit(observability.AgentEvent{
						Type:  "error",
						Round: st.Round,
						Data:  map[string]any{"message": "todo persist: " + err.Error()},
					})
				}
				items := make([]map[string]any, len(todos))
				for i, t := range todos {
					items[i] = map[string]any{
						"content":     t.Content,
						"status":      t.Status,
						"active_form": t.ActiveForm,
					}
				}
				appendTimeline(st, "todo_write", map[string]any{"todos": items})

				if cfg.Emitter != nil {
					cfg.Emitter.Emit(observability.AgentEvent{
						Type:  "todo_write",
						Round: st.Round,
						Data:  map[string]any{"todos": items},
					})
				}
			}
		}
	}

	// Rebuild memory index if write_file/edit_file targeted .agentgo/memory/.
	if cfg.MemoryStore != nil && cfg.WorkspacePath != "" {
		memoryBase := cfg.MemoryStore.GetBasePath(cfg.WorkspacePath)
		for _, tc := range toolCalls {
			if tc.Function.Name == "write_file" || tc.Function.Name == "edit_file" {
				if isMemoryPath(cfg.WorkspacePath, memoryBase, extractPathFromJSON(tc.Function.Arguments)) {
					idx := persistence.NewFileMemoryIndex(memoryBase)
					if err := idx.RebuildIndex(); err != nil && cfg.Emitter != nil {
						cfg.Emitter.Emit(observability.AgentEvent{
							Type:  "error",
							Round: st.Round,
							Data:  map[string]any{"message": "memory index rebuild: " + err.Error()},
						})
					}
					break
				}
			}
		}
	}

	return hadModification
}
