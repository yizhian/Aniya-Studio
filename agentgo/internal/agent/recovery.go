package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"agentgo/internal/model"
	"agentgo/internal/retry"
)

// RecoveryState tracks recovery attempts within a single RunStreaming call.
// Persisted in LoopState (single source of truth); SessionState does NOT
// duplicate recovery counters.
type RecoveryState struct {
	ContinueCount int `json:"continue_count"`
	CompressCount int `json:"compress_count"`
	BackoffCount  int `json:"backoff_count"`

	LastErrorKind             string `json:"last_error_kind"`
	LastErrorMsg              string `json:"last_error_msg"`
	LastRecoveryTokenEstimate int    `json:"last_recovery_token_estimate"`

	Terminal bool `json:"terminal"`
}

// RecoveryAction classifies the recovery response.
type RecoveryAction int

const (
	RecoveryNone    RecoveryAction = iota
	RecoveryContinue
	RecoveryCompress
	RecoveryBackoff
)

const (
	MaxContinueRetries = 2
	MaxCompressRetries = 1
	MaxBackoffRetries  = 5
)

// RecoveryActionResult carries the result of classifying a recovery.
type RecoveryActionResult struct {
	Action RecoveryAction
	Error  error
}

// ClassifyRecovery maps an error, finish reason, and stream state to a recovery action.
// Decision tree (first match wins), mirrors §2b.
func ClassifyRecovery(err error, finishReason string, contentText string, toolCalls []model.ToolCall, hadAnyDelta bool, rs *RecoveryState, cb *CircuitBreaker) RecoveryAction {
	// Case 1: Truncation — finish_reason signals output was cut.
	if finishReason == "length" || finishReason == "max_tokens" {
		if rs.ContinueCount < MaxContinueRetries {
			return RecoveryContinue
		}
		return RecoveryNone
	}

	// Case 2: DeepSeek resource exhaustion — Backoff, not Continue.
	if finishReason == "insufficient_system_resource" {
		if rs.BackoffCount < MaxBackoffRetries && (cb == nil || cb.Allow()) {
			return RecoveryBackoff
		}
		return RecoveryNone
	}

	// Case 3: Ambiguous EOF — run heuristics.
	if finishReason == "" {
		if isTruncated(contentText, toolCalls, hadAnyDelta) {
			if rs.ContinueCount < MaxContinueRetries {
				return RecoveryContinue
			}
			return RecoveryNone
		}
		// Not truncated — fall through to error classification (cases 4–7).
		if err == nil {
			return RecoveryNone // genuine normal completion
		}
	}

	// Cases 4–7: Error-based classification.
	if err != nil {
		errStr := err.Error()

		// Case 4 context overflow patterns
		if containsAny(errStr, "context_length_exceeded", "too many tokens", "token limit", "context window") {
			if rs.CompressCount < MaxCompressRetries {
				return RecoveryCompress
			}
			return RecoveryNone
		}

		// Case 5 transient errors
		if isTransientError(err) {
			if rs.BackoffCount < MaxBackoffRetries && (cb == nil || cb.Allow()) {
				return RecoveryBackoff
			}
			return RecoveryNone
		}
	}

	return RecoveryNone
}

// isTruncated detects whether a stream response was likely truncated.
func isTruncated(contentText string, toolCalls []model.ToolCall, hadAnyDelta bool) bool {
	// Guard: empty response with no deltas is not truncation.
	if len(contentText) == 0 && len(toolCalls) == 0 && !hadAnyDelta {
		return false
	}
	// Stream started (deltas received) but produced nothing → truncated.
	if len(contentText) == 0 && len(toolCalls) == 0 && hadAnyDelta {
		return true
	}
	// 1. Any incomplete tool call?
	for _, tc := range toolCalls {
		if !json.Valid([]byte(tc.Function.Arguments)) {
			return true
		}
		if !hasMinimumFields(tc.Function.Name, tc.Function.Arguments) {
			return true
		}
	}
	// 2. Unclosed markdown fence?
	trimmed := strings.TrimSpace(contentText)
	if strings.Count(trimmed, "```")%2 != 0 {
		return true
	}
	return false
}

// hasMinimumFields checks that a tool call JSON has required fields for its name.
// Catches semantically-truncated-but-syntactically-valid JSON (e.g. write_file
// with missing "content" key).
func hasMinimumFields(toolName string, argsJSON string) bool {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return false
	}
	switch toolName {
	case "write_file":
		_, hasPath := args["path"]
		_, hasContent := args["content"]
		return hasPath && hasContent
	case "edit_file":
		_, hasPath := args["path"]
		_, hasOld := args["old_string"]
		_, hasNew := args["new_string"]
		return hasPath && hasOld && hasNew
	default:
		return true // unknown tools: don't second-guess
	}
}

// BuildContinuePrompt creates a continue message after truncation.
// Case A: no valid tool calls → ask to re-issue.
// Case B: some tool calls completed → acknowledge and continue.
func BuildContinuePrompt(contentText string, toolCalls []model.ToolCall) string {
	allValid := true
	for _, tc := range toolCalls {
		if !json.Valid([]byte(tc.Function.Arguments)) || !hasMinimumFields(tc.Function.Name, tc.Function.Arguments) {
			allValid = false
			break
		}
	}

	if len(toolCalls) == 0 || !allValid {
		return fmt.Sprintf(
			"Your previous response was truncated due to output length limits. "+
				"None of your tool calls completed successfully.\n\n"+
				"Please re-issue your write_file or edit_file calls with the complete content. "+
				"If the content is very large, consider splitting it across multiple smaller files "+
				"or using edit_file for incremental changes.\n\n"+
				"Previous partial text (if any):\n%s",
			truncateText(contentText, 500),
		)
	}

	// Case B: all tool calls valid — they were executed. Ask to continue.
	var b strings.Builder
	b.WriteString("Your previous response was truncated after completing some tool calls. ")
	b.WriteString("The following tool calls were executed successfully:\n")
	for _, tc := range toolCalls {
		fmt.Fprintf(&b, "- %s(%s)\n", tc.Function.Name, truncateText(tc.Function.Arguments, 200))
	}
	b.WriteString("\nPlease continue from where you left off.")
	return b.String()
}

// CompressMessagePrefix marks compression summary messages so TrimMessages
// preserves them across rounds.
const CompressMessagePrefix = "[[RECOVERY_COMPRESS_V1]]"

// BuildCompressPrompt creates a rule-based compression summary.
func BuildCompressPrompt(messages []model.Message, timeline []any) string {
	var b strings.Builder
	b.WriteString(CompressMessagePrefix)
	b.WriteString("\n[Compressed Context — previous rounds summarized]\n\n")

	// Extract key decisions and tool calls from message history.
	b.WriteString("## Key Decisions Made\n")
	decisionCount := 0
	for _, m := range messages {
		if m.Role == "assistant" && m.Content != "" {
			b.WriteString(summarizeContent(m.Content))
			decisionCount++
			if decisionCount >= 5 {
				break
			}
		}
	}

	b.WriteString("\n## Active State\n")
	b.WriteString("Continue from the most recent round. The full context was compressed ")
	b.WriteString("to stay within token limits. You have access to the last few rounds ")
	b.WriteString("of detailed history below.\n")

	return b.String()
}

// summarizeContent extracts a brief summary line from assistant content.
func summarizeContent(content string) string {
	// Take the first meaningful line (skip empty lines and markdown headers).
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(trimmed) > 200 {
			trimmed = trimmed[:200] + "..."
		}
		return "- " + trimmed + "\n"
	}
	return ""
}

// truncateText truncates text to maxLen characters.
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// isTransientError returns true for errors that warrant a backoff retry.
func isTransientError(err error) bool {
	// Check for typed retryable HTTP errors first (structured classification).
	var httpErr *retry.RetryableHTTPError
	if errors.As(err, &httpErr) {
		return retry.IsRetryableHTTPStatus(httpErr.Code)
	}

	// EOF on stream read is a transient network issue.
	if errors.Is(err, io.EOF) {
		return true
	}

	// Fall back to substring matching for errors from non-provider sources.
	errStr := err.Error()
	transient := []string{
		"429", "503", "500", "502", "504",
		"rate_limit", "rate limit",
		"timeout", "timed out",
		"temporary", "i/o timeout",
		"connection reset", "connection refused",
	}
	return containsAny(errStr, transient...)
}