package observability

import (
	"strings"
	"testing"
	"time"
)

func TestEmitter_SingleSubscriber(t *testing.T) {
	e := NewEmitter()
	ch := make(chan AgentEvent, 16)
	e.Subscribe(ch)

	ev := AgentEvent{
		Type:  "round",
		Round: 1,
		Data:  map[string]any{"key": "value"},
	}
	e.Emit(ev)

	select {
	case received := <-ch:
		if received.Type != "round" {
			t.Fatalf("expected type round, got %q", received.Type)
		}
		if received.Round != 1 {
			t.Fatalf("expected round 1, got %d", received.Round)
		}
		if received.Data["key"] != "value" {
			t.Fatalf("expected data key=value, got %v", received.Data["key"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}

	e.Close()
}

func TestEmitter_MultipleSubscribers(t *testing.T) {
	e := NewEmitter()
	ch1 := make(chan AgentEvent, 16)
	ch2 := make(chan AgentEvent, 16)
	ch3 := make(chan AgentEvent, 16)
	e.Subscribe(ch1)
	e.Subscribe(ch2)
	e.Subscribe(ch3)

	e.Emit(AgentEvent{Type: "text", Data: map[string]any{"text": "hello"}})

	for i, ch := range []chan AgentEvent{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			if received.Type != "text" {
				t.Fatalf("subscriber %d: expected type text, got %q", i, received.Type)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}

	e.Close()
}

func TestEmitter_TimeIsSet(t *testing.T) {
	e := NewEmitter()
	ch := make(chan AgentEvent, 16)
	e.Subscribe(ch)

	before := time.Now()
	e.Emit(AgentEvent{Type: "round"})
	after := time.Now()

	received := <-ch
	if received.Time.Before(before) || received.Time.After(after) {
		t.Fatalf("expected Time between %v and %v, got %v", before, after, received.Time)
	}

	e.Close()
}

func TestEmitter_EmitAfterClose_NoPanic(t *testing.T) {
	e := NewEmitter()
	ch := make(chan AgentEvent, 128)
	e.Subscribe(ch)
	e.Close()

	// Emit after close should not panic. The subscriber channel is closed,
	// so this sends on a closed channel — but we drain the subscriber first.
	drainChannel(ch)

	// Emitting after close: behavior depends on whether subscribers list was nil'd.
	// The key test is that this does not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Emit after Close panicked: %v", r)
		}
	}()
	e.Emit(AgentEvent{Type: "text"})
}

func drainChannel(ch chan AgentEvent) {
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		default:
			return
		}
	}
}

func TestConsoleObserver_ReceivesEvents(t *testing.T) {
	// ConsoleObserver prints to stdout; we mainly verify it doesn't block
	// the emitter and can be closed without leaking goroutines.
	e := NewEmitter()
	obs := NewConsoleObserver()
	obs.Subscribe(e)

	// Emit several events that should be processed without blocking.
	for i := 1; i <= 5; i++ {
		e.Emit(AgentEvent{Type: "round", Round: i})
	}
	e.Emit(AgentEvent{Type: "text", Round: 1, Data: map[string]any{"text": "hello"}})
	e.Emit(AgentEvent{Type: "round_end", Round: 1, Data: map[string]any{"duration_ms": 150}})

	// Give the observer time to process.
	time.Sleep(100 * time.Millisecond)
	obs.Close()
}

func TestConsoleObserver_CloseTerminates(t *testing.T) {
	obs := NewConsoleObserver()

	// Close should terminate the background goroutine.
	done := make(chan struct{})
	go func() {
		obs.Close()
		close(done)
	}()

	select {
	case <-done:
		// Success — Close didn't hang.
	case <-time.After(2 * time.Second):
		t.Fatal("ConsoleObserver.Close() hung")
	}
}

func TestConsoleObserver_TextBuffering(t *testing.T) {
	e := NewEmitter()
	obs := NewConsoleObserver()
	obs.Subscribe(e)

	// Emit text events without newlines — they should be buffered, not stall.
	for i := 0; i < 5; i++ {
		e.Emit(AgentEvent{Type: "text", Round: 1, Data: map[string]any{"text": "word "}})
	}
	// Emit a text event with newline to trigger flush.
	e.Emit(AgentEvent{Type: "text", Round: 1, Data: map[string]any{"text": "done.\n"}})

	time.Sleep(100 * time.Millisecond)
	obs.Close()
}

func TestFormatDuration_Direct(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		contains string
	}{
		{"nil", nil, ""},
		{"float64_zero", float64(0), ""},
		{"float64_ms", float64(500), "500ms"},
		{"float64_seconds", float64(1500), "1.5s"},
		{"int64_zero", int64(0), ""},
		{"int64_ms", int64(999), "999ms"},
		{"int64_seconds", int64(2000), "2.0s"},
		{"int_zero", int(0), ""},
		{"int_ms", int(300), "300ms"},
		{"int_seconds", int(3000), "3.0s"},
		{"string", "not_a_number", ""},
		{"bool_true", true, ""},
		{"bool_false", false, ""},
		{"exactly_1s", int64(1000), "1.0s"},
		{"large_ms", int64(60000), "60.0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.input)
			if tt.contains == "" {
				if got != "" {
					t.Errorf("expected empty, got %q", got)
				}
			} else if !strings.Contains(got, tt.contains) {
				t.Errorf("expected %q to contain %q", got, tt.contains)
			}
		})
	}
}

func TestConsoleObserver_PrintAllEventTypes(t *testing.T) {
	// Verify print() handles all event types without panicking.
	o := &ConsoleObserver{ch: make(chan AgentEvent, 1)}
	now := time.Now()
	types := []AgentEvent{
		{Type: "thinking", Round: 1, Time: now, Data: map[string]any{"text": "hmm..."}},
		{Type: "thinking", Round: 1, Time: now, Data: map[string]any{"text": ""}},
		{Type: "tool_call_start", Round: 2, Time: now, Data: map[string]any{"name": "write_file"}},
		{Type: "tool_call_delta", Round: 2, Time: now, Data: map[string]any{"text": "delta"}},
		{Type: "tool_call_complete", Round: 2, Time: now, Data: map[string]any{"name": "read_file", "arguments": `{"path":"f.txt"}`, "duration_ms": float64(1234)}},
		{Type: "tool_call_complete", Round: 2, Time: now, Data: map[string]any{"name": "tool", "arguments": "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijZZZ", "duration_ms": float64(50)}},
		{Type: "tool_result", Round: 2, Time: now, Data: map[string]any{"name": "read_file", "content": "result text", "is_error": false, "duration_ms": int64(42)}},
		{Type: "tool_result", Round: 2, Time: now, Data: map[string]any{"name": "bad_tool", "content": "error occurred", "is_error": true}},
		{Type: "tool_result", Round: 2, Time: now, Data: map[string]any{"name": "tool", "content": longStr(350), "is_error": false}},
		{Type: "round_end", Round: 3, Time: now, Data: map[string]any{"duration_ms": float64(5600)}},
		{Type: "error", Round: 1, Time: now, Data: map[string]any{"message": "something broke"}},
		{Type: "error", Round: 1, Time: now, Data: map[string]any{"tool_name": "read_file", "message": "ignored"}},
	}
	for _, ev := range types {
		o.print(ev)
	}
}

func longStr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

func TestConsoleObserver_ToolCallDeltaMilestone(t *testing.T) {
	// Test that print() throttles tool_call_delta output to every 500 chars.
	// First call at 400 chars: no milestone change (400/500=0, not >0)
	// First call at 500 chars: milestone 1 > 0 → print
	// Next at 600 chars: milestone 1 == 1 → skip
	// Next at 1000 chars: milestone 2 > 1 → print
	// Different tool name: independent milestone tracking
	o := &ConsoleObserver{
		ch:                   make(chan AgentEvent, 1),
		toolDeltaMilestones:  make(map[string]int),
	}
	now := time.Now()

	// write_file: 400 chars → milestone 0, not > 0, no print
	o.print(AgentEvent{Type: "tool_call_delta", Round: 1, Time: now, Data: map[string]any{
		"name": "write_file", "accumulated_chars": float64(400),
	}})
	if o.toolDeltaMilestones["write_file"] != 0 {
		t.Errorf("after 400 chars, milestone should be 0, got %d", o.toolDeltaMilestones["write_file"])
	}

	// write_file: 500 chars → milestone 1 > 0, print triggered
	o.print(AgentEvent{Type: "tool_call_delta", Round: 1, Time: now, Data: map[string]any{
		"name": "write_file", "accumulated_chars": float64(500),
	}})
	if o.toolDeltaMilestones["write_file"] != 1 {
		t.Errorf("after 500 chars, milestone should be 1, got %d", o.toolDeltaMilestones["write_file"])
	}

	// write_file: 600 chars → milestone 1 == 1, no print
	o.print(AgentEvent{Type: "tool_call_delta", Round: 1, Time: now, Data: map[string]any{
		"name": "write_file", "accumulated_chars": float64(600),
	}})
	if o.toolDeltaMilestones["write_file"] != 1 {
		t.Errorf("at 600 chars, milestone should still be 1, got %d", o.toolDeltaMilestones["write_file"])
	}

	// write_file: 1000 chars → milestone 2 > 1, print triggered
	o.print(AgentEvent{Type: "tool_call_delta", Round: 1, Time: now, Data: map[string]any{
		"name": "write_file", "accumulated_chars": float64(1000),
	}})
	if o.toolDeltaMilestones["write_file"] != 2 {
		t.Errorf("after 1000 chars, milestone should be 2, got %d", o.toolDeltaMilestones["write_file"])
	}

	// read_file: independent milestone tracking
	o.print(AgentEvent{Type: "tool_call_delta", Round: 1, Time: now, Data: map[string]any{
		"name": "read_file", "accumulated_chars": float64(2500),
	}})
	if o.toolDeltaMilestones["read_file"] != 5 {
		t.Errorf("after 2500 chars for read_file, milestone should be 5, got %d", o.toolDeltaMilestones["read_file"])
	}
}

func TestConsoleObserver_ToolCallDeltaMilestone_IntType(t *testing.T) {
	// Regression test: streaming_loop.go stores accumulated_chars as int (not float64).
	// The observer must handle both types.
	o := &ConsoleObserver{
		ch:                  make(chan AgentEvent, 1),
		toolDeltaMilestones: make(map[string]int),
	}
	now := time.Now()

	// 400 chars (int type): milestone not triggered
	o.print(AgentEvent{Type: "tool_call_delta", Round: 1, Time: now, Data: map[string]any{
		"name": "write_file", "accumulated_chars": int(400),
	}})
	if o.toolDeltaMilestones["write_file"] != 0 {
		t.Errorf("milestone should be 0, got %d", o.toolDeltaMilestones["write_file"])
	}

	// 500 chars (int type): should trigger print
	o.print(AgentEvent{Type: "tool_call_delta", Round: 1, Time: now, Data: map[string]any{
		"name": "write_file", "accumulated_chars": int(500),
	}})
	if o.toolDeltaMilestones["write_file"] != 1 {
		t.Errorf("milestone should be 1, got %d", o.toolDeltaMilestones["write_file"])
	}

	// 1500 chars (int type): should trigger second print
	o.print(AgentEvent{Type: "tool_call_delta", Round: 1, Time: now, Data: map[string]any{
		"name": "write_file", "accumulated_chars": int(1500),
	}})
	if o.toolDeltaMilestones["write_file"] != 3 {
		t.Errorf("milestone should be 3, got %d", o.toolDeltaMilestones["write_file"])
	}
}

func TestConsoleObserver_FlushBufferEdgeCases(t *testing.T) {
	t.Run("empty buffer", func(t *testing.T) {
		var buf strings.Builder
		o := &ConsoleObserver{}
		o.flushBuffer(&buf, 1)
		// No output expected; just verifying no panic.
	})

	t.Run("whitespace only", func(t *testing.T) {
		var buf strings.Builder
		buf.WriteString("   \n  \t  ")
		o := &ConsoleObserver{}
		o.flushBuffer(&buf, 2)
		// No output expected; just verifying no panic.
	})

	t.Run("valid content resets buffer", func(t *testing.T) {
		var buf strings.Builder
		buf.WriteString("content")
		o := &ConsoleObserver{}
		o.flushBuffer(&buf, 3)
		if buf.Len() != 0 {
			t.Error("buffer should be reset after flush")
		}
	})
}

func TestEmitter_NonBlockingOnFullSubscriber(t *testing.T) {
	e := NewEmitter()

	slow := make(chan AgentEvent)
	e.Subscribe(slow)

	normal := make(chan AgentEvent, 128)
	e.Subscribe(normal)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			e.Emit(AgentEvent{Type: "text", Round: 1, Data: map[string]any{"text": "x"}})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Emit blocked on slow subscriber")
	}

	e.Close()
	drainChannel(slow)
}
