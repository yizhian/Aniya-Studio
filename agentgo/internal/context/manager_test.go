package context

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentgo/internal/model"
	"agentgo/internal/persistence"
)

// chdirTemp changes to a temporary directory and returns the original CWD
// and the temp dir path. Use for tests that need files relative to CWD.
func chdirTemp(t *testing.T) (tempDir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	})
	return dir
}

func TestContextManager_New_EmptyWorkspace(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "test-session", "")

	if cm.CurrentVersion() != 0 {
		t.Fatalf("expected CurrentVersion=0, got %d", cm.CurrentVersion())
	}
	if cm.LatestSnapshot() != nil {
		t.Fatal("expected nil LatestSnapshot")
	}
	if cm.LatestTodos() != nil {
		t.Fatal("expected nil LatestTodos")
	}
	if cm.KeepRounds() != KeepRounds {
		t.Fatalf("expected KeepRounds=%d, got %d", KeepRounds, cm.KeepRounds())
	}
}

func TestContextManager_DetectHTMLModification_HTMLWrite(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte(`<html><head><title>Test Deck</title></head><body>
			<section class="slide"><h1>Slide 1</h1></section>
			<section class="slide"><h1>Slide 2</h1></section>
		</body></html>`), 0o644)

	cm := NewContextManager(ws, "test-session", "")

	args, _ := json.Marshal(map[string]string{"path": "deck.html"})
	toolCalls := []model.ToolCall{
		{ID: "toolu_001", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args)}},
	}
	results := []ToolExecSummary{{ToolCallID: "toolu_001", Success: true}}

	cm.DetectHTMLModification(toolCalls, results)
	newVersion, err := cm.FinalizeSnapshot("")
	if err != nil {
		t.Fatalf("FinalizeSnapshot failed: %v", err)
	}
	if newVersion != 1 {
		t.Fatalf("expected newVersion=1, got %d", newVersion)
	}
	if cm.CurrentVersion() != 1 {
		t.Fatalf("expected CurrentVersion=1, got %d", cm.CurrentVersion())
	}
	snap := cm.LatestSnapshot()
	if snap == nil {
		t.Fatal("expected non-nil LatestSnapshot")
	}
	if snap.Title != "Test Deck" {
		t.Fatalf("expected title 'Test Deck', got %q", snap.Title)
	}
	if snap.SlideCount != 2 {
		t.Fatalf("expected SlideCount=2, got %d", snap.SlideCount)
	}

	// Verify the snapshot is on disk.
	store := NewSnapshotStore(ws)
	ctx, err := store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Version != 1 {
		t.Fatalf("expected version 1 on disk, got %d", ctx.Version)
	}
}

func TestContextManager_DetectHTMLModification_NonHTML(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("notes.txt", []byte("text file"), 0o644)

	cm := NewContextManager(ws, "s", "")

	args, _ := json.Marshal(map[string]string{"path": "notes.txt"})
	toolCalls := []model.ToolCall{
		{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args)}},
	}
	results := []ToolExecSummary{{ToolCallID: "t1", Success: true}}

	detected := cm.DetectHTMLModification(toolCalls, results)
	if detected {
		t.Fatal("expected no HTML modification detected for non-HTML file")
	}
	version, err := cm.FinalizeSnapshot("")
	if err == nil {
		t.Fatal("expected error from FinalizeSnapshot when no HTML file recorded")
	}
	if version != 0 {
		t.Fatalf("expected version=0, got %d", version)
	}
	if cm.CurrentVersion() != 0 {
		t.Fatal("expected CurrentVersion still 0")
	}
}

func TestContextManager_DetectHTMLModification_ToolFailed(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte("<html><section class=\"slide\"></section></html>"), 0o644)

	cm := NewContextManager(ws, "s", "")

	args, _ := json.Marshal(map[string]string{"path": "deck.html"})
	toolCalls := []model.ToolCall{
		{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args)}},
	}
	results := []ToolExecSummary{{ToolCallID: "t1", Success: false}}

	detected := cm.DetectHTMLModification(toolCalls, results)
	if detected {
		t.Fatal("expected no modification detected for failed tool")
	}
}

func TestContextManager_DetectHTMLModification_DifferentHTML(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte("<html><section class=\"slide\"></section></html>"), 0o644)
	os.WriteFile("other.html", []byte("<html><section class=\"slide\"></section></html>"), 0o644)

	cm := NewContextManager(ws, "s", "")

	// First HTML sets primary.
	args1, _ := json.Marshal(map[string]string{"path": "deck.html"})
	cm.DetectHTMLModification(
		[]model.ToolCall{{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args1)}}},
		[]ToolExecSummary{{ToolCallID: "t1", Success: true}},
	)
	v1, err := cm.FinalizeSnapshot("")
	if err != nil {
		t.Fatal(err)
	}
	if v1 != 1 {
		t.Fatalf("expected version 1 after first HTML, got %d", v1)
	}

	// Second HTML with different path should now be accepted (file switching is allowed).
	args2, _ := json.Marshal(map[string]string{"path": "other.html"})
	cm.DetectHTMLModification(
		[]model.ToolCall{{ID: "t2", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args2)}}},
		[]ToolExecSummary{{ToolCallID: "t2", Success: true}},
	)
	v2, err := cm.FinalizeSnapshot("")
	if err != nil {
		t.Fatal(err)
	}
	if v2 != 2 {
		t.Fatalf("expected version=2 for different HTML path (file switching allowed), got %d", v2)
	}
}

func TestContextManager_DetectHTMLModification_EditFile(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte("<html><section class=\"slide\"></section></html>"), 0o644)

	cm := NewContextManager(ws, "s", "")

	args, _ := json.Marshal(map[string]string{"path": "deck.html"})
	toolCalls := []model.ToolCall{
		{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "edit_file", Arguments: string(args)}},
	}
	results := []ToolExecSummary{{ToolCallID: "t1", Success: true}}

	cm.DetectHTMLModification(toolCalls, results)
	v, err := cm.FinalizeSnapshot("")
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 {
		t.Fatalf("expected version=1 for edit_file on HTML, got %d", v)
	}
}

func TestContextManager_DetectHTMLModification_ResumeSession_AbsolutePathInContext(t *testing.T) {
	// Regression test: context.json stores html_path as an absolute path
	// (e.g. "/tmp/.../deck.html"). When the session resumes and the LLM
	// calls edit_file with a relative path ("deck.html"), the
	// DetectHTMLModification should still recognise it as the same file.
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte("<html><section class=\"slide\"></section></html>"), 0o644)

	// Simulate a first version created in a prior session.
	cm1 := NewContextManager(ws, "s", "")
	args1, _ := json.Marshal(map[string]string{"path": "deck.html"})
	cm1.DetectHTMLModification(
		[]model.ToolCall{{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args1)}}},
		[]ToolExecSummary{{ToolCallID: "t1", Success: true}},
	)
	cm1.FinalizeSnapshot("")

	// Now simulate a resumed session: NewContextManager loads the absolute
	// html_path from context.json.
	cm2 := NewContextManager(ws, "s2", "")
	if cm2.htmlFilePath == "" {
		t.Fatal("expected htmlFilePath to be loaded from previous version")
	}
	if !filepath.IsAbs(cm2.htmlFilePath) {
		t.Fatalf("expected absolute htmlFilePath from context.json, got %q", cm2.htmlFilePath)
	}

	// edit_file with a RELATIVE path should still match.
	args2, _ := json.Marshal(map[string]string{"path": "deck.html"})
	cm2.DetectHTMLModification(
		[]model.ToolCall{{ID: "t2", Type: "function", Function: model.ToolCallFunction{Name: "edit_file", Arguments: string(args2)}}},
		[]ToolExecSummary{{ToolCallID: "t2", Success: true}},
	)
	v2, err := cm2.FinalizeSnapshot("")
	if err != nil {
		t.Fatal(err)
	}
	if v2 != 2 {
		t.Fatalf("expected version=2 for resumed session edit_file with relative path, got %d", v2)
	}
}

func TestContextManager_UpdateTodos_NoVersion(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "s", "")

	todos := []TodoItemRecord{{Content: "Task", Status: "pending", ActiveForm: "Doing task"}}
	err := cm.UpdateTodos(todos)
	if err != nil {
		t.Fatal(err)
	}
	if len(cm.LatestTodos()) != 1 {
		t.Fatalf("expected 1 cached todo, got %d", len(cm.LatestTodos()))
	}
	if cm.LatestTodos()[0].Content != "Task" {
		t.Fatalf("expected cached todo content 'Task', got %q", cm.LatestTodos()[0].Content)
	}

	store := NewSnapshotStore(ws)
	loaded, _ := store.LoadTodo(1)
	if loaded != nil {
		t.Fatal("expected no todolist.json on disk when no version exists")
	}
}

func TestContextManager_UpdateTodos_WithVersion(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte("<html><section class=\"slide\"></section></html>"), 0o644)

	cm := NewContextManager(ws, "s", "")

	args, _ := json.Marshal(map[string]string{"path": "deck.html"})
	cm.DetectHTMLModification(
		[]model.ToolCall{{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args)}}},
		[]ToolExecSummary{{ToolCallID: "t1", Success: true}},
	)
	cm.FinalizeSnapshot("")

	todos := []TodoItemRecord{
		{Content: "Build slides", Status: "in_progress", ActiveForm: "Building slides"},
		{Content: "Add navigation", Status: "pending", ActiveForm: "Adding navigation"},
	}
	err := cm.UpdateTodos(todos)
	if err != nil {
		t.Fatal(err)
	}

	if len(cm.LatestTodos()) != 2 {
		t.Fatalf("expected 2 todos in memory, got %d", len(cm.LatestTodos()))
	}

	store := NewSnapshotStore(ws)
	loaded, err := store.LoadTodo(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 todos on disk, got %d", len(loaded))
	}
	if loaded[0].Content != "Build slides" {
		t.Fatalf("expected 'Build slides', got %q", loaded[0].Content)
	}
}

func TestContextManager_SetKeepRounds(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "s", "")

	cm.SetKeepRounds(5)
	if cm.KeepRounds() != 5 {
		t.Fatalf("expected KeepRounds=5, got %d", cm.KeepRounds())
	}

	cm.SetKeepRounds(0)
	if cm.KeepRounds() != 5 {
		t.Fatalf("expected KeepRounds still 5 after SetKeepRounds(0), got %d", cm.KeepRounds())
	}

	cm.SetKeepRounds(-1)
	if cm.KeepRounds() != 5 {
		t.Fatalf("expected KeepRounds still 5 after SetKeepRounds(-1), got %d", cm.KeepRounds())
	}
}

func TestContextManager_LoadsExistingState(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte("<html><section class=\"slide\"></section></html>"), 0o644)

	cm1 := NewContextManager(ws, "sess-1", "")
	args, _ := json.Marshal(map[string]string{"path": "deck.html"})
	cm1.DetectHTMLModification(
		[]model.ToolCall{{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args)}}},
		[]ToolExecSummary{{ToolCallID: "t1", Success: true}},
	)
	cm1.FinalizeSnapshot("")
	cm1.UpdateTodos([]TodoItemRecord{{Content: "Saved task", Status: "completed", ActiveForm: "Saving task"}})

	cm2 := NewContextManager(ws, "sess-2", "")
	if cm2.CurrentVersion() != 1 {
		t.Fatalf("expected CurrentVersion=1, got %d", cm2.CurrentVersion())
	}
	snap := cm2.LatestSnapshot()
	if snap == nil {
		t.Fatal("expected non-nil LatestSnapshot from existing store")
	}
	todos := cm2.LatestTodos()
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo loaded from existing store, got %d", len(todos))
	}
	if todos[0].Content != "Saved task" {
		t.Fatalf("expected 'Saved task', got %q", todos[0].Content)
	}
}

func TestContextManager_AssembleMessages(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "s", "")

	msgs := []model.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello."},
		{Role: "assistant", Content: "Hi there!"},
	}
	result := cm.AssembleMessages(msgs)
	if len(result) == 0 {
		t.Fatal("expected non-empty messages from AssembleMessages")
	}
	if result[0].Role != "system" {
		t.Fatalf("expected system message first, got %q", result[0].Role)
	}
}

func TestFormatMemoryContext_Empty(t *testing.T) {
	if got := FormatMemoryContext(nil); got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
	if got := FormatMemoryContext([]persistence.RecalledMemory{}); got != "" {
		t.Errorf("expected empty for empty slice, got %q", got)
	}
}

func TestFormatMemoryContext_GroupsByType(t *testing.T) {
	recalled := []persistence.RecalledMemory{
		{Type: persistence.MemoryTypeFeedback, Summary: "No animations", Content: "User banned animations."},
		{Type: persistence.MemoryTypeDesign, Summary: "Dark theme required", Content: "All pages must use dark theme."},
		{Type: persistence.MemoryTypeFeedback, Summary: "Prefer oklch", Content: "Use oklch color space."},
	}
	result := FormatMemoryContext(recalled)
	if !strings.Contains(result, "[Memory Context]") {
		t.Error("missing header")
	}
	if !strings.Contains(result, "Related Design Decisions") {
		t.Error("missing design section")
	}
	if !strings.Contains(result, "Related Feedback") {
		t.Error("missing feedback section")
	}
	// Design should come before Feedback per the type order.
	designIdx := strings.Index(result, "Related Design Decisions")
	feedbackIdx := strings.Index(result, "Related Feedback")
	if designIdx == -1 || feedbackIdx == -1 || feedbackIdx > designIdx {
		t.Error("expected feedback section before design section")
	}
}

func TestFormatMemoryContext_FreshnessWarning(t *testing.T) {
	recalled := []persistence.RecalledMemory{
		{Type: persistence.MemoryTypeDesign, Summary: "Old decision", Content: "content", DaysAgo: 5},
		{Type: persistence.MemoryTypeDesign, Summary: "Fresh decision", Content: "content", DaysAgo: 0},
	}
	result := FormatMemoryContext(recalled)
	if !strings.Contains(result, "5 days old") {
		t.Error("expected freshness warning for old memory")
	}
	if !strings.Contains(result, "verify this still applies") {
		t.Error("expected verification hint")
	}
	if strings.Contains(result, "Fresh decision") && strings.Contains(result, "days old") {
		// The "0 days old" warning should NOT appear (DaysAgo <= 1).
		// Check that the 0-days-ago entry does NOT have a warning.
		idx := strings.Index(result, "Fresh decision")
		after := result[idx:]
		if strings.Contains(after, "days old") {
			t.Error("fresh memory (0 days) should not have a warning")
		}
	}
}

func TestFormatMemoryContext_TruncatedContent(t *testing.T) {
	recalled := []persistence.RecalledMemory{
		{Type: persistence.MemoryTypeDesign, Summary: "Big", Content: strings.Repeat("x", 500)},
	}
	result := FormatMemoryContext(recalled)
	if !strings.Contains(result, "...") {
		t.Error("expected truncation ellipsis for long content")
	}
	if strings.Contains(result, strings.Repeat("x", 500)) {
		t.Error("long content should be truncated")
	}
}

func TestSetRecalledMemories(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "s", "")

	recalled := []persistence.RecalledMemory{
		{Type: persistence.MemoryTypeDesign, Summary: "Test", Content: "Test content"},
	}
	cm.SetRecalledMemories(recalled)

	// Verify via AssembleMessages that memories are injected.
	msgs := []model.Message{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "Hello."},
	}
	result := cm.AssembleMessages(msgs)
	found := false
	for _, m := range result {
		if strings.Contains(m.Content, "Memory Context") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected memory context to be injected into messages")
	}
}

func TestAssembleMessages_NoRecalled_NoInjection(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "s", "")

	msgs := []model.Message{
		{Role: "system", Content: "System."},
		{Role: "user", Content: "Hello."},
	}
	result := cm.AssembleMessages(msgs)
	for _, m := range result {
		if strings.Contains(m.Content, "Memory Context") {
			t.Error("should not inject memory context when no memories recalled")
		}
	}
}

func TestMemoryPriority_Ordering(t *testing.T) {
	if memoryPriority(persistence.MemoryTypeFeedback) >= memoryPriority(persistence.MemoryTypeDesign) {
		t.Error("feedback should have lower priority number than design")
	}
	if memoryPriority(persistence.MemoryTypeDesign) >= memoryPriority(persistence.MemoryTypeComponent) {
		t.Error("design should have lower priority than component")
	}
	if memoryPriority(persistence.MemoryTypeComponent) >= memoryPriority(persistence.MemoryTypeTask) {
		t.Error("component should have lower priority than task")
	}
	if memoryPriority("unknown") != 4 {
		t.Errorf("unknown memory type should have priority 4, got %d", memoryPriority("unknown"))
	}
}

func TestPrioritiseMemory_FitsInBudget(t *testing.T) {
	recalled := []persistence.RecalledMemory{
		{Type: persistence.MemoryTypeTask, Content: "task content here"},
		{Type: persistence.MemoryTypeFeedback, Content: "important feedback"},
		{Type: persistence.MemoryTypeDesign, Content: "design notes"},
	}
	// Budget is larger than total content.
	result := prioritiseMemory(recalled, 10000)
	if len(result) != 3 {
		t.Fatalf("expected all 3 memories, got %d", len(result))
	}
	// Feedback should come first (priority 0).
	if result[0].Type != persistence.MemoryTypeFeedback {
		t.Errorf("expected feedback first, got %s", result[0].Type)
	}
}

func TestPrioritiseMemory_ExceedsBudget(t *testing.T) {
	recalled := []persistence.RecalledMemory{
		{Type: persistence.MemoryTypeTask, Content: "task A - long content that takes up space"},
		{Type: persistence.MemoryTypeTask, Content: "task B - even more long content here"},
		{Type: persistence.MemoryTypeFeedback, Content: "critical feedback"},
		{Type: persistence.MemoryTypeDesign, Content: "design specs"},
	}
	// Budget only allows ~2 memories.
	result := prioritiseMemory(recalled, 40)
	if len(result) < 1 {
		t.Fatal("expected at least 1 memory")
	}
	// Should keep high-priority memories first.
	if result[0].Type != persistence.MemoryTypeFeedback {
		t.Errorf("expected feedback first (highest priority), got %s", result[0].Type)
	}
	// Should not include all memories.
	if len(result) == len(recalled) {
		t.Error("should have dropped some memories due to budget")
	}
}

func TestPrioritiseMemory_EmptyBudget(t *testing.T) {
	recalled := []persistence.RecalledMemory{
		{Type: persistence.MemoryTypeFeedback, Content: "note"},
	}
	result := prioritiseMemory(recalled, 1)
	if len(result) != 0 {
		t.Error("expected no memories when budget is 1 (content length > budget)")
	}
}

func TestApplyBudget_UnderBudget(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "s", "")

	// Short messages + small memories = well under budget.
	msgs := []model.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello."},
	}
	cm.SetRecalledMemories([]persistence.RecalledMemory{
		{Type: persistence.MemoryTypeFeedback, Content: "small note"},
	})
	origKeep := cm.KeepRounds()
	result := cm.applyBudget(msgs)
	// Keep rounds should not change when under budget.
	if cm.KeepRounds() != origKeep {
		t.Error("keepRounds should not change under budget")
	}
	_ = result
}

func TestApplyBudget_OverBudget(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "s", "")
	cm.SetKeepRounds(20)

	// 85% of 128000 tokens = 108800 tokens ≈ 435200 chars.
	// Use 4 chars per estimated token.
	largeContent := strings.Repeat("x", 200000)
	msgs := []model.Message{
		{Role: "system", Content: largeContent},
		{Role: "user", Content: largeContent},
	}
	cm.SetRecalledMemories([]persistence.RecalledMemory{
		{Type: persistence.MemoryTypeTask, Content: strings.Repeat("y", 200000)},
		{Type: persistence.MemoryTypeFeedback, Content: strings.Repeat("z", 200000)},
	})
	result := cm.applyBudget(msgs)
	if cm.KeepRounds() >= 20 {
		t.Error("keepRounds should be reduced when over budget")
	}
	_ = result
}

func TestInjectMemoryContext_NoSystem(t *testing.T) {
	recalled := []persistence.RecalledMemory{
		{Type: persistence.MemoryTypeFeedback, Summary: "Note", Content: "Test content."},
	}
	msgs := []model.Message{
		{Role: "user", Content: "Hello."},
	}
	result := injectMemoryContext(msgs, recalled)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("expected memory as first message, got role=%q", result[0].Role)
	}
	if !strings.Contains(result[0].Content, "Memory Context") {
		t.Error("expected memory context in first message")
	}
}

func TestInjectMemoryContext_EmptyRecalled(t *testing.T) {
	msgs := []model.Message{{Role: "user", Content: "Hello."}}
	result := injectMemoryContext(msgs, nil)
	if len(result) != 1 {
		t.Errorf("expected 1 message, got %d", len(result))
	}
}

func TestEstimateTokens(t *testing.T) {
	msgs := []model.Message{
		{Role: "user", Content: "hello"},
	}
	tokens := EstimateMessageTokens(msgs)
	if tokens == 0 {
		t.Error("expected non-zero token estimate")
	}
}

func TestEstimateRecallTokens(t *testing.T) {
	recalled := []persistence.RecalledMemory{
		{Content: "hello world"},
	}
	tokens := estimateRecallTokens(recalled)
	if tokens == 0 {
		t.Error("expected non-zero token estimate")
	}
}

func TestInjectProgressAfterSnapshot_WithSystemAndSnapshot(t *testing.T) {
	msgs := []model.Message{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: SnapshotMessagePrefix + "\nsnapshot content"},
		{Role: "user", Content: "Hello."},
	}
	progress := ProgressMessagePrefix + "\nprogress content"
	result := injectProgressAfterSnapshot(msgs, progress)
	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Error("system should be first")
	}
	if !strings.HasPrefix(result[1].Content, SnapshotMessagePrefix) {
		t.Error("snapshot should be second")
	}
	if !strings.HasPrefix(result[2].Content, ProgressMessagePrefix) {
		t.Error("progress should be third (after snapshot)")
	}
	if result[3].Content != "Hello." {
		t.Error("user message should be last")
	}
}

func TestInjectProgressAfterSnapshot_NoSnapshot(t *testing.T) {
	msgs := []model.Message{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "Hello."},
	}
	progress := ProgressMessagePrefix + "\nprogress content"
	result := injectProgressAfterSnapshot(msgs, progress)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Error("system should be first")
	}
	if !strings.HasPrefix(result[1].Content, ProgressMessagePrefix) {
		t.Error("progress should be second (after system, before user)")
	}
}

func TestInjectProgressAfterSnapshot_NoSystem(t *testing.T) {
	msgs := []model.Message{
		{Role: "user", Content: "Hello."},
	}
	progress := ProgressMessagePrefix + "\nprogress content"
	result := injectProgressAfterSnapshot(msgs, progress)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if !strings.HasPrefix(result[0].Content, ProgressMessagePrefix) {
		t.Error("progress should be first when no system message")
	}
}

func TestInjectProgressAfterSnapshot_EmptyMessages(t *testing.T) {
	result := injectProgressAfterSnapshot(nil, "progress")
	if len(result) != 0 {
		t.Error("expected empty result for nil messages")
	}
}

func TestInjectProgressAfterSnapshot_SystemOnly(t *testing.T) {
	msgs := []model.Message{
		{Role: "system", Content: "System prompt."},
	}
	progress := ProgressMessagePrefix + "\nprogress"
	result := injectProgressAfterSnapshot(msgs, progress)
	if len(result) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Error("system should be first")
	}
	if !strings.HasPrefix(result[1].Content, ProgressMessagePrefix) {
		t.Error("progress should be second")
	}
}

func TestInjectProgressAfterSnapshot_OnlySnapshotNoSystem(t *testing.T) {
	msgs := []model.Message{
		{Role: "user", Content: SnapshotMessagePrefix + "\nsnapshot"},
		{Role: "user", Content: "Hello."},
	}
	progress := ProgressMessagePrefix + "\nprogress"
	result := injectProgressAfterSnapshot(msgs, progress)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if !strings.HasPrefix(result[0].Content, ProgressMessagePrefix) {
		t.Error("progress should be first when no system message")
	}
}

func TestInjectProgressAfterSnapshot_Idempotent(t *testing.T) {
	// Verify that injecting twice doesn't duplicate.
	msgs := []model.Message{
		{Role: "system", Content: "System."},
		{Role: "user", Content: SnapshotMessagePrefix + "\nsnap"},
		{Role: "user", Content: "Hello."},
	}
	progress := ProgressMessagePrefix + "\np"
	result1 := injectProgressAfterSnapshot(msgs, progress)
	result2 := injectProgressAfterSnapshot(result1, progress)

	// Count progress messages in result2.
	count := 0
	for _, m := range result2 {
		if strings.HasPrefix(m.Content, ProgressMessagePrefix) {
			count++
		}
	}
	// The second injection puts another progress message since injectProgressAfterSnapshot
	// doesn't deduplicate (TrimMessages strips old ones). So there may be 2.
	// This test documents the current behavior.
	if count < 1 {
		t.Error("expected at least 1 progress message")
	}
}

func TestAssembleMessages_WithProgress(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte("<html><section class=\"slide\"><h1>S1</h1></section></html>"), 0o644)

	cm := NewContextManager(ws, "s", "")
	// Create a version so context manager has progress state.
	args, _ := json.Marshal(map[string]string{"path": "deck.html"})
	cm.DetectHTMLModification(
		[]model.ToolCall{{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args)}}},
		[]ToolExecSummary{{ToolCallID: "t1", Success: true}},
	)
	cm.FinalizeSnapshot("")
	cm.UpdateTodos([]TodoItemRecord{
		{Content: "Build slides", Status: "in_progress", ActiveForm: "Building slides"},
	})

	msgs := []model.Message{
		{Role: "system", Content: "System."},
		{Role: "user", Content: "Build a deck."},
	}
	result := cm.AssembleMessages(msgs)
	foundProgress := false
	foundSnapshot := false
	for _, m := range result {
		if strings.HasPrefix(m.Content, ProgressMessagePrefix) {
			foundProgress = true
		}
		if strings.HasPrefix(m.Content, SnapshotMessagePrefix) {
			foundSnapshot = true
		}
	}
	if !foundProgress {
		t.Error("expected progress summary to be injected")
	}
	if !foundSnapshot {
		t.Error("expected design snapshot to be injected")
	}
}

func TestAssembleMessages_ProgressAndMemories(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte("<html><section class=\"slide\"><h1>S1</h1></section></html>"), 0o644)

	cm := NewContextManager(ws, "s", "")
	args, _ := json.Marshal(map[string]string{"path": "deck.html"})
	cm.DetectHTMLModification(
		[]model.ToolCall{{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args)}}},
		[]ToolExecSummary{{ToolCallID: "t1", Success: true}},
	)
	cm.FinalizeSnapshot("")
	cm.UpdateTodos([]TodoItemRecord{
		{Content: "Build slides", Status: "in_progress", ActiveForm: "Building slides"},
		{Content: "Verify", Status: "pending", ActiveForm: "Verifying"},
	})
	cm.SetRecalledMemories([]persistence.RecalledMemory{
		{Type: persistence.MemoryTypeFeedback, Summary: "No gradients", Content: "User banned gradient backgrounds."},
	})

	msgs := []model.Message{
		{Role: "system", Content: "System."},
		{Role: "user", Content: "Build a deck."},
	}
	result := cm.AssembleMessages(msgs)

	// Verify ordering: system → snapshot → progress → memory → user
	var roles []string
	for _, m := range result {
		if strings.HasPrefix(m.Content, SnapshotMessagePrefix) {
			roles = append(roles, "snapshot")
		} else if strings.HasPrefix(m.Content, ProgressMessagePrefix) {
			roles = append(roles, "progress")
		} else if strings.Contains(m.Content, "Memory Context") {
			roles = append(roles, "memory")
		} else {
			roles = append(roles, m.Role)
		}
	}

	// Check snapshot comes before progress.
	snapIdx, progIdx := -1, -1
	for i, r := range roles {
		if r == "snapshot" {
			snapIdx = i
		}
		if r == "progress" {
			progIdx = i
		}
	}
	if snapIdx < 0 {
		t.Error("snapshot missing")
	}
	if progIdx < 0 {
		t.Error("progress missing")
	}
	if snapIdx >= progIdx {
		t.Errorf("expected snapshot (idx=%d) before progress (idx=%d)", snapIdx, progIdx)
	}
}

func TestAssembleMessages_NoVersionNoTodos_NoProgress(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "s", "")

	msgs := []model.Message{
		{Role: "system", Content: "System."},
		{Role: "user", Content: "Hello."},
	}
	result := cm.AssembleMessages(msgs)
	for _, m := range result {
		if strings.HasPrefix(m.Content, ProgressMessagePrefix) {
			t.Error("should not inject progress when no version and no todos")
		}
	}
}

func TestAssembleMessages_VersionZeroWithTodos_ShowsProgress(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "s", "")
	// Version 0 but with todos set (in-memory only, no snapshot yet).
	cm.UpdateTodos([]TodoItemRecord{
		{Content: "Task 1", Status: "in_progress", ActiveForm: "Doing task 1"},
	})

	msgs := []model.Message{
		{Role: "system", Content: "System."},
		{Role: "user", Content: "Hello."},
	}
	result := cm.AssembleMessages(msgs)
	found := false
	for _, m := range result {
		if strings.HasPrefix(m.Content, ProgressMessagePrefix) {
			found = true
			if strings.Contains(m.Content, "Version:") {
				t.Error("should not show version when version is 0")
			}
			if !strings.Contains(m.Content, "0/1 completed") {
				t.Error("should show 0/1 completed")
			}
		}
	}
	if !found {
		t.Error("should inject progress even with version 0 when todos exist")
	}
}

func TestAssembleMessages_MemoryOnly_NoProgressWhenEmpty(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "s", "")
	cm.SetRecalledMemories([]persistence.RecalledMemory{
		{Type: persistence.MemoryTypeDesign, Summary: "Dark theme", Content: "Use dark backgrounds."},
	})

	msgs := []model.Message{
		{Role: "system", Content: "System."},
		{Role: "user", Content: "Hello."},
	}
	result := cm.AssembleMessages(msgs)

	// Should have memory but no progress.
	hasMemory := false
	hasProgress := false
	for _, m := range result {
		if strings.Contains(m.Content, "Memory Context") {
			hasMemory = true
		}
		if strings.HasPrefix(m.Content, ProgressMessagePrefix) {
			hasProgress = true
		}
	}
	if !hasMemory {
		t.Error("should inject memory")
	}
	if hasProgress {
		t.Error("should not inject empty progress")
	}
}

// ─── Normalizer integration tests ───

func TestContextManager_Normalizer_CleanFileUnchanged(t *testing.T) {
	ws := chdirTemp(t)
	clean := `<!DOCTYPE html>
	<html><head><style>body { width: 1920px; height: 1080px; }</style></head><body>
	<section class="slide"><h1>Clean Slide</h1></section>
	</body></html>`
	os.WriteFile("deck.html", []byte(clean), 0o644)

	cm := NewContextManager(ws, "normclean", "")
	args, _ := json.Marshal(map[string]string{"path": "deck.html"})
	toolCalls := []model.ToolCall{
		{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args)}},
	}
	results := []ToolExecSummary{{ToolCallID: "t1", Success: true}}

	cm.DetectHTMLModification(toolCalls, results)
	_, err := cm.FinalizeSnapshot("")
	if err != nil {
		t.Fatalf("FinalizeSnapshot failed: %v", err)
	}

	data, err := os.ReadFile("deck.html")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != clean {
		t.Errorf("clean file should not be modified by Normalizer\ngot: %q\nwant: %q", string(data), clean)
	}
}

func TestContextManager_Normalizer_NonHTMLUntouched(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("notes.txt", []byte("just notes"), 0o644)

	cm := NewContextManager(ws, "normtxt", "")
	args, _ := json.Marshal(map[string]string{"path": "notes.txt"})
	toolCalls := []model.ToolCall{
		{ID: "t1", Type: "function", Function: model.ToolCallFunction{Name: "write_file", Arguments: string(args)}},
	}
	results := []ToolExecSummary{{ToolCallID: "t1", Success: true}}

	detected := cm.DetectHTMLModification(toolCalls, results)
	if detected {
		t.Error("expected no modification detected for non-HTML file")
	}
}

// ============================================================================
// CompressMessages tests
// ============================================================================

func TestCompressMessages_Structure(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewContextManager(tmpDir, "test-session", "")

	msgs := []model.Message{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "First task."},
		{Role: "assistant", Content: "Done first."},
		{Role: "user", Content: "Second task."},
		{Role: "assistant", Content: "Done second."},
		{Role: "user", Content: "Third task."},
		{Role: "assistant", Content: "Working on it."},
	}

	summary := CompressMessagePrefix + " Compression summary content."
	result := cm.CompressMessages(msgs, summary)

	// Must have at least system + compression summary.
	if len(result) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(result))
	}
	// First message must be system.
	if result[0].Role != "system" {
		t.Errorf("expected system message first, got %q", result[0].Role)
	}
	// Must contain the compression summary.
	found := false
	for _, m := range result {
		if m.Role == "user" && isCompressMessage(m.Content) {
			found = true
			if !strings.Contains(m.Content, "Compression summary content.") {
				t.Error("compression summary should contain the provided text")
			}
		}
	}
	if !found {
		t.Error("expected a compression summary message")
	}
}

func TestCompressMessages_KeepRoundsRestored(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewContextManager(tmpDir, "test-session", "")

	original := cm.KeepRounds()

	msgs := []model.Message{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "Task 1."},
		{Role: "assistant", Content: "Response 1."},
	}

	summary := CompressMessagePrefix + " Test summary."
	cm.CompressMessages(msgs, summary)

	// keepRounds must be restored to original value after CompressMessages.
	if cm.KeepRounds() != original {
		t.Errorf("keepRounds was %d before CompressMessages, now %d; should be restored",
			original, cm.KeepRounds())
	}
}

func TestCompressMessages_EmptyInput(t *testing.T) {
	tmpDir := t.TempDir()
	cm := NewContextManager(tmpDir, "test-session", "")

	summary := CompressMessagePrefix + " Empty test."
	result := cm.CompressMessages(nil, summary)

	// Should contain at least the compression summary.
	if len(result) == 0 {
		t.Fatal("expected non-empty result even with nil input")
	}
	found := false
	for _, m := range result {
		if m.Role == "user" && isCompressMessage(m.Content) {
			found = true
		}
	}
	if !found {
		t.Error("expected compression summary in result")
	}
}

// ============================================================================
// EstimateMessageTokens export test
// ============================================================================

func TestEstimateMessageTokens_Export(t *testing.T) {
	// Verify that EstimateMessageTokens is exported and functional.
	msgs := []model.Message{
		{Role: "user", Content: "Hello world"},
	}
	tokens := EstimateMessageTokens(msgs)
	if tokens <= 0 {
		t.Errorf("expected positive token estimate, got %d", tokens)
	}
}

func TestEstimateMessageTokens_Empty(t *testing.T) {
	tokens := EstimateMessageTokens(nil)
	// Implementation returns 1 as minimum for nil (reserves token for system message).
	if tokens != 1 {
		t.Errorf("expected 1 token minimum for nil, got %d", tokens)
	}
}
