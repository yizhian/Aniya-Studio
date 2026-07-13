// Package observability provides real-time event logging for the agent loop.
//
// Events flow: StreamingLoop → Emitter → Subscribers (ConsoleObserver, etc.)
package observability

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Event type constants define the canonical event type strings used across the codebase.
const (
	// Existing (already emitted in codebase)
	EventRound            = "round"
	EventThinking         = "thinking"
	EventText             = "text"
	EventToolCallStart    = "tool_call_start"
	EventToolCallDelta    = "tool_call_delta"
	EventToolCallComplete = "tool_call_complete"
	EventToolResult       = "tool_result"
	EventRoundEnd         = "round_end"
	EventRoundRetry       = "round_retry"
	EventError            = "error"
	EventHookBlocked      = "hook:blocked"
	EventHookWarn         = "hook:warn"
	EventTodoWrite        = "todo_write"

	// Provider layer
	EventProviderRequest   = "provider:request"
	EventProviderResponse  = "provider:response"
	EventProviderRetry     = "provider:retry"
	EventProviderStreamErr = "provider:stream_error"

	// Recovery
	EventRecoveryBackoff  = "recovery:backoff"
	EventRecoveryContinue = "recovery:continue"
	EventRecoveryCompress = "recovery:compress"

	// Context management
	EventContextSnapshot = "context:snapshot"
	EventContextCompress = "context:compress"
	EventContextTrim     = "context:trim"
	EventContextBudget   = "context:budget"

	// Hook engine
	EventHookError = "hook:error"

	// Tool system
	EventToolBlocked = "tool:blocked"
	EventToolError   = "tool:error"

	// Circuit breaker
	EventCircuitOpen     = "circuit:open"
	EventCircuitHalfOpen = "circuit:half_open"
	EventCircuitClosed   = "circuit:closed"

	// Infrastructure
	EventSessionSaveError = "session:save_error"
	EventMemoryWrite      = "memory:write"
	EventMemorySecurity   = "memory:security"
	EventUploadParseError = "upload:parse_error"
	EventSSEConnect       = "sse:connect"
	EventSSEDisconnect    = "sse:disconnect"
	EventSSEInitFailed    = "sse:init_failed"
	EventProviderConfig   = "provider:config_update"
	EventProviderTest     = "provider:test_result"
)

// AgentEvent is emitted by the streaming loop for real-time display.
type AgentEvent struct {
	Time  time.Time
	Type  string
	Round int
	Data  map[string]any
}

// Emitter broadcasts events to all registered subscribers.
type Emitter struct {
	mu          sync.RWMutex
	subscribers []chan<- AgentEvent
}

func NewEmitter() *Emitter {
	return &Emitter{}
}

// Subscribe adds a channel that receives all future events.
// The caller should drain the channel to avoid blocking the emitter.
func (e *Emitter) Subscribe(ch chan<- AgentEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.subscribers = append(e.subscribers, ch)
}

// Emit sends an event to all subscribers without blocking.
// Slow or full subscribers have their events dropped to prevent backpressure
// from stalling the agent main loop (critical for SSE/long-connection scenarios).
func (e *Emitter) Emit(ev AgentEvent) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	ev.Time = time.Now()
	for _, sub := range e.subscribers {
		select {
		case sub <- ev:
		default:
			// Subscriber not ready; drop event to avoid blocking the emitter.
		}
	}
}

// Close unsubscribes all subscribers.
func (e *Emitter) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, sub := range e.subscribers {
		func() {
			defer func() { recover() }()
			close(sub)
		}()
	}
	e.subscribers = nil
}

// ---------------------------------------------------------------------------
// ANSI color codes
// ---------------------------------------------------------------------------

const (
	colorDim    = "\033[2m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorReset  = "\033[0m"
)

// ConsoleObserver prints agent events to stdout in real-time.
type ConsoleObserver struct {
	ch                   chan AgentEvent
	toolDeltaMilestones  map[string]int // tool_name → last printed 500-char milestone
	printedRound         bool           // suppress leading newline before first round
}

func NewConsoleObserver() *ConsoleObserver {
	o := &ConsoleObserver{
		ch:                  make(chan AgentEvent, 128),
		toolDeltaMilestones: make(map[string]int),
	}
	go o.loop()
	return o
}

func (o *ConsoleObserver) Subscribe(e *Emitter) {
	e.Subscribe(o.ch)
}

// Close stops the observer goroutine.
func (o *ConsoleObserver) Close() {
	close(o.ch)
}

func (o *ConsoleObserver) loop() {
	var buf strings.Builder
	var bufRound int
	var timerCh <-chan time.Time

	for {
		select {
		case ev, ok := <-o.ch:
			if !ok {
				if buf.Len() > 0 {
					o.flushBuffer(&buf, bufRound)
				}
				return
			}

			if ev.Type == "text" {
				if buf.Len() == 0 {
					bufRound = ev.Round
				}
				text, _ := ev.Data["text"].(string)
				buf.WriteString(text)
				if strings.Contains(text, "\n") {
					timerCh = nil
					o.flushBuffer(&buf, bufRound)
				} else {
					timerCh = time.After(300 * time.Millisecond)
				}
				continue
			}

			// Non-text event: flush any buffered text first.
			if buf.Len() > 0 {
				o.flushBuffer(&buf, bufRound)
				timerCh = nil
			}
			o.print(ev)

		case <-timerCh:
			if buf.Len() > 0 {
				o.flushBuffer(&buf, bufRound)
			}
			timerCh = nil
		}
	}
}

func (o *ConsoleObserver) flushBuffer(buf *strings.Builder, round int) {
	text := strings.TrimSpace(buf.String())
	buf.Reset()
	if text == "" {
		return
	}
	prefix := fmt.Sprintf("[%s] [R%d]", time.Now().Format("15:04:05"), round)
	fmt.Printf("%s%s💬 %s%s\n", prefix, colorGreen, text, colorReset)
}

func (o *ConsoleObserver) print(ev AgentEvent) {
	prefix := fmt.Sprintf("[%s] [R%d]", ev.Time.Format("15:04:05"), ev.Round)

	switch ev.Type {
	case "round":
		if o.printedRound {
			fmt.Println()
		}
		o.printedRound = true
		fmt.Printf("%s ━━━ Round %d ━━━\n", prefix, ev.Round)

	case "thinking":
		text, _ := ev.Data["text"].(string)
		text = strings.TrimSpace(text)
		if text != "" {
			fmt.Printf("%s%s💭 %s%s\n", prefix, colorDim, text, colorReset)
		}

	case "tool_call_start":
		name, _ := ev.Data["name"].(string)
		fmt.Printf("%s%s🛠  %s%s\n", prefix, colorCyan, name, colorReset)

	case "tool_call_delta":
		name, _ := ev.Data["name"].(string)
		var total int
		switch v := ev.Data["accumulated_chars"].(type) {
		case float64:
			total = int(v)
		case int:
			total = v
		default:
			total = 0
		}
		if name != "" && total > 0 {
			milestone := total / 500
			if milestone > o.toolDeltaMilestones[name] {
				o.toolDeltaMilestones[name] = milestone
				fmt.Printf("%s%s  ⟳ %s (%d chars generating...)%s\n", prefix, colorCyan, name, total, colorReset)
			}
		}
	case "tool_call_complete":
		name, _ := ev.Data["name"].(string)
		args, _ := ev.Data["arguments"].(string)
		if len(args) > 80 {
			args = args[:80] + "..."
		}
		dur := formatDuration(ev.Data["duration_ms"])
		fmt.Printf("%s%s  → %s(%s)%s%s\n", prefix, colorCyan, name, args, dur, colorReset)

	case "tool_result":
		name, _ := ev.Data["name"].(string)
		result, _ := ev.Data["content"].(string)
		isError, _ := ev.Data["is_error"].(bool)
		dur := formatDuration(ev.Data["duration_ms"])
		resultLen := len(result)
		if len(result) > 300 {
			result = result[:300] + "..."
		}
		if isError {
			fmt.Printf("%s%s  ⚠ %s: %s%s%s\n", prefix, colorRed, name, result, dur, colorReset)
		} else {
			path, _ := ev.Data["path"].(string)
			if path != "" {
				fmt.Printf("%s%s  ✅ %s(%s) returned %d chars%s%s\n", prefix, colorGreen, name, path, resultLen, dur, colorReset)
			} else {
				fmt.Printf("%s%s  ✅ %s returned %d chars%s%s\n", prefix, colorGreen, name, resultLen, dur, colorReset)
			}
		}

	case "round_end":
		dur := formatDuration(ev.Data["duration_ms"])
		fmt.Printf("%s── Round %d end%s\n", prefix, ev.Round, dur)

	case "round_retry":
		msg, _ := ev.Data["message"].(string)
		if msg == "" {
			msg = "retrying"
		}
		fmt.Printf("%s%s↻ Round %d %s%s\n", prefix, colorDim, ev.Round, msg, colorReset)

	case "hook:warn":
		msg, _ := ev.Data["message"].(string)
		if idx := strings.Index(msg, "\n"); idx > 0 {
			msg = msg[:idx]
		}
		fmt.Printf("%s%s🔧 %s%s\n", prefix, colorDim, msg, colorReset)


	case "todo_write":
		todos, _ := ev.Data["todos"].([]any)
		fmt.Printf("%s%s📋 Todo: %d 项%s\n", prefix, colorCyan, len(todos), colorReset)

	case EventProviderRequest:
		model, _ := ev.Data["model"].(string)
		msgCount, _ := ev.Data["message_count"].(float64)
		toolCount, _ := ev.Data["tool_count"].(float64)
		fmt.Printf("%s%s🌐 API → %s (%d msgs, %d tools)%s\n",
			prefix, colorDim, model, int(msgCount), int(toolCount), colorReset)

	case EventProviderResponse:
		status, _ := ev.Data["status"].(float64)
		dur := formatDuration(ev.Data["duration_ms"])
		tokens, _ := ev.Data["total_tokens"].(float64)
		if int(status) > 0 {
			fmt.Printf("%s%s  ← HTTP %d%s tokens:%.0f%s\n",
				prefix, colorDim, int(status), dur, tokens, colorReset)
		} else {
			fmt.Printf("%s%s  ← tokens:%.0f%s%s\n",
				prefix, colorDim, tokens, dur, colorReset)
		}

	case EventProviderRetry:
		attempt, _ := ev.Data["attempt"].(float64)
		errMsg, _ := ev.Data["error"].(string)
		fmt.Printf("%s%s  ↻ Retry %d: %s%s\n",
			prefix, colorYellow, int(attempt), errMsg, colorReset)

	case EventRecoveryBackoff, EventRecoveryContinue, EventRecoveryCompress:
		reason, _ := ev.Data["reason"].(string)
		attempt, _ := ev.Data["attempt"].(float64)
		fmt.Printf("%s%s  ↻ %s (attempt %d): %s%s\n",
			prefix, colorDim, ev.Type, int(attempt), reason, colorReset)

	case EventCircuitOpen:
		fmt.Printf("%s%s⚡ Circuit OPEN — requests blocked%s\n", prefix, colorRed, colorReset)

	case EventCircuitHalfOpen:
		fmt.Printf("%s%s⚡ Circuit HALF-OPEN — testing%s\n", prefix, colorYellow, colorReset)

	case EventCircuitClosed:
		fmt.Printf("%s%s⚡ Circuit CLOSED — resumed%s\n", prefix, colorGreen, colorReset)

	case EventHookError:
		hookName, _ := ev.Data["hook_name"].(string)
		mountPoint, _ := ev.Data["mount_point"].(string)
		errMsg, _ := ev.Data["error"].(string)
		fmt.Printf("%s%s🔧 Hook error [%s @ %s]: %s%s\n",
			prefix, colorYellow, hookName, mountPoint, errMsg, colorReset)

	case EventToolBlocked:
		toolName, _ := ev.Data["tool_name"].(string)
		reason, _ := ev.Data["reason"].(string)
		fmt.Printf("%s%s🚫 Tool blocked [%s]: %s%s\n",
			prefix, colorYellow, toolName, reason, colorReset)

	case EventContextSnapshot:
		version, _ := ev.Data["version"].(float64)
		slideCount, _ := ev.Data["slide_count"].(float64)
		fmt.Printf("%s%s📸 Version v%d created (%d slides)%s\n",
			prefix, colorDim, int(version), int(slideCount), colorReset)

	case EventContextCompress:
		preCount, _ := ev.Data["pre_count"].(float64)
		postCount, _ := ev.Data["post_count"].(float64)
		fmt.Printf("%s%s🗜  Context compressed: %d → %d messages%s\n",
			prefix, colorDim, int(preCount), int(postCount), colorReset)

	case EventContextBudget:
		totalTokens, _ := ev.Data["total_tokens_est"].(float64)
		budget, _ := ev.Data["budget"].(float64)
		fmt.Printf("%s%s💰 Budget: est %d / limit %d tokens%s\n",
			prefix, colorDim, int(totalTokens), int(budget), colorReset)

	case EventSSEConnect:
		sessID, _ := ev.Data["session_id"].(string)
		fmt.Printf("%s%s🔌 SSE connected (session: %s)%s\n", prefix, colorDim, sessID, colorReset)

	case EventSSEDisconnect:
		fmt.Printf("%s%s🔌 SSE disconnected%s\n", prefix, colorDim, colorReset)

	case EventMemorySecurity:
		path, _ := ev.Data["path"].(string)
		errMsg, _ := ev.Data["error"].(string)
		fmt.Printf("%s%s🔒 Memory security: %s (%s)%s\n",
			prefix, colorYellow, path, errMsg, colorReset)

	case "hook:blocked":
		msg, _ := ev.Data["message"].(string)
		fmt.Printf("%s%s🚫 %s%s\n", prefix, colorYellow, msg, colorReset)

	case "error":
		// Tool errors are already printed via tool_result; skip duplicates.
		if _, isTool := ev.Data["tool_name"]; isTool {
			break
		}
		msg, _ := ev.Data["message"].(string)
		fmt.Printf("%s%s❌ %s%s\n", prefix, colorRed, msg, colorReset)
	}
}

func formatDuration(v any) string {
	if v == nil {
		return ""
	}
	var ms int64
	switch n := v.(type) {
	case float64:
		ms = int64(n)
	case int64:
		ms = n
	case int:
		ms = int64(n)
	default:
		return ""
	}
	if ms == 0 {
		return ""
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf(" ⏱ %dms", ms)
	}
	return fmt.Sprintf(" ⏱ %.1fs", d.Seconds())
}
