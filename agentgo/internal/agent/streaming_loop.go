package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	agentctx "agentgo/internal/context"
	"agentgo/internal/hook"
	"agentgo/internal/model"
	"agentgo/internal/observability"
	"agentgo/internal/persistence"
	"agentgo/internal/provider"
	"agentgo/internal/toolkit/contracts"
)

// StreamingLoopConfig configures the streaming agent loop.
type StreamingLoopConfig struct {
	SystemPrompt string
	UserMessage  string
	History      []model.Message
	Tools        []model.ToolDefinition
	MaxRounds    int
	MaxTokens    int

	DesignSkill      string
	PreviousTimeline []TimelineEvent
	DomContext        map[string]any
	Attachments       []map[string]any

	Provider       provider.StreamingProvider
	Execute        ToolExecutor
	Emitter        *observability.Emitter
	SessionStore   *persistence.SessionStore
	MemoryStore    persistence.MemoryStore
	SessionID      string
	WorkspacePath  string
	ContextManager *agentctx.ContextManager

	HookEngine   *hook.Engine
	SessionState *hook.SessionState
}

// ToolFlagProvider allows the loop to check tool flags for parallel execution.
type ToolFlagProvider interface {
	GetToolFlags(name string) (contracts.ToolBehaviorFlags, error)
}

// callWithRecovery wraps StreamChat with circuit breaker gating and error classification.
func callWithRecovery(
	ctx context.Context,
	cfg StreamingLoopConfig,
	messages []model.Message,
	rs *RecoveryState,
	cb *CircuitBreaker,
) (<-chan provider.StreamEvent, *RecoveryActionResult, error) {
	if !cb.Allow() {
		rs.Terminal = true
		return nil, nil, fmt.Errorf("circuit breaker open")
	}

	eventCh, err := cfg.Provider.StreamChat(ctx, provider.ChatRequest{
		Messages:  messages,
		Tools:     cfg.Tools,
		Stream:    true,
		MaxTokens: cfg.MaxTokens,
	})
	if err != nil {
		cb.RecordFailure()
		action := ClassifyRecovery(err, "", "", nil, false, rs, cb)
		if action == RecoveryNone {
			rs.Terminal = true
			if cfg.Emitter != nil {
				cfg.Emitter.Emit(observability.AgentEvent{
					Type: "error",
					Data: map[string]any{"message": err.Error()},
				})
			}
			return nil, nil, fmt.Errorf("stream chat: %w", err)
		}
		return nil, &RecoveryActionResult{Action: action, Error: err}, nil
	}

	return eventCh, nil, nil
}

// RunStreaming runs the agent loop with streaming output and parallel read-only tools.
func RunStreaming(ctx context.Context, cfg StreamingLoopConfig) (LoopState, error) {
	var st LoopState

	if cfg.Provider == nil {
		return st, fmt.Errorf("agent: nil Provider")
	}
	if cfg.Execute == nil {
		return st, fmt.Errorf("agent: nil ToolExecutor")
	}

	max := cfg.MaxRounds
	if max <= 0 {
		max = 100
	}

	// Phase 1: Build initial messages and run semantic recall.
	_ = buildInitialMessages(ctx, cfg, &st)

	hadModification := false
	cb := NewCircuitBreaker()
	for st.Round = 1; st.Round <= max; st.Round++ {
		roundStart := time.Now()
		toolRoundBuf := observability.NewRoundEventBuffer()
		if cfg.SessionState != nil {
			cfg.SessionState.RecordRoundStart()
		}

		if cfg.Emitter != nil {
			cfg.Emitter.Emit(observability.AgentEvent{
				Time:  time.Now(),
				Type:  "round",
				Round: st.Round,
			})
		}

		// Phase 2: Assemble messages for this round (with hooks + context trimming).
		messagesForContext := st.Messages
		if cfg.HookEngine != nil && cfg.SessionState != nil {
			hctx := cfg.SessionState.ToHookContext()
			hctx.Round = st.Round
			hctx.MessageCount = len(st.Messages)
			warnings, hookErr := cfg.HookEngine.Run(ctx, hook.PointPreContextAssemble, hctx)
			if hookErr != nil {
				if blocked, ok := hookErr.(*hook.BlockedError); ok {
					if cfg.Emitter != nil {
						cfg.Emitter.Emit(observability.AgentEvent{
							Type:  "hook:blocked",
							Round: st.Round,
							Data:  map[string]any{"message": blocked.Error()},
						})
					}
					return st, fmt.Errorf("pre_context_assemble blocked: %w", blocked)
				}
			}
			if len(warnings) > 0 {
				messagesForContext = hook.InjectWarnings(st.Messages, warnings)
			}
		}

		chatStart := time.Now()
		timelineSnapshotLen := len(st.Timeline)

		// Phase 3: Model call with recovery.
		eventCh, action, err := callWithRecovery(ctx, cfg, messagesForContext, &st.Recovery, cb)
		if err != nil {
			return st, fmt.Errorf("stream chat round %d: %w", st.Round, err)
		}
		if action != nil {
			skip, err := applyRecoveryAction(ctx, cfg, &st, action.Action, action.Error, timelineSnapshotLen, toolRoundBuf)
			if err != nil {
				return st, err
			}
			if skip {
				continue
			}
		}

		// Phase 4: Process stream events.
		sr := processStreamEvents(ctx, cfg, &st, eventCh, chatStart, timelineSnapshotLen, toolRoundBuf, cb)
		if sr.terminalError != nil {
			return st, fmt.Errorf("stream error round %d: %w", st.Round, sr.terminalError)
		}
		if sr.nextRound {
			continue
		}

		if sr.streamUsage != nil {
			st.CumulativeUsage.add(*sr.streamUsage)
			if cfg.Emitter != nil {
				cfg.Emitter.Emit(observability.AgentEvent{
					Type:  observability.EventProviderResponse,
					Round: st.Round,
					Data: map[string]any{
						"prompt_tokens":     sr.streamUsage.PromptTokens,
						"completion_tokens": sr.streamUsage.CompletionTokens,
						"total_tokens":      sr.streamUsage.TotalTokens,
						"cumulative_tokens": st.CumulativeUsage.TotalTokens,
					},
				})
			}
		}

		// Phase 5: Non-truncation finish_reason routing.
		if sr.finishReason == "insufficient_system_resource" {
			if st.Recovery.BackoffCount >= MaxBackoffRetries || !cb.Allow() {
				st.Recovery.Terminal = true
				return st, fmt.Errorf("backoff recovery exhausted: %s", sr.finishReason)
			}
			st.Recovery.BackoffCount++
			st.Timeline = st.Timeline[:timelineSnapshotLen]
			rollbackToolObs(cfg.Emitter, toolRoundBuf, st.Round)
			select {
			case <-time.After(backoffDuration(st.Recovery.BackoffCount)):
			case <-ctx.Done():
				return st, ctx.Err()
			}
			st.Round--
			continue
		}

		// Phase 6: Truncation detection.
		if sr.finishReason == "length" || sr.finishReason == "max_tokens" ||
			(sr.finishReason == "" && isTruncated(sr.contentText, sr.toolCalls, sr.hadAnyDelta)) {
			if st.Recovery.ContinueCount >= MaxContinueRetries {
				st.Recovery.Terminal = true
				return st, fmt.Errorf("continue recovery exhausted")
			}
			st.Recovery.ContinueCount++
			rollbackToolObs(cfg.Emitter, toolRoundBuf, st.Round)
			applyContinueRule(&st, sr.contentText, sr.toolCalls)
			continue
		}

		// Phase 7: Build assistant message.
		sr.asst = model.Message{
			Role:             "assistant",
			Content:          sr.contentText,
			ReasoningContent: sr.thinkingText,
			ToolCalls:        sr.toolCalls,
		}
		st.Messages = append(st.Messages, sr.asst)

		// No tool calls → conversation complete.
		if len(sr.toolCalls) == 0 {
			st.TransitionReason = TransitionModelCompleted
			saveSession(cfg, st)
			emitRoundEnd(cfg, &st, toolRoundBuf, roundStart, sr.chatDuration)
			break
		}

		// Phase 8: Execute tools.
		sr.results = executeTools(ctx, cfg, st.Round, sr.toolCalls, toolRoundBuf)
		for _, r := range sr.results {
			st.Messages = append(st.Messages, r.message)
			st.ToolResultBlocks = append(st.ToolResultBlocks, ToolResultBlock{
				Type:      "tool_result",
				ToolUseID: r.toolCallID,
				Content:   r.content,
			})
			summary := r.content
			if len(summary) > 200 {
				summary = summary[:200]
			}
			appendTimeline(&st, "tool", map[string]any{
				"phase":       "result",
				"name":        r.name,
				"call_id":     r.toolCallID,
				"success":     !r.isError,
				"summary":     summary,
				"duration_ms": r.durationMs,
			})
		}

		// Phase 9: Context operations (HTML detection, todos, memory).
		hadModification = processContextOperations(cfg, &st, sr.toolCalls, sr.results, hadModification)

		// Save session after each round.
		saveSession(cfg, st)

		// Phase 10: Post-round hooks + round-end event.
		emitRoundEnd(cfg, &st, toolRoundBuf, roundStart, sr.chatDuration)
	}

	// Finalize: create ONE version snapshot per conversation.
	if hadModification && cfg.ContextManager != nil {
		title := generateVersionTitle(ctx, cfg.Provider, cfg.UserMessage, cfg.Emitter)
		newVersion, err := cfg.ContextManager.FinalizeSnapshot(title)
		if err != nil {
			if cfg.Emitter != nil {
				cfg.Emitter.Emit(observability.AgentEvent{
					Type: "error",
					Data: map[string]any{"message": "finalize snapshot: " + err.Error()},
				})
			}
		}
		if newVersion > 0 {
			st.Version = newVersion
		}
	}

	if st.Round > max && st.TransitionReason == TransitionModelRequestedTools {
		return st, fmt.Errorf("agent: exceeded MaxRounds=%d while tools still requested", max)
	}
	return st, nil
}

// emitRoundEnd fires post-round hooks and the round_end emitter event.
func emitRoundEnd(cfg StreamingLoopConfig, st *LoopState, toolRoundBuf *observability.RoundEventBuffer, roundStart time.Time, chatDuration time.Duration) {
	if cfg.HookEngine != nil && cfg.SessionState != nil {
		warnings := cfg.HookEngine.RecordRoundEnd(context.Background(), st.Round)
		if len(warnings) > 0 {
			cfg.SessionState.AddPendingWarnings(warnings)
			if cfg.Emitter != nil {
				for _, w := range warnings {
					cfg.Emitter.Emit(observability.AgentEvent{
						Type:  "hook:warn",
						Round: st.Round,
						Data:  map[string]any{"message": w},
					})
				}
			}
		}
	}
	commitToolObs(cfg.Emitter, toolRoundBuf)
	if cfg.Emitter != nil {
		cfg.Emitter.Emit(observability.AgentEvent{
			Type:  "round_end",
			Round: st.Round,
			Data: map[string]any{
				"duration_ms":      time.Since(roundStart).Milliseconds(),
				"chat_duration_ms": chatDuration.Milliseconds(),
			},
		})
	}
}

// ─── Helpers ───

func appendTimeline(st *LoopState, eventType string, data map[string]any) {
	if st == nil {
		return
	}
	if eventType == "thinking" || eventType == "text" {
		if len(st.Timeline) > 0 {
			last := &st.Timeline[len(st.Timeline)-1]
			if last.Event == eventType {
				if existing, ok := last.Data["content"].(string); ok {
					if delta, ok := data["content"].(string); ok {
						last.Data["content"] = existing + delta
						last.Timestamp = time.Now()
						return
					}
				}
			}
		}
	}
	st.Timeline = append(st.Timeline, TimelineEvent{
		Event:     eventType,
		Timestamp: time.Now(),
		Data:      data,
	})
}

func applyContinueRule(st *LoopState, contentText string, toolCalls []model.ToolCall) {
	prompt := BuildContinuePrompt(contentText, toolCalls)

	allValid := !hasBrokenToolCalls(toolCalls)
	if len(toolCalls) > 0 && allValid {
		st.Messages = append(st.Messages, model.Message{
			Role:      "assistant",
			Content:   contentText,
			ToolCalls: toolCalls,
		})
		for _, tc := range toolCalls {
			st.Messages = append(st.Messages, model.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    "Tool executed before truncation.",
			})
		}
	} else if contentText != "" {
		st.Messages = append(st.Messages, model.Message{
			Role:    "assistant",
			Content: contentText,
		})
	}
	st.Messages = append(st.Messages, model.Message{
		Role:    "user",
		Content: prompt,
	})
}

func saveSession(cfg StreamingLoopConfig, st LoopState) {
	if cfg.SessionStore == nil {
		return
	}
	id := cfg.SessionID
	if id == "" {
		id = time.Now().Format("20060102_150405")
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		log.Printf("saveSession marshal: %v", err)
		return
	}
	cfg.SessionStore.Save(id, data)
}

func extractPathFromJSON(argsJSON string) string {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ""
	}
	return args.Path
}

func isMemoryPath(workspacePath, memoryBase, toolPath string) bool {
	if toolPath == "" {
		return false
	}
	clean := filepath.Clean(toolPath)
	if !filepath.IsAbs(clean) {
		clean = filepath.Join(workspacePath, clean)
	}
	absPath, err := filepath.Abs(clean)
	if err != nil {
		return false
	}
	absBase, err := filepath.Abs(memoryBase)
	if err != nil {
		return false
	}
	baseWithSep := absBase + string(filepath.Separator)
	return absPath == absBase || strings.HasPrefix(absPath, baseWithSep)
}

func applyRecoveryAction(
	ctx context.Context,
	cfg StreamingLoopConfig,
	st *LoopState,
	action RecoveryAction,
	actionErr error,
	timelineSnapshotLen int,
	toolRoundBuf *observability.RoundEventBuffer,
) (skipToNextRound bool, err error) {
	switch action {
	case RecoveryBackoff:
		st.Recovery.BackoffCount++
		if cfg.Emitter != nil {
			cfg.Emitter.Emit(observability.AgentEvent{
				Type:  observability.EventRecoveryBackoff,
				Round: st.Round,
				Data: map[string]any{
					"attempt": st.Recovery.BackoffCount,
					"reason":  actionErr.Error(),
				},
			})
		}
		select {
		case <-time.After(backoffDuration(st.Recovery.BackoffCount)):
		case <-ctx.Done():
			return false, ctx.Err()
		}
		st.Timeline = st.Timeline[:timelineSnapshotLen]
		rollbackToolObs(cfg.Emitter, toolRoundBuf, st.Round)
		st.Round--
		return true, nil
	case RecoveryCompress:
		st.Recovery.CompressCount++
		if cfg.Emitter != nil {
			cfg.Emitter.Emit(observability.AgentEvent{
				Type:  observability.EventRecoveryCompress,
				Round: st.Round,
				Data: map[string]any{
					"attempt": st.Recovery.CompressCount,
					"reason":  actionErr.Error(),
				},
			})
		}
		if cfg.ContextManager == nil {
			emitError(cfg, st, actionErr)
			st.Recovery.Terminal = true
			return false, fmt.Errorf("compress required but ContextManager is nil: %w", actionErr)
		}
		tlAny := make([]any, len(st.Timeline))
		for i, e := range st.Timeline {
			tlAny[i] = e
		}
		summary := BuildCompressPrompt(st.Messages, tlAny)
		st.Messages = cfg.ContextManager.CompressMessages(st.Messages, summary)
		st.Timeline = st.Timeline[:timelineSnapshotLen]
		rollbackToolObs(cfg.Emitter, toolRoundBuf, st.Round)
		st.Round--
		return true, nil
	default:
		return false, actionErr
	}
}

func backoffDuration(attempt int) time.Duration {
	const maxBackoff = 30 * time.Second
	d := time.Duration(1<<uint(attempt)) * time.Second
	if d > maxBackoff {
		d = maxBackoff
	}
	jitter := time.Duration(float64(d) * 0.25)
	return d - jitter
}
