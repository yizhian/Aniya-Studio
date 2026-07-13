package agent

import (
	"context"
	"time"

	"agentgo/internal/model"
	"agentgo/internal/observability"
	"agentgo/internal/provider"
)

// streamResult holds the accumulated state from processing one SSE event stream.
type streamResult struct {
	contentText   string
	thinkingText  string
	toolCalls     []model.ToolCall
	finishReason  string
	streamUsage   *model.Usage
	hadAnyDelta   bool
	toolAccLen    map[int]int
	results       []toolResult
	chatDuration  time.Duration
	asst          model.Message
	// terminalError is set when the stream encounters a non-recoverable error.
	terminalError error
	// nextRound signals that the caller should skip to the next round (goto nextRound).
	nextRound bool
}

// processStreamEvents reads SSE events from the channel and accumulates the response.
// Recovery actions for continuation, backoff, and compression are applied inline.
// Returns the accumulated result. The caller must check terminalError and nextRound.
func processStreamEvents(
	ctx context.Context,
	cfg StreamingLoopConfig,
	st *LoopState,
	eventCh <-chan provider.StreamEvent,
	chatStart time.Time,
	timelineSnapshotLen int,
	toolRoundBuf *observability.RoundEventBuffer,
	cb *CircuitBreaker,
) streamResult {
	var sr streamResult
	sr.toolAccLen = make(map[int]int)
	chatOver := false

	for ev := range eventCh {
		switch ev.Type {
		case provider.EventThinking:
			sr.thinkingText += ev.Delta
			sr.hadAnyDelta = true
			if cfg.Emitter != nil {
				cfg.Emitter.Emit(observability.AgentEvent{
					Type:  "thinking",
					Round: st.Round,
					Data:  map[string]any{"text": ev.Delta},
				})
			}
			appendTimeline(st, "thinking", map[string]any{"content": ev.Delta})

		case provider.EventTextDelta:
			sr.contentText += ev.Delta
			sr.hadAnyDelta = true
			if cfg.Emitter != nil {
				cfg.Emitter.Emit(observability.AgentEvent{
					Type:  "text",
					Round: st.Round,
					Data:  map[string]any{"text": ev.Delta},
				})
			}
			appendTimeline(st, "text", map[string]any{"content": ev.Delta})

		case provider.EventToolCallStart:
			sr.hadAnyDelta = true
			bufferToolObs(cfg.Emitter, toolRoundBuf, observability.AgentEvent{
				Type:  "tool_call_start",
				Round: st.Round,
				Data:  map[string]any{"name": ev.ToolCallName, "id": ev.ToolCallID, "tool_call_index": ev.ToolCallIndex},
			})
			appendTimeline(st, "tool", map[string]any{
				"phase":   "start",
				"name":    ev.ToolCallName,
				"call_id": ev.ToolCallID,
			})

		case provider.EventToolCallDelta:
			sr.hadAnyDelta = true
			if cfg.Emitter != nil {
				sr.toolAccLen[ev.ToolCallIndex] += len(ev.Delta)
				bufferToolObs(cfg.Emitter, toolRoundBuf, observability.AgentEvent{
					Type:  "tool_call_delta",
					Round: st.Round,
					Data: map[string]any{
						"name":              ev.ToolCallName,
						"accumulated_chars": sr.toolAccLen[ev.ToolCallIndex],
					},
				})
			}

		case provider.EventToolCallComplete:
			sr.hadAnyDelta = true
			st.TransitionReason = TransitionModelRequestedTools
			tc := model.ToolCall{
				ID:   ev.ToolCallID,
				Type: "function",
				Function: model.ToolCallFunction{
					Name:      ev.ToolCallName,
					Arguments: ev.ToolCall.Arguments,
				},
			}
			sr.toolCalls = append(sr.toolCalls, tc)

			bufferToolObs(cfg.Emitter, toolRoundBuf, observability.AgentEvent{
				Type:  "tool_call_complete",
				Round: st.Round,
				Data: map[string]any{
					"name":      ev.ToolCallName,
					"id":        ev.ToolCallID,
					"arguments": ev.ToolCall.Arguments,
				},
			})

		case provider.EventDone:
			sr.finishReason = ev.FinishReason
			sr.streamUsage = ev.Usage
			chatOver = true

		case provider.EventError:
			action := ClassifyRecovery(ev.Error, sr.finishReason, sr.contentText, sr.toolCalls, sr.hadAnyDelta, &st.Recovery, cb)
			switch action {
			case RecoveryContinue:
				if st.Recovery.ContinueCount >= MaxContinueRetries {
					emitError(cfg, st, ev.Error)
					st.Recovery.Terminal = true
					sr.terminalError = ev.Error
					return sr
				}
				st.Recovery.ContinueCount++
				rollbackToolObs(cfg.Emitter, toolRoundBuf, st.Round)
				applyContinueRule(st, sr.contentText, sr.toolCalls)
				sr.nextRound = true
				return sr
			case RecoveryBackoff, RecoveryCompress:
				skip, recErr := applyRecoveryAction(ctx, cfg, st, action, ev.Error, timelineSnapshotLen, toolRoundBuf)
				if recErr != nil {
					emitError(cfg, st, ev.Error)
					st.Recovery.Terminal = true
					sr.terminalError = ev.Error
					return sr
				}
				if skip {
					sr.nextRound = true
					return sr
				}
			default:
				emitError(cfg, st, ev.Error)
				st.Recovery.Terminal = true
				sr.terminalError = ev.Error
				return sr
			}
		}
	}

	_ = chatOver
	sr.chatDuration = time.Since(chatStart)
	return sr
}

// emitError emits an error event through the configured emitter, if any.
func emitError(cfg StreamingLoopConfig, st *LoopState, err error) {
	if cfg.Emitter != nil {
		cfg.Emitter.Emit(observability.AgentEvent{
			Type:  "error",
			Round: st.Round,
			Data:  map[string]any{"message": err.Error()},
		})
	}
}
