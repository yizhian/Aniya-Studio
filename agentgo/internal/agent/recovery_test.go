package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"agentgo/internal/model"
)

// ============================================================================
// isTruncated tests
// ============================================================================

func TestIsTruncated_EmptyNoDelta(t *testing.T) {
	// Empty content, no tool calls, no deltas → not truncated (genuine empty reply).
	if isTruncated("", nil, false) {
		t.Error("expected false for empty response with no deltas")
	}
}

func TestIsTruncated_EmptyWithDeltas(t *testing.T) {
	// Empty content but deltas were received — could be truncated.
	if !isTruncated("", nil, true) {
		t.Error("expected true for empty response that had deltas")
	}
}

func TestIsTruncated_ValidJSON_Complete(t *testing.T) {
	toolCalls := []model.ToolCall{
		{Function: model.ToolCallFunction{
			Name:      "write_file",
			Arguments: `{"path":"index.html","content":"<html></html>"}`,
		}},
	}
	if isTruncated("", toolCalls, true) {
		t.Error("expected false for complete valid tool call")
	}
}

func TestIsTruncated_BrokenJSON(t *testing.T) {
	// Arguments is truncated mid-string.
	toolCalls := []model.ToolCall{
		{Function: model.ToolCallFunction{
			Name:      "write_file",
			Arguments: `{"path":"index.html","content":"<html>...`,
		}},
	}
	if !isTruncated("", toolCalls, true) {
		t.Error("expected true for broken JSON in tool call")
	}
}

func TestIsTruncated_SemanticallyTruncated(t *testing.T) {
	// Valid JSON but missing required content field.
	toolCalls := []model.ToolCall{
		{Function: model.ToolCallFunction{
			Name:      "write_file",
			Arguments: `{"path":"index.html"}`,
		}},
	}
	if !isTruncated("", toolCalls, true) {
		t.Error("expected true for semantically truncated tool call (missing content)")
	}
}

func TestIsTruncated_UnclosedFence(t *testing.T) {
	if !isTruncated("```json\n{\"key\":\"value\"}\n", nil, true) {
		t.Error("expected true for content with unclosed markdown fence")
	}
}

func TestIsTruncated_ClosedFence(t *testing.T) {
	content := "```json\n{\"key\":\"value\"}\n```"
	if isTruncated(content, nil, true) {
		t.Error("expected false for content with properly closed fence")
	}
}

func TestIsTruncated_MultipleToolCalls_OneBroken(t *testing.T) {
	toolCalls := []model.ToolCall{
		{Function: model.ToolCallFunction{
			Name:      "read_file",
			Arguments: `{"path":"x.html"}`,
		}},
		{Function: model.ToolCallFunction{
			Name:      "write_file",
			Arguments: `{"path":"y.html","content":"broken...`,
		}},
	}
	if !isTruncated("", toolCalls, true) {
		t.Error("expected true when one of multiple tool calls is broken")
	}
}

func TestIsTruncated_NoDelta_ButHasContent(t *testing.T) {
	// Content present but no deltas (impossible in practice — deltas must precede content).
	// Without deltas or tool calls, plain text content is not evidence of truncation.
	if isTruncated("some text", nil, false) {
		t.Error("expected false: content without deltas is not evidence of truncation")
	}
}

// ============================================================================
// hasMinimumFields tests
// ============================================================================

func TestHasMinimumFields_WriteFile_Complete(t *testing.T) {
	if !hasMinimumFields("write_file", `{"path":"x.html","content":"hello"}`) {
		t.Error("expected true for complete write_file args")
	}
}

func TestHasMinimumFields_WriteFile_MissingContent(t *testing.T) {
	if hasMinimumFields("write_file", `{"path":"x.html"}`) {
		t.Error("expected false for write_file missing content")
	}
}

func TestHasMinimumFields_WriteFile_MissingPath(t *testing.T) {
	if hasMinimumFields("write_file", `{"content":"hello"}`) {
		t.Error("expected false for write_file missing path")
	}
}

func TestHasMinimumFields_EditFile_Complete(t *testing.T) {
	if !hasMinimumFields("edit_file", `{"path":"x.html","old_string":"a","new_string":"b"}`) {
		t.Error("expected true for complete edit_file args")
	}
}

func TestHasMinimumFields_EditFile_MissingOldString(t *testing.T) {
	if hasMinimumFields("edit_file", `{"path":"x.html","new_string":"b"}`) {
		t.Error("expected false for edit_file missing old_string")
	}
}

func TestHasMinimumFields_UnknownTool(t *testing.T) {
	if !hasMinimumFields("read_file", `{}`) {
		t.Error("expected true for unknown tool (don't second-guess)")
	}
}

func TestHasMinimumFields_InvalidJSON(t *testing.T) {
	if hasMinimumFields("write_file", `{broken`) {
		t.Error("expected false for invalid JSON")
	}
}

// ============================================================================
// ClassifyRecovery tests
// ============================================================================

func TestClassifyRecovery_Case1_Length(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(nil, "length", "", nil, false, rs, nil)
	if action != RecoveryContinue {
		t.Errorf("expected RecoveryContinue for finish_reason=length, got %d", action)
	}
}

func TestClassifyRecovery_Case1_MaxTokens(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(nil, "max_tokens", "", nil, false, rs, nil)
	if action != RecoveryContinue {
		t.Errorf("expected RecoveryContinue for finish_reason=max_tokens, got %d", action)
	}
}

func TestClassifyRecovery_Case1_Exhausted(t *testing.T) {
	rs := &RecoveryState{ContinueCount: MaxContinueRetries}
	action := ClassifyRecovery(nil, "length", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone when continue retries exhausted, got %d", action)
	}
}

func TestClassifyRecovery_Case2_InsufficientResource(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(nil, "insufficient_system_resource", "", nil, false, rs, nil)
	if action != RecoveryBackoff {
		t.Errorf("expected RecoveryBackoff for insufficient_system_resource, got %d", action)
	}
}

func TestClassifyRecovery_Case2_Exhausted(t *testing.T) {
	rs := &RecoveryState{BackoffCount: MaxBackoffRetries}
	action := ClassifyRecovery(nil, "insufficient_system_resource", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone when backoff retries exhausted, got %d", action)
	}
}

func TestClassifyRecovery_Case2_CircuitOpen(t *testing.T) {
	rs := &RecoveryState{}
	cb := NewCircuitBreaker()
	cb.tripThreshold = 1
	cb.RecordFailure() // trip
	action := ClassifyRecovery(nil, "insufficient_system_resource", "", nil, false, rs, cb)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone when circuit is open, got %d", action)
	}
}

func TestClassifyRecovery_Case3_AmbiguousEOF_Truncated(t *testing.T) {
	rs := &RecoveryState{}
	toolCalls := []model.ToolCall{
		{Function: model.ToolCallFunction{
			Name:      "write_file",
			Arguments: `{"path":"x.html","content":"broken...`,
		}},
	}
	action := ClassifyRecovery(nil, "", "", toolCalls, true, rs, nil)
	if action != RecoveryContinue {
		t.Errorf("expected RecoveryContinue for truncated ambiguous EOF, got %d", action)
	}
}

func TestClassifyRecovery_Case3_AmbiguousEOF_Normal(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(nil, "", "normal completion text", nil, true, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone for normal ambiguous EOF, got %d", action)
	}
}

func TestClassifyRecovery_Case3_AmbiguousEOF_EmptyNoDelta(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(nil, "", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone for empty with no deltas, got %d", action)
	}
}

func TestClassifyRecovery_Case4_ContextOverflow(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("context_length_exceeded: too many tokens"), "stop", "", nil, false, rs, nil)
	if action != RecoveryCompress {
		t.Errorf("expected RecoveryCompress for context overflow, got %d", action)
	}
}

func TestClassifyRecovery_Case4_ContextOverflow_Exhausted(t *testing.T) {
	rs := &RecoveryState{CompressCount: MaxCompressRetries}
	action := ClassifyRecovery(errors.New("token limit exceeded"), "stop", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone when compress retries exhausted, got %d", action)
	}
}

func TestClassifyRecovery_Case5_Transient429(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("HTTP 429 Too Many Requests"), "stop", "", nil, false, rs, nil)
	if action != RecoveryBackoff {
		t.Errorf("expected RecoveryBackoff for 429, got %d", action)
	}
}

func TestClassifyRecovery_Case5_Transient503(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("service returned 503"), "stop", "", nil, false, rs, nil)
	if action != RecoveryBackoff {
		t.Errorf("expected RecoveryBackoff for 503, got %d", action)
	}
}

func TestClassifyRecovery_Case5_Transient500(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("internal server error 500"), "stop", "", nil, false, rs, nil)
	if action != RecoveryBackoff {
		t.Errorf("expected RecoveryBackoff for 500, got %d", action)
	}
}

func TestClassifyRecovery_Case5_Transient502(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("502 Bad Gateway"), "stop", "", nil, false, rs, nil)
	if action != RecoveryBackoff {
		t.Errorf("expected RecoveryBackoff for 502, got %d", action)
	}
}

func TestClassifyRecovery_Case5_Transient504(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("504 Gateway Timeout"), "stop", "", nil, false, rs, nil)
	if action != RecoveryBackoff {
		t.Errorf("expected RecoveryBackoff for 504, got %d", action)
	}
}

func TestClassifyRecovery_Case5_TransientTimeout(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("request timed out"), "stop", "", nil, false, rs, nil)
	if action != RecoveryBackoff {
		t.Errorf("expected RecoveryBackoff for timeout, got %d", action)
	}
}

func TestClassifyRecovery_Case5_TransientRateLimit(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("rate_limit exceeded"), "stop", "", nil, false, rs, nil)
	if action != RecoveryBackoff {
		t.Errorf("expected RecoveryBackoff for rate_limit, got %d", action)
	}
}

func TestClassifyRecovery_Case5_TransientConnectionReset(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("connection reset by peer"), "stop", "", nil, false, rs, nil)
	if action != RecoveryBackoff {
		t.Errorf("expected RecoveryBackoff for connection reset, got %d", action)
	}
}

func TestClassifyRecovery_Case5_TransientEOF(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(io.EOF, "stop", "", nil, false, rs, nil)
	if action != RecoveryBackoff {
		t.Errorf("expected RecoveryBackoff for EOF, got %d", action)
	}
}

func TestClassifyRecovery_Case5_TransientExhausted(t *testing.T) {
	rs := &RecoveryState{BackoffCount: MaxBackoffRetries}
	action := ClassifyRecovery(errors.New("HTTP 429"), "stop", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone when backoff exhausted for transient, got %d", action)
	}
}

func TestClassifyRecovery_Case5_TransientCircuitOpen(t *testing.T) {
	rs := &RecoveryState{}
	cb := NewCircuitBreaker()
	cb.tripThreshold = 1
	cb.RecordFailure()
	action := ClassifyRecovery(errors.New("HTTP 429"), "stop", "", nil, false, rs, cb)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone when circuit open for transient, got %d", action)
	}
}

func TestClassifyRecovery_Case6_NonRetryable401(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("401 Unauthorized"), "stop", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone for 401, got %d", action)
	}
}

func TestClassifyRecovery_Case6_NonRetryable403(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("403 Forbidden"), "stop", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone for 403, got %d", action)
	}
}

func TestClassifyRecovery_Case6_InvalidAPIKey(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(errors.New("invalid_api_key"), "stop", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone for invalid_api_key, got %d", action)
	}
}

func TestClassifyRecovery_Case7_Fallback(t *testing.T) {
	// finish_reason="stop" with no error → terminal (normal completion).
	rs := &RecoveryState{}
	action := ClassifyRecovery(nil, "stop", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone for clean stop, got %d", action)
	}
}

func TestClassifyRecovery_ToolCalls(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(nil, "tool_calls", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone for tool_calls finish reason, got %d", action)
	}
}

func TestClassifyRecovery_EndTurn(t *testing.T) {
	rs := &RecoveryState{}
	action := ClassifyRecovery(nil, "end_turn", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone for end_turn, got %d", action)
	}
}

// ============================================================================
// BuildContinuePrompt tests
// ============================================================================

func TestBuildContinuePrompt_CaseA_NoToolCalls(t *testing.T) {
	prompt := BuildContinuePrompt("some partial text", nil)
	if !strings.Contains(prompt, "None of your tool calls completed") {
		t.Errorf("expected Case A message, got: %s", prompt)
	}
	if !strings.Contains(prompt, "partial text") {
		t.Errorf("expected truncated partial text in prompt, got: %s", prompt)
	}
}

func TestBuildContinuePrompt_CaseA_BrokenToolCalls(t *testing.T) {
	toolCalls := []model.ToolCall{
		{Function: model.ToolCallFunction{
			Name:      "write_file",
			Arguments: `{"path":"x.html","content":"broken...`,
		}},
	}
	prompt := BuildContinuePrompt("", toolCalls)
	if !strings.Contains(prompt, "None of your tool calls completed") {
		t.Errorf("expected Case A message for broken tool calls, got: %s", prompt)
	}
}

func TestBuildContinuePrompt_CaseB_ValidToolCalls(t *testing.T) {
	toolCalls := []model.ToolCall{
		{Function: model.ToolCallFunction{
			Name:      "write_file",
			Arguments: `{"path":"index.html","content":"<html></html>"}`,
		}},
	}
	prompt := BuildContinuePrompt("", toolCalls)
	if !strings.Contains(prompt, "executed successfully") {
		t.Errorf("expected Case B message for valid tool calls, got: %s", prompt)
	}
	if !strings.Contains(prompt, "write_file") {
		t.Errorf("expected tool name in Case B prompt, got: %s", prompt)
	}
}

func TestBuildContinuePrompt_LongContentTruncated(t *testing.T) {
	longText := strings.Repeat("abcdefghij", 100) // 1000 chars
	prompt := BuildContinuePrompt(longText, nil)
	if len(prompt) > 2000 {
		t.Errorf("prompt with long content should be reasonably sized, got %d chars", len(prompt))
	}
}

// ============================================================================
// BuildCompressPrompt tests
// ============================================================================

func TestBuildCompressPrompt_Empty(t *testing.T) {
	prompt := BuildCompressPrompt(nil, nil)
	if !strings.Contains(prompt, CompressMessagePrefix) {
		t.Errorf("expected CompressMessagePrefix in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "Compressed Context") {
		t.Errorf("expected 'Compressed Context' header, got: %s", prompt)
	}
}

func TestBuildCompressPrompt_WithMessages(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Create a slide deck."},
		{Role: "assistant", Content: "I'll create a 5-slide presentation on AI trends.\nLet me start by loading the compliance skill."},
		{Role: "assistant", Content: "Now I'll read the design template."},
	}
	prompt := BuildCompressPrompt(messages, nil)
	if !strings.Contains(prompt, "Key Decisions Made") {
		t.Errorf("expected 'Key Decisions Made' section")
	}
	if !strings.Contains(prompt, "Active State") {
		t.Errorf("expected 'Active State' section")
	}
}

func TestBuildCompressPrompt_MaxDecisions(t *testing.T) {
	// More than 5 assistant messages — only first 5 should be summarized.
	var messages []model.Message
	for i := 0; i < 10; i++ {
		messages = append(messages, model.Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Decision %d: do something important here.", i),
		})
	}
	prompt := BuildCompressPrompt(messages, nil)
	decisionCount := strings.Count(prompt, "- Decision")
	if decisionCount > 5 {
		t.Errorf("expected at most 5 decision summaries, got %d", decisionCount)
	}
}

// ============================================================================
// Helper function tests
// ============================================================================

func TestSummarizeContent_Normal(t *testing.T) {
	result := summarizeContent("This is the first meaningful line.\nSecond line here.")
	if !strings.Contains(result, "This is the first meaningful line") {
		t.Errorf("expected first line in summary, got: %s", result)
	}
}

func TestSummarizeContent_SkipsEmptyAndHeaders(t *testing.T) {
	result := summarizeContent("\n# Header\n\nActual content starts here.\nMore text.")
	if !strings.Contains(result, "Actual content starts here") {
		t.Errorf("expected content after skipping empty lines and headers, got: %s", result)
	}
}

func TestSummarizeContent_LongLine(t *testing.T) {
	longLine := strings.Repeat("x", 300)
	result := summarizeContent(longLine)
	if len(result) > 210 { // "- " + 200 chars + "..." + "\n"
		t.Errorf("expected truncated long line, got %d chars", len(result))
	}
	if !strings.HasSuffix(strings.TrimSpace(result), "...") {
		t.Errorf("expected truncation marker, got: %s", result)
	}
}

func TestSummarizeContent_AllEmpty(t *testing.T) {
	result := summarizeContent("\n\n  \n")
	if result != "" {
		t.Errorf("expected empty result for all-empty content, got: %s", result)
	}
}

func TestTruncateText_WithinLimit(t *testing.T) {
	result := truncateText("hello", 10)
	if result != "hello" {
		t.Errorf("expected unchanged text, got: %s", result)
	}
}

func TestTruncateText_OverLimit(t *testing.T) {
	result := truncateText("hello world", 5)
	if result != "hello..." {
		t.Errorf("expected truncated text, got: %s", result)
	}
}

func TestContainsAny_Match(t *testing.T) {
	if !containsAny("HTTP 429 Too Many Requests", "429", "503") {
		t.Error("expected match for 429")
	}
}

func TestContainsAny_NoMatch(t *testing.T) {
	if containsAny("HTTP 200 OK", "429", "503") {
		t.Error("expected no match for 200")
	}
}

func TestContainsAny_Empty(t *testing.T) {
	if containsAny("", "429") {
		t.Error("expected no match for empty string")
	}
}

func TestIsTransientError_AllPatterns(t *testing.T) {
	transientTests := []string{
		"HTTP 429",
		"503 Service Unavailable",
		"500 Internal Server Error",
		"502 Bad Gateway",
		"504 Gateway Timeout",
		"rate_limit exceeded",
		"rate limit hit",
		"request timeout",
		"connection timed out",
		"temporary failure",
		"i/o timeout",
		"connection reset",
		"connection refused",
	}
	for _, tc := range transientTests {
		if !isTransientError(fmt.Errorf("%s", tc)) {
			t.Errorf("expected transient: %q", tc)
		}
	}
	// io.EOF is detected via errors.Is, not substring matching.
	if !isTransientError(io.EOF) {
		t.Error("expected transient for io.EOF")
	}
}

func TestIsTransientError_NonTransient(t *testing.T) {
	nonTransient := []string{
		"401 Unauthorized",
		"403 Forbidden",
		"invalid_api_key",
		"not found",
		"bad request",
	}
	for _, tc := range nonTransient {
		if isTransientError(fmt.Errorf("%s", tc)) {
			t.Errorf("expected non-transient: %q", tc)
		}
	}
}

// ============================================================================
// RecoveryState exhaustion tests (SIT)
// ============================================================================

func TestRecoveryState_ContinueExhaustion(t *testing.T) {
	rs := &RecoveryState{ContinueCount: MaxContinueRetries}
	action := ClassifyRecovery(nil, "length", "truncated text", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone when continue exhausted, got %d", action)
	}
	if rs.ContinueCount != MaxContinueRetries {
		t.Errorf("expected ContinueCount to not change, got %d", rs.ContinueCount)
	}
}

func TestRecoveryState_CompressExhaustion(t *testing.T) {
	rs := &RecoveryState{CompressCount: MaxCompressRetries}
	action := ClassifyRecovery(errors.New("context_length_exceeded"), "stop", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone when compress exhausted, got %d", action)
	}
}

func TestRecoveryState_BackoffExhaustion_Transient(t *testing.T) {
	rs := &RecoveryState{BackoffCount: MaxBackoffRetries}
	action := ClassifyRecovery(errors.New("HTTP 429"), "stop", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone when backoff exhausted for transient, got %d", action)
	}
}

func TestRecoveryState_BackoffExhaustion_Resource(t *testing.T) {
	rs := &RecoveryState{BackoffCount: MaxBackoffRetries}
	action := ClassifyRecovery(nil, "insufficient_system_resource", "", nil, false, rs, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone when backoff exhausted for resource, got %d", action)
	}
}

// ============================================================================
// SIT: Full recovery pipeline simulation
// ============================================================================

func TestRecovery_SIT_FullContinuePipeline(t *testing.T) {
	// Simulate: model returns truncated output with finish_reason=length.
	// → Continue prompt injected → model completes normally.
	rs := &RecoveryState{}

	// Attempt 1: length → Continue.
	action1 := ClassifyRecovery(nil, "length", "partial text", nil, true, rs, nil)
	if action1 != RecoveryContinue {
		t.Fatalf("expected Continue on first length, got %d", action1)
	}
	rs.ContinueCount++

	// Attempt 2: length again → Continue.
	action2 := ClassifyRecovery(nil, "length", "more partial text", nil, true, rs, nil)
	if action2 != RecoveryContinue {
		t.Fatalf("expected Continue on second length, got %d", action2)
	}
	rs.ContinueCount++

	// Attempt 3: length → exhausted.
	action3 := ClassifyRecovery(nil, "length", "still partial", nil, true, rs, nil)
	if action3 != RecoveryNone {
		t.Fatalf("expected None on third length (exhausted), got %d", action3)
	}
}

func TestRecovery_SIT_MixedRecovery(t *testing.T) {
	// Simulate: transient error → Backoff → then length → Continue → then context overflow → Compress.
	rs := &RecoveryState{}
	cb := NewCircuitBreaker()

	// Step 1: 429 → Backoff.
	a1 := ClassifyRecovery(errors.New("HTTP 429"), "", "", nil, false, rs, cb)
	if a1 != RecoveryBackoff {
		t.Fatalf("step 1: expected Backoff, got %d", a1)
	}
	rs.BackoffCount++
	cb.RecordFailure()

	// Step 2: length → Continue.
	a2 := ClassifyRecovery(nil, "length", "", nil, true, rs, cb)
	if a2 != RecoveryContinue {
		t.Fatalf("step 2: expected Continue, got %d", a2)
	}
	rs.ContinueCount++
	cb.RecordSuccess()

	// Step 3: context overflow → Compress.
	a3 := ClassifyRecovery(errors.New("context_length_exceeded"), "stop", "", nil, false, rs, cb)
	if a3 != RecoveryCompress {
		t.Fatalf("step 3: expected Compress, got %d", a3)
	}
	rs.CompressCount++

	// Verify final state.
	if rs.BackoffCount != 1 {
		t.Errorf("expected BackoffCount 1, got %d", rs.BackoffCount)
	}
	if rs.ContinueCount != 1 {
		t.Errorf("expected ContinueCount 1, got %d", rs.ContinueCount)
	}
	if rs.CompressCount != 1 {
		t.Errorf("expected CompressCount 1, got %d", rs.CompressCount)
	}
}

func TestRecovery_SIT_TerminalNonRetryable(t *testing.T) {
	// 401 should never trigger recovery.
	action := ClassifyRecovery(errors.New("401 Unauthorized: invalid API key"), "", "", nil, false, &RecoveryState{}, nil)
	if action != RecoveryNone {
		t.Errorf("expected RecoveryNone (terminal) for 401, got %d", action)
	}
}

// Ensure context import is used.
var _ = context.Background