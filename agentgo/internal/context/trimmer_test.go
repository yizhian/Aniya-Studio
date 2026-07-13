package context

import (
	"strings"
	"testing"

	"agentgo/internal/model"
)

func makeMsg(role, content string) model.Message {
	return model.Message{Role: role, Content: content}
}

func TestTrimMessages_PreservesSystem(t *testing.T) {
	msgs := []model.Message{
		makeMsg("system", "You are a helpful assistant."),
		makeMsg("user", "Hello."),
		makeMsg("assistant", "Hi there!"),
	}
	result := TrimMessages(msgs, 2, nil)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	if result[0].Role != "system" {
		t.Fatalf("expected first message to be system, got %q", result[0].Role)
	}
}

func TestTrimMessages_InjectsSnapshot(t *testing.T) {
	msgs := []model.Message{
		makeMsg("system", "System prompt."),
		makeMsg("user", "Build a deck."),
		makeMsg("assistant", "Working."),
	}
	snap := &DesignSnapshot{SlideCount: 3, Title: "My Deck", SlideHeadings: []string{"Slide 1", "Slide 2"}}
	result := TrimMessages(msgs, 2, snap)

	found := false
	for _, m := range result {
		if m.Role == "user" && strings.HasPrefix(m.Content, SnapshotMessagePrefix) {
			found = true
			if !strings.Contains(m.Content, "My Deck") {
				t.Fatalf("expected snapshot to contain title 'My Deck', got: %s", m.Content)
			}
			if !strings.Contains(m.Content, "3") {
				t.Fatalf("expected snapshot to mention slide count 3, got: %s", m.Content)
			}
		}
	}
	if !found {
		t.Fatal("expected snapshot message in output")
	}
}

func TestTrimMessages_NilSnapshot(t *testing.T) {
	msgs := []model.Message{
		makeMsg("system", "System."),
		makeMsg("user", "Hello."),
		makeMsg("assistant", "Hi."),
	}
	result := TrimMessages(msgs, 2, nil)
	for _, m := range result {
		if m.Role == "user" && strings.HasPrefix(m.Content, SnapshotMessagePrefix) {
			t.Fatal("did not expect snapshot message when snapshot is nil")
		}
	}
}

func TestTrimMessages_ZeroSlideCountSnapshot(t *testing.T) {
	msgs := []model.Message{
		makeMsg("system", "System."),
		makeMsg("user", "Hello."),
		makeMsg("assistant", "Hi."),
	}
	snap := &DesignSnapshot{SlideCount: 0, Title: "Empty Deck"}
	result := TrimMessages(msgs, 2, snap)
	for _, m := range result {
		if m.Role == "user" && strings.HasPrefix(m.Content, SnapshotMessagePrefix) {
			t.Fatal("did not expect snapshot message for zero-slide snapshot")
		}
	}
}

func TestTrimMessages_KeepsLastNRounds(t *testing.T) {
	// Build: system + R1(user, asst, tool, asst) + R2(user, asst) + R3(user, asst)
	msgs := []model.Message{
		makeMsg("system", "System."),
		makeMsg("user", "Q1"),
		makeMsg("assistant", "A1 tool."),
		makeMsg("tool", "tool result 1"),
		makeMsg("assistant", "A1 final."),
		makeMsg("user", "Q2"),
		makeMsg("assistant", "A2."),
		makeMsg("user", "Q3"),
		makeMsg("assistant", "A3."),
	}

	result := TrimMessages(msgs, 2, nil)

	// 4 assistant messages = 4 "rounds". keepRounds=2 means keep last 2.
	// Index of 3rd assistant (0-based: 2nd in roundStarts slice) = roundStarts[2] = 6.
	// Result keeps from index 6 onward: A2, Q3, A3.
	// Also includes system prompt. Total = 4 messages.

	// Q1 should be trimmed.
	for _, m := range result {
		if m.Content == "Q1" {
			t.Fatal("expected Q1 to be trimmed")
		}
		if m.Content == "A1 tool." || m.Content == "A1 final." || m.Content == "tool result 1" {
			t.Fatal("expected round 1 messages to be trimmed")
		}
	}

	// Q2 is before walkStart (index 5 < walkStart 6), so also trimmed.
	for _, m := range result {
		if m.Content == "Q2" {
			t.Fatal("expected Q2 to be trimmed (it precedes the first kept assistant)")
		}
	}

	// Q3 and A2 and A3 should be present.
	foundQ3 := false
	foundA2 := false
	foundA3 := false
	for _, m := range result {
		if m.Content == "Q3" {
			foundQ3 = true
		}
		if m.Content == "A2." {
			foundA2 = true
		}
		if m.Content == "A3." {
			foundA3 = true
		}
	}
	if !foundQ3 {
		t.Fatal("expected Q3 in output")
	}
	if !foundA2 {
		t.Fatal("expected A2 in output")
	}
	if !foundA3 {
		t.Fatal("expected A3 in output")
	}

	if result[0].Role != "system" {
		t.Fatalf("expected system message first, got %q", result[0].Role)
	}
}

func TestTrimMessages_StripsOldSnapshots(t *testing.T) {
	msgs := []model.Message{
		makeMsg("system", "System."),
		makeMsg("user", SnapshotMessagePrefix+"\nold snapshot"),
		makeMsg("user", "Actual request."),
		makeMsg("assistant", "Response."),
	}
	result := TrimMessages(msgs, 2, nil)

	// The old snapshot user message should be stripped.
	for _, m := range result {
		if strings.HasPrefix(m.Content, SnapshotMessagePrefix) {
			t.Fatal("old snapshot message should have been stripped")
		}
	}
	// But actual request should remain.
	found := false
	for _, m := range result {
		if m.Content == "Actual request." {
			found = true
		}
	}
	if !found {
		t.Fatal("expected actual request message to be preserved")
	}
}

func TestTrimMessages_LegacySnapshotPrefix(t *testing.T) {
	msgs := []model.Message{
		makeMsg("system", "System."),
		makeMsg("user", "[Design Context]\nold design context"),
		makeMsg("user", "Real question."),
		makeMsg("assistant", "Answer."),
	}
	result := TrimMessages(msgs, 2, nil)

	for _, m := range result {
		if m.Role == "user" && strings.HasPrefix(m.Content, "[Design Context]") {
			t.Fatal("legacy [Design Context] snapshot should have been stripped")
		}
	}
}

func TestTrimMessages_KeepRoundsZero(t *testing.T) {
	// keepRounds <= 0 defaults to KeepRounds (2).
	msgs := []model.Message{
		makeMsg("system", "System."),
		makeMsg("user", "Hello."),
		makeMsg("assistant", "Hi."),
	}
	result := TrimMessages(msgs, 0, nil)
	if len(result) == 0 {
		t.Fatal("expected non-empty result with keepRounds=0 (defaults to 2)")
	}
}

func TestTrimMessages_StripsBothSnapshotAndProgress(t *testing.T) {
	msgs := []model.Message{
		makeMsg("system", "System."),
		makeMsg("user", SnapshotMessagePrefix+"\nold snapshot"),
		makeMsg("user", ProgressMessagePrefix+"\nold progress"),
		makeMsg("user", "Actual request."),
		makeMsg("assistant", "Response."),
	}
	result := TrimMessages(msgs, 2, nil)

	for _, m := range result {
		if strings.HasPrefix(m.Content, SnapshotMessagePrefix) {
			t.Fatal("old snapshot message should have been stripped")
		}
		if strings.HasPrefix(m.Content, ProgressMessagePrefix) {
			t.Fatal("old progress message should have been stripped")
		}
	}
}

func TestTrimMessages_StripsMultiRoundProgress(t *testing.T) {
	// Simulate 3 rounds with progress injected each time.
	msgs := []model.Message{
		makeMsg("system", "System."),
		makeMsg("user", ProgressMessagePrefix+"\nprogress r1"),
		makeMsg("user", "Q1"),
		makeMsg("assistant", "A1."),
		makeMsg("user", ProgressMessagePrefix+"\nprogress r2"),
		makeMsg("user", "Q2"),
		makeMsg("assistant", "A2."),
		makeMsg("user", ProgressMessagePrefix+"\nprogress r3"),
		makeMsg("user", "Q3"),
		makeMsg("assistant", "A3."),
	}
	result := TrimMessages(msgs, 2, nil)

	count := 0
	for _, m := range result {
		if strings.HasPrefix(m.Content, ProgressMessagePrefix) {
			count++
		}
	}
	if count > 0 {
		t.Fatalf("all old progress messages should be stripped, found %d", count)
	}
}

func TestTrimMessages_ProgressBeforeSystem(t *testing.T) {
	// Edge case: progress message appears before system prompt.
	msgs := []model.Message{
		makeMsg("user", ProgressMessagePrefix+"\nrogue progress"),
		makeMsg("system", "System."),
		makeMsg("user", "Hello."),
		makeMsg("assistant", "Hi."),
	}
	result := TrimMessages(msgs, 2, nil)
	for _, m := range result {
		if strings.HasPrefix(m.Content, ProgressMessagePrefix) {
			t.Fatal("progress message should be stripped even if before system")
		}
	}
}

func TestTrimMessages_SystemOnly(t *testing.T) {
	msgs := []model.Message{
		makeMsg("system", "System prompt only."),
	}
	result := TrimMessages(msgs, 2, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Fatalf("expected system message, got %q", result[0].Role)
	}
}

func TestFormatProgressSummary_Empty(t *testing.T) {
	if got := FormatProgressSummary(0, nil); got != "" {
		t.Errorf("expected empty for no version and no todos, got %q", got)
	}
	if got := FormatProgressSummary(0, []TodoItemRecord{}); got != "" {
		t.Errorf("expected empty for zero version and empty todos, got %q", got)
	}
}

func TestFormatProgressSummary_VersionOnly(t *testing.T) {
	result := FormatProgressSummary(3, nil)
	if !strings.Contains(result, ProgressMessagePrefix) {
		t.Error("missing progress prefix")
	}
	if !strings.Contains(result, "Version: v3") {
		t.Error("missing version info")
	}
	if strings.Contains(result, "Tasks") {
		t.Error("should not contain tasks when no todos")
	}
}

func TestFormatProgressSummary_TodosOnly(t *testing.T) {
	todos := []TodoItemRecord{
		{Content: "Build slides", Status: "completed", ActiveForm: "Building slides"},
		{Content: "Add navigation", Status: "in_progress", ActiveForm: "Adding navigation"},
		{Content: "Verify", Status: "pending", ActiveForm: "Verifying"},
	}
	result := FormatProgressSummary(0, todos)
	if !strings.Contains(result, "1/3 completed") {
		t.Error("expected 1/3 completed")
	}
	if !strings.Contains(result, "Adding navigation") {
		t.Error("expected current task in progress")
	}
}

func TestFormatProgressSummary_Full(t *testing.T) {
	todos := []TodoItemRecord{
		{Content: "Build slides", Status: "completed", ActiveForm: "Building slides"},
		{Content: "Add navigation", Status: "completed", ActiveForm: "Adding navigation"},
	}
	result := FormatProgressSummary(5, todos)
	if !strings.Contains(result, "Version: v5") {
		t.Error("missing version")
	}
	if !strings.Contains(result, "2/2 completed") {
		t.Error("expected 2/2 completed")
	}
}

func TestFormatProgressSummary_AllCompleted(t *testing.T) {
	todos := []TodoItemRecord{
		{Content: "Task A", Status: "completed", ActiveForm: "Doing A"},
		{Content: "Task B", Status: "completed", ActiveForm: "Doing B"},
		{Content: "Task C", Status: "completed", ActiveForm: "Doing C"},
	}
	result := FormatProgressSummary(0, todos)
	if !strings.Contains(result, "3/3 completed") {
		t.Error("expected 3/3 completed")
	}
	// All completed means no in_progress → no "Current:".
	if strings.Contains(result, "Current:") {
		t.Error("should not show 'Current:' when all tasks completed")
	}
}

func TestFormatProgressSummary_OnlyPending(t *testing.T) {
	todos := []TodoItemRecord{
		{Content: "Task A", Status: "pending", ActiveForm: "Doing A"},
		{Content: "Task B", Status: "pending", ActiveForm: "Doing B"},
	}
	result := FormatProgressSummary(0, todos)
	if !strings.Contains(result, "0/2 completed") {
		t.Error("expected 0/2 completed")
	}
	if strings.Contains(result, "Current:") {
		t.Error("should not show 'Current:' when no task in progress")
	}
}

func TestFormatProgressSummary_MultipleInProgress(t *testing.T) {
	// Only the first in_progress task is shown.
	todos := []TodoItemRecord{
		{Content: "Task A", Status: "completed", ActiveForm: "Doing A"},
		{Content: "Task B", Status: "in_progress", ActiveForm: "First active"},
		{Content: "Task C", Status: "in_progress", ActiveForm: "Second active"},
	}
	result := FormatProgressSummary(2, todos)
	if !strings.Contains(result, "First active") {
		t.Error("should show the first in_progress task")
	}
	// Should contain first but we don't assert exclusion of second (implementation detail).
}

func TestFormatProgressSummary_VeryLongActiveForm(t *testing.T) {
	longName := "Building a very complex slide with many charts and diagrams and interactive elements"
	todos := []TodoItemRecord{
		{Content: "Task", Status: "in_progress", ActiveForm: longName},
	}
	result := FormatProgressSummary(1, todos)
	if !strings.Contains(result, longName) {
		t.Error("should contain the full active form name")
	}
	if !strings.Contains(result, "Current:") {
		t.Error("should show 'Current:' for in-progress task")
	}
}

func TestFormatProgressSummary_EmptyActiveForm(t *testing.T) {
	todos := []TodoItemRecord{
		{Content: "Empty form", Status: "in_progress", ActiveForm: ""},
	}
	result := FormatProgressSummary(0, todos)
	if !strings.Contains(result, "0/1 completed") {
		t.Error("expected 0/1 completed")
	}
	// Empty ActiveForm should result in no Current: display.
	if strings.Contains(result, "Current:") {
		t.Error("should not show 'Current:' when activeForm is empty")
	}
}

func TestFormatProgressSummary_VersionZeroWithTodos(t *testing.T) {
	// Version 0 means no snapshot yet — but todos may exist.
	todos := []TodoItemRecord{
		{Content: "Task", Status: "in_progress", ActiveForm: "Working"},
	}
	result := FormatProgressSummary(0, todos)
	if result == "" {
		t.Error("should not be empty when todos exist even with version 0")
	}
	if strings.Contains(result, "Version:") {
		t.Error("should not include version when version is 0")
	}
	if !strings.Contains(result, "0/1 completed") {
		t.Error("should include todo progress")
	}
}

func TestTrimMessages_StripsOldProgressMessages(t *testing.T) {
	msgs := []model.Message{
		makeMsg("system", "System."),
		makeMsg("user", ProgressMessagePrefix+"\nold progress"),
		makeMsg("user", "Actual request."),
		makeMsg("assistant", "Response."),
	}
	result := TrimMessages(msgs, 2, nil)

	for _, m := range result {
		if strings.HasPrefix(m.Content, ProgressMessagePrefix) {
			t.Fatal("old progress message should have been stripped")
		}
	}
	found := false
	for _, m := range result {
		if m.Content == "Actual request." {
			found = true
		}
	}
	if !found {
		t.Fatal("expected actual request message to be preserved")
	}
}

func TestFormatSnapshotContext_Nil(t *testing.T) {
	if got := FormatSnapshotContext(nil); got != "" {
		t.Errorf("expected empty for nil snapshot, got %q", got)
	}
}

func TestFormatSnapshotContext_ZeroSlides(t *testing.T) {
	s := &DesignSnapshot{SlideCount: 0, Title: "Empty"}
	if got := FormatSnapshotContext(s); got != "" {
		t.Errorf("expected empty for zero slides, got %q", got)
	}
}

func TestFormatSnapshotContext_TitleOnly(t *testing.T) {
	s := &DesignSnapshot{SlideCount: 5, Title: "My Deck"}
	result := FormatSnapshotContext(s)
	if !strings.Contains(result, "My Deck") {
		t.Error("missing title")
	}
	if !strings.Contains(result, "5") {
		t.Error("missing slide count")
	}
	if strings.Contains(result, "Structure") {
		t.Error("should not contain structure when no headings")
	}
	if strings.Contains(result, "Colors") {
		t.Error("should not contain colors when no palette")
	}
	if strings.Contains(result, "Fonts") {
		t.Error("should not contain fonts when none")
	}
	if strings.Contains(result, "CSS classes") {
		t.Error("should not contain CSS classes when none")
	}
}

func TestFormatSnapshotContext_WithHeadings(t *testing.T) {
	s := &DesignSnapshot{
		SlideCount:    3,
		Title:         "Pitch Deck",
		SlideHeadings: []string{"Intro", "Problem", "Solution"},
	}
	result := FormatSnapshotContext(s)
	if !strings.Contains(result, "Intro") {
		t.Error("missing heading Intro")
	}
	if !strings.Contains(result, "Problem") {
		t.Error("missing heading Problem")
	}
	if !strings.Contains(result, "Solution") {
		t.Error("missing heading Solution")
	}
	if !strings.Contains(result, "Structure") {
		t.Error("missing Structure section")
	}
}

func TestFormatSnapshotContext_ColorPalette(t *testing.T) {
	s := &DesignSnapshot{
		SlideCount:   2,
		ColorPalette: map[string]string{"primary": "#FF0000", "secondary": "#00FF00"},
	}
	result := FormatSnapshotContext(s)
	if !strings.Contains(result, "Colors") {
		t.Error("missing Colors section")
	}
	if !strings.Contains(result, "primary: #FF0000") {
		t.Error("missing primary color")
	}
	if !strings.Contains(result, "secondary: #00FF00") {
		t.Error("missing secondary color")
	}
}

func TestFormatSnapshotContext_ColorPaletteSingleEntry(t *testing.T) {
	s := &DesignSnapshot{
		SlideCount:   1,
		ColorPalette: map[string]string{"brand": "#123456"},
	}
	result := FormatSnapshotContext(s)
	if !strings.Contains(result, "Colors: brand: #123456") {
		t.Errorf("expected single color entry, got %q", result)
	}
}

func TestFormatSnapshotContext_Fonts(t *testing.T) {
	s := &DesignSnapshot{
		SlideCount: 1,
		Fonts: []FontInfo{
			{Family: "Inter", Source: "Google Fonts"},
			{Family: "JetBrains Mono", Source: "System"},
		},
	}
	result := FormatSnapshotContext(s)
	if !strings.Contains(result, "Fonts") {
		t.Error("missing Fonts section")
	}
	if !strings.Contains(result, "Inter (Google Fonts)") {
		t.Error("missing Inter font")
	}
	if !strings.Contains(result, "JetBrains Mono (System)") {
		t.Error("missing JetBrains Mono font")
	}
}

func TestFormatSnapshotContext_CSSClasses(t *testing.T) {
	s := &DesignSnapshot{
		SlideCount: 2,
		CSSClasses: []string{"slide-title", "slide-body", "fade-in"},
	}
	result := FormatSnapshotContext(s)
	if !strings.Contains(result, "CSS classes: slide-title, slide-body, fade-in") {
		t.Errorf("expected CSS classes joined, got %q", result)
	}
}

func TestFormatSnapshotContext_Full(t *testing.T) {
	s := &DesignSnapshot{
		SlideCount:    4,
		Title:          "Investor Deck",
		SlideHeadings:  []string{"Cover", "Market", "Traction", "Ask"},
		ColorPalette:   map[string]string{"bg": "#FFF", "text": "#333"},
		Fonts:          []FontInfo{{Family: "Lato", Source: "Google Fonts"}},
		CSSClasses:     []string{"hero", "grid"},
	}
	result := FormatSnapshotContext(s)
	if !strings.Contains(result, SnapshotMessagePrefix) {
		t.Error("missing prefix")
	}
	if !strings.Contains(result, "Investor Deck") {
		t.Error("missing title")
	}
	if !strings.Contains(result, "4") {
		t.Error("missing slide count")
	}
	if !strings.Contains(result, "Cover") {
		t.Error("missing heading")
	}
	if !strings.Contains(result, "bg: #FFF") {
		t.Error("missing color")
	}
	if !strings.Contains(result, "Lato (Google Fonts)") {
		t.Error("missing font")
	}
	if !strings.Contains(result, "hero, grid") {
		t.Error("missing CSS classes")
	}
}

func TestFormatSnapshotContext_EmptyTitleWithSlides(t *testing.T) {
	s := &DesignSnapshot{SlideCount: 3}
	result := FormatSnapshotContext(s)
	if !strings.Contains(result, "3") {
		t.Error("missing slide count")
	}
	if strings.Contains(result, "Title:") {
		t.Error("should not include Title when empty")
	}
}

// ============================================================================
// Compression summary preservation tests
// ============================================================================

func TestIsCompressMessage_MatchesPrefix(t *testing.T) {
	if !isCompressMessage(CompressMessagePrefix + " summary content") {
		t.Error("expected isCompressMessage to match prefix")
	}
	if isCompressMessage("regular message") {
		t.Error("expected isCompressMessage to not match regular content")
	}
	if isCompressMessage("") {
		t.Error("expected isCompressMessage to not match empty string")
	}
}

func TestTrimMessages_PreservesCompressionSummary(t *testing.T) {
	// Compression summary is within the keepRounds window: it is the user
	// message immediately before the last assistant round.
	msgs := []model.Message{
		makeMsg("system", "System prompt."),
		makeMsg("assistant", "Old response 1."),
		makeMsg("user", "Old task 2."),
		makeMsg("assistant", "Old response 2."),
		makeMsg("user", CompressMessagePrefix+" conversation history summarized below."),
		makeMsg("assistant", "Acknowledged. Continuing work."),
		makeMsg("user", "Next task."),
		makeMsg("assistant", "Working on it."),
	}
	result := TrimMessages(msgs, 2, nil)

	// The compression summary must be preserved if within the keepRounds window.
	found := false
	for _, m := range result {
		if m.Role == "user" && isCompressMessage(m.Content) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected compression summary to be preserved")
	}
}

func TestTrimMessages_PreservesCompressionSummaryEvenWithLowKeepRounds(t *testing.T) {
	// keepRounds=1, compression summary is in the last (only) kept round.
	msgs := []model.Message{
		makeMsg("system", "System prompt."),
		makeMsg("user", "First task."),
		makeMsg("assistant", "Done first."),
		makeMsg("user", CompressMessagePrefix+" summary of earlier work."),
		makeMsg("assistant", "Got it. Continuing."),
	}
	result := TrimMessages(msgs, 1, nil)

	found := false
	for _, m := range result {
		if m.Role == "user" && isCompressMessage(m.Content) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected compression summary to be preserved even with keepRounds=1")
	}
}

func TestTrimMessages_CompressionSummaryOutsideWindow_NotResurrected(t *testing.T) {
	// Compression summary is in round 1 of 5 (far outside keepRounds=2 window).
	// The scan is limited to [walkStart, len(full)), so it should NOT pull in
	// rounds already trimmed by keepRounds.
	msgs := []model.Message{
		makeMsg("system", "System prompt."),
		makeMsg("user", CompressMessagePrefix+" very old summary."),
		makeMsg("assistant", "Old response."),
		makeMsg("user", "Task 2."),
		makeMsg("assistant", "Response 2."),
		makeMsg("user", "Task 3."),
		makeMsg("assistant", "Response 3."),
		makeMsg("user", "Task 4. Current."),
		makeMsg("assistant", "Response 4."),
	}
	result := TrimMessages(msgs, 2, nil)

	found := false
	for _, m := range result {
		if m.Role == "user" && isCompressMessage(m.Content) {
			found = true
			break
		}
	}
	if found {
		t.Error("old compression summary outside keepRounds window should NOT be resurrected")
	}
}
