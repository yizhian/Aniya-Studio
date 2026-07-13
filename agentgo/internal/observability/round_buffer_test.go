package observability

import (
	"sync"
	"testing"
)

func TestRoundEventBuffer_FlushDiscard(t *testing.T) {
	e := NewEmitter()
	ch := make(chan AgentEvent, 8)
	e.Subscribe(ch)

	buf := NewRoundEventBuffer()
	buf.Emit(AgentEvent{Type: "tool_call_start", Round: 1, Data: map[string]any{"name": "read_file"}})
	buf.Discard()
	buf.Emit(AgentEvent{Type: "tool_call_start", Round: 1, Data: map[string]any{"name": "skill"}})
	buf.Flush(e)

	if len(ch) != 1 {
		t.Fatalf("expected 1 flushed event after discard, got %d", len(ch))
	}
	ev := <-ch
	if ev.Type != "tool_call_start" || ev.Data["name"] != "skill" {
		t.Fatalf("unexpected flushed event: %+v", ev)
	}
}

func TestRoundEventBuffer_ConcurrentEmit(t *testing.T) {
	buf := NewRoundEventBuffer()
	var wg sync.WaitGroup

	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				buf.Emit(AgentEvent{Type: "ev", Round: id, Data: map[string]any{"n": i}})
			}
		}(g)
	}
	wg.Wait()

	if buf.Len() != 1000 {
		t.Errorf("expected 1000 events, got %d", buf.Len())
	}
}

func TestRoundEventBuffer_Len(t *testing.T) {
	buf := NewRoundEventBuffer()
	if buf.Len() != 0 {
		t.Errorf("expected 0 for empty buffer, got %d", buf.Len())
	}

	buf.Emit(AgentEvent{Type: "a"})
	buf.Emit(AgentEvent{Type: "b"})
	if buf.Len() != 2 {
		t.Errorf("expected 2, got %d", buf.Len())
	}

	buf.Discard()
	if buf.Len() != 0 {
		t.Errorf("expected 0 after discard, got %d", buf.Len())
	}
}

func TestRoundEventBuffer_NilReceiver(t *testing.T) {
	var buf *RoundEventBuffer

	// None of these should panic.
	buf.Emit(AgentEvent{Type: "x"})
	buf.Discard()
	buf.Flush(nil)

	if buf.Len() != 0 {
		t.Errorf("expected 0 for nil buffer, got %d", buf.Len())
	}
}

func TestRoundEventBuffer_FlushNilEmitter(t *testing.T) {
	buf := NewRoundEventBuffer()
	buf.Emit(AgentEvent{Type: "x"})
	buf.Flush(nil) // should not panic
	if buf.Len() != 0 {
		t.Error("expected buffer cleared after flush even with nil emitter")
	}
}
