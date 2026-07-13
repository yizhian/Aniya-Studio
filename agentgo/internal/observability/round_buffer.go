package observability

import "sync"

// RoundEventBuffer holds observability events for the current round until the
// round commits successfully. On recovery rollback, Discard clears buffered
// events so stdout/SSE stay consistent with the rolled-back timeline.
type RoundEventBuffer struct {
	mu     sync.Mutex
	events []AgentEvent
}

func NewRoundEventBuffer() *RoundEventBuffer {
	return &RoundEventBuffer{}
}

func (b *RoundEventBuffer) Emit(ev AgentEvent) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.events = append(b.events, ev)
	b.mu.Unlock()
}

func (b *RoundEventBuffer) Flush(e *Emitter) {
	if b == nil {
		return
	}
	b.mu.Lock()
	events := b.events
	b.events = nil
	b.mu.Unlock()
	if e != nil {
		for _, ev := range events {
			e.Emit(ev)
		}
	}
}

func (b *RoundEventBuffer) Discard() {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.events = nil
	b.mu.Unlock()
}

func (b *RoundEventBuffer) Len() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}
