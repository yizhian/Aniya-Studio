package agent

import (
	"context"

	"agentgo/internal/model"
	"agentgo/internal/observability"
	"agentgo/internal/persistence"
)

// buildInitialMessages constructs the initial message list for the agent loop:
// system prompt + history (from previous rounds) + new user message.
// It also initializes the loop state's Messages, Timeline, and runs semantic recall.
func buildInitialMessages(ctx context.Context, cfg StreamingLoopConfig, st *LoopState) error {
	msgs := make([]model.Message, 0, 8)
	if cfg.SystemPrompt != "" {
		msgs = append(msgs, model.Message{Role: "system", Content: cfg.SystemPrompt})
	}
	msgs = append(msgs, cfg.History...)
	msgs = append(msgs, model.Message{Role: "user", Content: cfg.UserMessage})
	st.Messages = append([]model.Message(nil), msgs...)

	// Preserve timeline from previous conversations so the full chat history
	// survives across multiple conversations in the same session.
	if len(cfg.PreviousTimeline) > 0 {
		st.Timeline = append(st.Timeline, cfg.PreviousTimeline...)
	}
	userMsgData := map[string]any{"content": cfg.UserMessage}
	if len(cfg.DomContext) > 0 {
		userMsgData["dom_context"] = cfg.DomContext
	}
	if len(cfg.Attachments) > 0 {
		userMsgData["attachments"] = cfg.Attachments
	}
	appendTimeline(st, "user_message", userMsgData)

	// Synchronous semantic memory recall before the first round.
	if cfg.MemoryStore != nil && cfg.ContextManager != nil && cfg.Provider != nil && cfg.WorkspacePath != "" {
		idx, err := persistence.NewFileMemoryIndex(
			cfg.MemoryStore.GetBasePath(cfg.WorkspacePath),
		).LoadIndex()
		if err == nil && len(idx) > 0 {
			recalled, err := SemanticRecall(
				ctx, cfg.UserMessage, cfg.WorkspacePath, cfg.MemoryStore, idx, cfg.Provider,
			)
			if err != nil {
				if cfg.Emitter != nil {
					cfg.Emitter.Emit(observability.AgentEvent{
						Type: "error",
						Data: map[string]any{"message": "semantic recall: " + err.Error()},
					})
				}
			} else if len(recalled) > 0 {
				cfg.ContextManager.SetRecalledMemories(recalled)
			}
		}
	}

	return nil
}
