package agent

import "agentgo/internal/observability"

func bufferToolObs(emitter *observability.Emitter, buf *observability.RoundEventBuffer, ev observability.AgentEvent) {
	if emitter == nil {
		return
	}
	if buf != nil {
		buf.Emit(ev)
	} else {
		emitter.Emit(ev)
	}
}

func rollbackToolObs(emitter *observability.Emitter, buf *observability.RoundEventBuffer, round int) {
	if buf != nil {
		buf.Discard()
	}
	if emitter != nil {
		emitter.Emit(observability.AgentEvent{
			Type:  "round_retry",
			Round: round,
			Data:  map[string]any{"message": "round rolled back, retrying"},
		})
	}
}

func commitToolObs(emitter *observability.Emitter, buf *observability.RoundEventBuffer) {
	if buf != nil {
		buf.Flush(emitter)
	}
}
