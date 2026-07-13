package context

import (
	"fmt"
	"strings"

	"agentgo/internal/model"
	"agentgo/internal/observability"
)

// KeepRounds is the default number of assistant/tool round-trip cycles to retain.
const KeepRounds = 2

// SnapshotMessagePrefix is a stable marker for snapshot context messages.
// Keep this marker independent from human-readable content to avoid regressions
// when display formatting changes.
const SnapshotMessagePrefix = "[[SNAPSHOT_CONTEXT_V1]]"

// TrimMessages builds the message array to send to the LLM.
// It preserves: system prompt, a fresh design snapshot (if non-nil), and the last
// keepRounds conversation cycles in natural chronological order.
// Called before every LLM round so the snapshot is always current.
func TrimMessages(full []model.Message, keepRounds int, snapshot *DesignSnapshot) []model.Message {
	return TrimMessagesWithEmitter(full, keepRounds, snapshot, nil)
}

// TrimMessagesWithEmitter is like TrimMessages but emits context:trim events through the optional emitter.
func TrimMessagesWithEmitter(full []model.Message, keepRounds int, snapshot *DesignSnapshot, emitter *observability.Emitter) []model.Message {
	if keepRounds <= 0 {
		keepRounds = KeepRounds
	}

	systemIdx := -1
	lastUserIdx := -1
	var roundStarts []int

	for i := range full {
		switch full[i].Role {
		case "system":
			systemIdx = i
		case "user":
			if !isSnapshotMessage(full[i].Content) {
				lastUserIdx = i
			}
		case "assistant":
			roundStarts = append(roundStarts, i)
		}
	}

	startRoundIdx := 0
	if len(roundStarts) > keepRounds {
		startRoundIdx = len(roundStarts) - keepRounds
		removed := len(roundStarts) - keepRounds
		observability.EmitOrLog(emitter, observability.AgentEvent{
			Type: observability.EventContextTrim,
			Data: map[string]any{
				"total_rounds":   len(roundStarts),
				"kept_rounds":    keepRounds,
				"removed":        removed,
				"total_messages": len(full),
			},
		})
	}

	result := make([]model.Message, 0, 4+keepRounds*4)

	// 1. System prompt.
	if systemIdx >= 0 {
		result = append(result, full[systemIdx])
	}

	// 2. Fresh design snapshot (injected before every LLM call so it reflects the
	//    latest version created during this turn).
	if snapshot != nil && snapshot.SlideCount > 0 {
		result = append(result, model.Message{
			Role:    "user",
			Content: FormatSnapshotContext(snapshot),
		})
	}

	// 3. Walk from the earliest needed position to end, preserving chronological order.
	walkStart := len(full)
	if len(roundStarts) > 0 {
		walkStart = roundStarts[startRoundIdx]
	}
	// Include the user message that triggered these rounds if it's before the first kept round.
	if lastUserIdx >= 0 && lastUserIdx < walkStart {
		walkStart = lastUserIdx
	}

	// 3a. Scan backward from walkStart to include an adjacent compression
	// summary. Stop at assistant messages — don't cross round boundaries
	// into already-trimmed history.
	minIdx := walkStart
	for i := walkStart - 1; i >= 0; i-- {
		if full[i].Role == "assistant" {
			break
		}
		if full[i].Role == "user" && isCompressMessage(full[i].Content) {
			minIdx = i
			break
		}
	}
	walkStart = minIdx

	for i := walkStart; i < len(full); i++ {
		m := full[i]
		// Skip previously injected snapshot or progress user messages.
		if m.Role == "user" && (isSnapshotMessage(m.Content) || isProgressMessage(m.Content)) {
			continue
		}
		result = append(result, m)
	}

	return result
}

// FormatSnapshotContext formats a DesignSnapshot for injection into the system prompt.
func FormatSnapshotContext(s *DesignSnapshot) string {
	if s == nil || s.SlideCount == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(SnapshotMessagePrefix + "\n")
	b.WriteString("## Current Design State\n")
	if s.ActiveFile != "" {
		b.WriteString(fmt.Sprintf("- Active file: %s\n", s.ActiveFile))
	}
	if s.Title != "" {
		b.WriteString(fmt.Sprintf("- Title: %s\n", s.Title))
	}
	b.WriteString(fmt.Sprintf("- Slides: %d\n", s.SlideCount))
	if len(s.SlideHeadings) > 0 {
		b.WriteString("- Structure:\n")
		for _, h := range s.SlideHeadings {
			b.WriteString(fmt.Sprintf("  - %s\n", h))
		}
	}
	if len(s.ColorPalette) > 0 {
		b.WriteString("- Colors: ")
		first := true
		for k, v := range s.ColorPalette {
			if !first {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%s: %s", k, v))
			first = false
		}
		b.WriteString("\n")
	}
	if len(s.Fonts) > 0 {
		b.WriteString("- Fonts: ")
		for i, f := range s.Fonts {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%s (%s)", f.Family, f.Source))
		}
		b.WriteString("\n")
	}
	if len(s.CSSClasses) > 0 {
		b.WriteString(fmt.Sprintf("- CSS classes: %s\n", strings.Join(s.CSSClasses, ", ")))
	}
	return b.String()
}

// isSnapshotMessage returns true if the message content looks like a design snapshot.
func isSnapshotMessage(content string) bool {
	// Backward compatibility: keep recognizing older sessions that used
	// the previous snapshot prefix.
	return strings.HasPrefix(content, SnapshotMessagePrefix) ||
		strings.HasPrefix(content, "[Design Context]")
}

// ProgressMessagePrefix is a stable marker for progress summary context messages.
const ProgressMessagePrefix = "[[PROGRESS_CONTEXT_V1]]"

// FormatProgressSummary builds a brief progress summary for injection into the context.
// Returns empty string if there is nothing to report.
func FormatProgressSummary(version int, todos []TodoItemRecord) string {
	if version <= 0 && len(todos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(ProgressMessagePrefix + "\n")
	b.WriteString("## Progress\n")
	if version > 0 {
		b.WriteString(fmt.Sprintf("- Version: v%d\n", version))
	}
	if len(todos) > 0 {
		done := 0
		var inProgress string
		for _, t := range todos {
			if t.Status == "completed" {
				done++
			} else if t.Status == "in_progress" && inProgress == "" {
				inProgress = t.ActiveForm
			}
		}
		b.WriteString(fmt.Sprintf("- Tasks: %d/%d completed", done, len(todos)))
		if inProgress != "" {
			b.WriteString(fmt.Sprintf(" | Current: %s", inProgress))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// isProgressMessage returns true if the message content looks like a progress summary.
func isProgressMessage(content string) bool {
	return strings.HasPrefix(content, ProgressMessagePrefix)
}

// CompressMessagePrefix is the marker for recovery compression summaries.
// TrimMessages always preserves messages with this prefix.
const CompressMessagePrefix = "[[RECOVERY_COMPRESS_V1]]"

// isCompressMessage returns true if the message content is a recovery compression summary.
func isCompressMessage(content string) bool {
	return strings.HasPrefix(content, CompressMessagePrefix)
}
