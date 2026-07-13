package hook

import (
	"testing"
)

func TestNewSessionState(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	if s.SessionID != "sess1" {
		t.Errorf("expected session ID 'sess1', got %q", s.SessionID)
	}
	if s.WorkspacePath != "/tmp/ws" {
		t.Errorf("expected workspace '/tmp/ws', got %q", s.WorkspacePath)
	}
	if s.Stage != StageInitialGeneration {
		t.Errorf("expected stage initial_generation, got %q", s.Stage)
	}
	if s.MaxConsecutiveFailures != 3 {
		t.Errorf("expected max failures 3, got %d", s.MaxConsecutiveFailures)
	}
}

func TestRecordToolCallReadFile(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.RecordToolCall("read_file", map[string]any{"path": "index.html"}, false, map[string]any{
		"read_mtime_unix_ns": int64(1000),
	})

	if s.ToolsCalled["read_file"] != 1 {
		t.Errorf("expected read_file called once, got %d", s.ToolsCalled["read_file"])
	}
	absPath := resolveStatePath("/tmp/ws", "index.html")
	if s.FilesRead[absPath] != 1000 {
		t.Errorf("expected mtime 1000 for %q, got %d", absPath, s.FilesRead[absPath])
	}
}

func TestRecordToolCallWriteFileHTML(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.RecordToolCall("write_file", map[string]any{"path": "slide.html"}, false, nil)

	if !s.HTMLWritten {
		t.Error("expected HTMLWritten to be true after writing .html file")
	}
	absPath := resolveStatePath("/tmp/ws", "slide.html")
	if !s.FilesWritten[absPath] {
		t.Errorf("expected %q to be marked as written", absPath)
	}
}

func TestRecordToolCallWriteFileNonHTML(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.RecordToolCall("write_file", map[string]any{"path": "styles.css"}, false, nil)

	if s.HTMLWritten {
		t.Error("expected HTMLWritten to be false after writing .css file")
	}
}

func TestRecordToolCallEditFile(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	// edit_file should mark file as written but NOT trigger HTMLWritten for the edit_file handler
	// Actually wait, edit_file does set HTMLWritten for .html files
	s.RecordToolCall("edit_file", map[string]any{"path": "index.html"}, false, nil)

	absPath := resolveStatePath("/tmp/ws", "index.html")
	if !s.FilesWritten[absPath] {
		t.Errorf("expected %q to be marked as written after edit", absPath)
	}
	if !s.HTMLWritten {
		t.Error("expected HTMLWritten to be true after editing .html file")
	}
}

func TestRecordToolCall_HTMLPublishCount_WriteFileHTML(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.RecordToolCall("write_file", map[string]any{"path": "deck.html"}, false, nil)
	if s.HTMLPublishCount != 1 {
		t.Errorf("expected HTMLPublishCount 1 after writing .html, got %d", s.HTMLPublishCount)
	}

	s.RecordToolCall("write_file", map[string]any{"path": "deck.html"}, false, nil)
	if s.HTMLPublishCount != 2 {
		t.Errorf("expected HTMLPublishCount 2 after second write, got %d", s.HTMLPublishCount)
	}
}

func TestRecordToolCall_HTMLPublishCount_EditFileHTML(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.RecordToolCall("edit_file", map[string]any{"path": "slides.html"}, false, nil)
	if s.HTMLPublishCount != 0 {
		t.Errorf("expected HTMLPublishCount 0 after editing .html, got %d", s.HTMLPublishCount)
	}
	if s.HTMLPatchCount != 1 {
		t.Errorf("expected HTMLPatchCount 1 after editing .html, got %d", s.HTMLPatchCount)
	}
}

func TestRecordToolCall_HTMLPublishCount_NonHTML(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.RecordToolCall("write_file", map[string]any{"path": "styles.css"}, false, nil)
	if s.HTMLPublishCount != 0 {
		t.Errorf("expected HTMLPublishCount 0 for .css write, got %d", s.HTMLPublishCount)
	}

	s.RecordToolCall("edit_file", map[string]any{"path": "app.js"}, false, nil)
	if s.HTMLPublishCount != 0 {
		t.Errorf("expected HTMLPublishCount 0 for .js edit, got %d", s.HTMLPublishCount)
	}
}

func TestRecordToolCall_HTMLPublishCount_MixedWrites(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	// HTML write → +1 Publish
	s.RecordToolCall("write_file", map[string]any{"path": "page.html"}, false, nil)
	// Non-HTML write → no change
	s.RecordToolCall("write_file", map[string]any{"path": "style.css"}, false, nil)
	// HTML edit → +1 Patch, Publish unchanged
	s.RecordToolCall("edit_file", map[string]any{"path": "page.html"}, false, nil)
	// Failed HTML write → no change
	s.RecordToolCall("write_file", map[string]any{"path": "other.html"}, true, nil)

	if s.HTMLPublishCount != 1 {
		t.Errorf("expected HTMLPublishCount 1 (1 successful HTML write), got %d", s.HTMLPublishCount)
	}
	if s.HTMLPatchCount != 1 {
		t.Errorf("expected HTMLPatchCount 1 (1 successful HTML edit), got %d", s.HTMLPatchCount)
	}
}

func TestRecordToolCall_HTMLPublishCount_ErrorDoesNotIncrement(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.RecordToolCall("write_file", map[string]any{"path": "deck.html"}, true, nil)
	if s.HTMLPublishCount != 0 {
		t.Errorf("expected HTMLPublishCount 0 for failed write, got %d", s.HTMLPublishCount)
	}

	s.RecordToolCall("edit_file", map[string]any{"path": "deck.html"}, true, nil)
	if s.HTMLPublishCount != 0 {
		t.Errorf("expected HTMLPublishCount 0 for failed edit, got %d", s.HTMLPublishCount)
	}
	if s.HTMLPatchCount != 0 {
		t.Errorf("expected HTMLPatchCount 0 for failed edit, got %d", s.HTMLPatchCount)
	}
}

func TestRecordToolCall_HTMLPatchCount_EditFileHTML(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.RecordToolCall("edit_file", map[string]any{"path": "deck.html"}, false, nil)
	if s.HTMLPatchCount != 1 {
		t.Errorf("expected HTMLPatchCount 1 after first edit, got %d", s.HTMLPatchCount)
	}

	s.RecordToolCall("edit_file", map[string]any{"path": "deck.html"}, false, nil)
	if s.HTMLPatchCount != 2 {
		t.Errorf("expected HTMLPatchCount 2 after second edit, got %d", s.HTMLPatchCount)
	}

	// Publish count should remain unchanged.
	if s.HTMLPublishCount != 0 {
		t.Errorf("expected HTMLPublishCount 0 after edits only, got %d", s.HTMLPublishCount)
	}
}

func TestRecordToolCall_HTMLPublishCount_HTMLExtension(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	// .htm should also count.
	s.RecordToolCall("write_file", map[string]any{"path": "index.htm"}, false, nil)
	if s.HTMLPublishCount != 1 {
		t.Errorf("expected HTMLPublishCount 1 for .htm, got %d", s.HTMLPublishCount)
	}
}

func TestRecordToolCallSkillGet(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.RecordToolCall("skill", map[string]any{"operation": "get", "name": "frontend-design"}, false, nil)

	if !s.SkillGetCalled {
		t.Error("expected SkillGetCalled to be true")
	}
	if !s.SkillsLoaded["frontend-design"] {
		t.Error("expected frontend-design to be in SkillsLoaded")
	}
	if !s.HasLoadedSkill() {
		t.Error("expected HasLoadedSkill to return true")
	}
}

func TestRecordToolCallTodoWrite(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.RecordToolCall("todo_write", map[string]any{}, false, map[string]any{
		"count": 5,
	})

	if !s.HasUsedTodoWrite {
		t.Error("expected HasUsedTodoWrite to be true")
	}
	if s.LastTodoCount != 5 {
		t.Errorf("expected LastTodoCount 5, got %d", s.LastTodoCount)
	}
}

func TestRecordToolCallFailureTracking(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	// Three failures in a row.
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, true, nil)
	if s.ToolsFailed["write_file"] != 1 {
		t.Errorf("expected 1 failure, got %d", s.ToolsFailed["write_file"])
	}

	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, true, nil)
	if s.ToolsFailed["write_file"] != 2 {
		t.Errorf("expected 2 failures, got %d", s.ToolsFailed["write_file"])
	}

	// Success resets the counter.
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, false, nil)
	if s.ToolsFailed["write_file"] != 0 {
		t.Errorf("expected 0 failures after success, got %d", s.ToolsFailed["write_file"])
	}
}

func TestRecordToolCallDuplicateDetection(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	args := map[string]any{"path": "test.html", "content": "hello"}

	// Call with same args 3 times.
	s.RecordToolCall("write_file", args, true, nil)
	s.RecordToolCall("write_file", args, true, nil)
	s.RecordToolCall("write_file", args, true, nil)

	key := "write_file:" + HashToolArgs(args)
	if s.CallHistory[key] != 3 {
		t.Errorf("expected CallHistory 3, got %d", s.CallHistory[key])
	}
}

func TestAllLastRoundReadOnly(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	// Empty = not read-only
	if s.AllLastRoundReadOnly() {
		t.Error("expected false for empty last round tools")
	}

	s.RecordToolStartCalled("read_file")
	s.RecordToolStartCalled("web_fetch")

	if !s.AllLastRoundReadOnly() {
		t.Error("expected true when all tools are read-only")
	}

	s.RecordToolStartCalled("write_file")
	if s.AllLastRoundReadOnly() {
		t.Error("expected false when destructive tool is present")
	}
}

func TestTrackRoundEnd(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	s.TrackRoundEnd()
	if s.ConsecutiveRoundsNoWrite != 1 {
		t.Errorf("expected 1 consecutive rounds no write, got %d", s.ConsecutiveRoundsNoWrite)
	}

	// Simulate writing HTML.
	s.RecordToolCall("write_file", map[string]any{"path": "test.html"}, false, nil)
	s.TrackRoundEnd()
	if s.ConsecutiveRoundsNoWrite != 0 {
		t.Errorf("expected 0 consecutive rounds after write, got %d", s.ConsecutiveRoundsNoWrite)
	}
}

func TestMemoryModified(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	if s.MemoryModified() {
		t.Error("expected false with no modifications")
	}

	s.RecordToolCall("write_file", map[string]any{"path": "/tmp/ws/.agentgo/memory/feedback/something.md"}, false, nil)
	if !s.MemoryModified() {
		t.Error("expected true when memory file written")
	}
}

func TestToHookContext(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	hctx := s.ToHookContext()
	if hctx.SessionID != "sess1" {
		t.Errorf("expected session ID 'sess1', got %q", hctx.SessionID)
	}
	if hctx.Stage != StageInitialGeneration {
		t.Errorf("expected stage initial_generation, got %q", hctx.Stage)
	}
	if hctx.SessionState != s {
		t.Error("expected SessionState pointer to be same")
	}
}

func TestPendingWarnings_AddDrain(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	if len(s.DrainPendingWarnings()) != 0 {
		t.Error("expected no warnings when none added")
	}

	s.AddPendingWarnings([]string{"warning 1", "warning 2"})
	s.AddPendingWarnings([]string{"warning 3"})

	w := s.DrainPendingWarnings()
	if len(w) != 3 {
		t.Fatalf("expected 3 warnings, got %d", len(w))
	}
	if w[0] != "warning 1" || w[1] != "warning 2" || w[2] != "warning 3" {
		t.Errorf("warnings mismatch: %v", w)
	}

	// After drain, buffer should be empty.
	if len(s.DrainPendingWarnings()) != 0 {
		t.Error("expected empty after drain")
	}
}

func TestPendingWarnings_AddEmptyAndNil(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	// nil should not add anything.
	s.AddPendingWarnings(nil)
	if len(s.DrainPendingWarnings()) != 0 {
		t.Error("expected no warnings after adding nil")
	}

	// empty slice should not add anything.
	s.AddPendingWarnings([]string{})
	if len(s.DrainPendingWarnings()) != 0 {
		t.Error("expected no warnings after adding empty slice")
	}
}

func TestDestructiveInProgress_Toggle(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	if s.DestructiveInProgress {
		t.Error("expected DestructiveInProgress to default to false")
	}

	s.DestructiveInProgress = true
	if !s.DestructiveInProgress {
		t.Error("expected DestructiveInProgress to be true after setting")
	}

	s.DestructiveInProgress = false
	if s.DestructiveInProgress {
		t.Error("expected DestructiveInProgress to be false after reset")
	}
}

func TestRecordRoundStart(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	s.RecordToolStartCalled("read_file")
	s.RecordToolStartCalled("write_file")
	if len(s.LastRoundTools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(s.LastRoundTools))
	}

	s.RecordRoundStart()
	if len(s.LastRoundTools) != 0 {
		t.Errorf("expected 0 tools after round start, got %d", len(s.LastRoundTools))
	}
}

// ---------------------------------------------------------------------------
// RoundHadHTMLWrite lifecycle
// ---------------------------------------------------------------------------

func TestRoundHadHTMLWrite_RecordRoundStartResets(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	// Set it true first.
	s.mu.Lock()
	s.RoundHadHTMLWrite = true
	s.mu.Unlock()

	// RecordRoundStart should reset.
	s.RecordRoundStart()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.RoundHadHTMLWrite {
		t.Error("expected RoundHadHTMLWrite to be false after RecordRoundStart")
	}
}

func TestRoundHadHTMLWrite_WriteFileHTML(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	s.RecordToolCall("write_file", map[string]any{"path": "deck.html"}, false, nil)

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.RoundHadHTMLWrite {
		t.Error("expected RoundHadHTMLWrite=true after write_file with HTML path")
	}
}

func TestRoundHadHTMLWrite_WriteFileNonHTML(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	s.RecordToolCall("write_file", map[string]any{"path": "styles.css"}, false, nil)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.RoundHadHTMLWrite {
		t.Error("expected RoundHadHTMLWrite=false after write_file with non-HTML path")
	}
}

func TestRoundHadHTMLWrite_EditFileHTML(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	s.RecordToolCall("edit_file", map[string]any{"path": "deck.html"}, false, nil)

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.RoundHadHTMLWrite {
		t.Error("expected RoundHadHTMLWrite=true after edit_file with HTML path")
	}
}

func TestRoundHadHTMLWrite_ErrorDoesNotSet(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	s.RecordToolCall("write_file", map[string]any{"path": "deck.html"}, true, nil)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.RoundHadHTMLWrite {
		t.Error("expected RoundHadHTMLWrite=false after write_file with error")
	}
}

func TestTrackRoundEnd_ResetsOnHTMLWrite(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	// First: no HTML write → counter should increment.
	s.TrackRoundEnd()
	if s.ConsecutiveRoundsNoWrite != 1 {
		t.Errorf("expected ConsecutiveRoundsNoWrite=1 after round without HTML write, got %d", s.ConsecutiveRoundsNoWrite)
	}

	// Second: HTML write happens.
	s.RecordRoundStart()
	s.RecordToolCall("write_file", map[string]any{"path": "deck.html"}, false, nil)
	s.TrackRoundEnd()
	if s.ConsecutiveRoundsNoWrite != 0 {
		t.Errorf("expected ConsecutiveRoundsNoWrite=0 after round with HTML write, got %d", s.ConsecutiveRoundsNoWrite)
	}
}

func TestTrackRoundEnd_IncrementsConsecutively(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	// Three rounds with no HTML writes.
	for i := 1; i <= 3; i++ {
		s.RecordRoundStart()
		s.TrackRoundEnd()
		if s.ConsecutiveRoundsNoWrite != i {
			t.Errorf("round %d: expected ConsecutiveRoundsNoWrite=%d, got %d", i, i, s.ConsecutiveRoundsNoWrite)
		}
	}
}

func TestTrackRoundEnd_EditsCountAsWrites(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	// edit_file on HTML should count as a write and reset counter.
	s.TrackRoundEnd() // counter = 1
	s.RecordRoundStart()
	s.RecordToolCall("edit_file", map[string]any{"path": "deck.html"}, false, nil)
	s.TrackRoundEnd() // counter should be 0

	if s.ConsecutiveRoundsNoWrite != 0 {
		t.Errorf("expected ConsecutiveRoundsNoWrite=0 after edit_file on HTML, got %d", s.ConsecutiveRoundsNoWrite)
	}
}

// ---------------------------------------------------------------------------
// Thread-safe accessor tests (C1 fix)
// ---------------------------------------------------------------------------

func TestGetToolFailureCount(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	s.RecordToolCall("write_file", map[string]any{"path": "a.html"}, true, nil)
	s.RecordToolCall("write_file", map[string]any{"path": "b.html"}, true, nil)

	if n := s.GetToolFailureCount("write_file"); n != 2 {
		t.Errorf("expected 2 failures, got %d", n)
	}
	if n := s.GetToolFailureCount("read_file"); n != 0 {
		t.Errorf("expected 0 failures for uncalled tool, got %d", n)
	}
}

func TestGetCallHistoryCount(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	args := map[string]any{"path": "test.html"}
	s.RecordToolCall("write_file", args, false, nil)
	s.RecordToolCall("write_file", args, false, nil)

	key := "write_file:" + HashToolArgs(args)
	if n := s.GetCallHistoryCount(key); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
	if n := s.GetCallHistoryCount("nonexistent"); n != 0 {
		t.Errorf("expected 0 for unknown key, got %d", n)
	}
}

func TestGetHTMLPublishCount(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	s.RecordToolCall("write_file", map[string]any{"path": "a.html"}, false, nil)
	s.RecordToolCall("write_file", map[string]any{"path": "b.html"}, false, nil)

	if n := s.GetHTMLPublishCount(); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestGetHTMLPatchCount(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	s.RecordToolCall("edit_file", map[string]any{"path": "a.html"}, false, nil)
	s.RecordToolCall("edit_file", map[string]any{"path": "b.html"}, false, nil)

	if n := s.GetHTMLPatchCount(); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestGetConsecutiveRoundsNoWrite(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	s.TrackRoundEnd()
	s.TrackRoundEnd()

	if n := s.GetConsecutiveRoundsNoWrite(); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

func TestWasFileRead(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)
	s.RecordToolCall("read_file", map[string]any{"path": "index.html"}, false, map[string]any{
		"read_mtime_unix_ns": int64(12345),
	})

	abs := resolveStatePath("/tmp/ws", "index.html")
	mtime, ok := s.WasFileRead(abs)
	if !ok {
		t.Fatal("expected file to be marked as read")
	}
	if mtime != 12345 {
		t.Errorf("expected mtime 12345, got %d", mtime)
	}

	_, ok = s.WasFileRead("/tmp/ws/nonexistent.html")
	if ok {
		t.Error("expected false for unread file")
	}
}

func TestIsSkillLoaded(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	if s.IsSkillLoaded("grapesjs-html-compliance") {
		t.Error("expected false before loading")
	}

	s.RecordToolCall("skill", map[string]any{"operation": "get", "name": "grapesjs-html-compliance"}, false, nil)

	if !s.IsSkillLoaded("grapesjs-html-compliance") {
		t.Error("expected true after loading")
	}
}

func TestGetMaxConsecutiveFailures(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	if n := s.GetMaxConsecutiveFailures(); n != 3 {
		t.Errorf("expected default 3, got %d", n)
	}

	s.MaxConsecutiveFailures = 5
	if n := s.GetMaxConsecutiveFailures(); n != 5 {
		t.Errorf("expected 5 after mutation, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// SIT: Concurrent access safety (C1 verification)
// ---------------------------------------------------------------------------

func TestSessionState_ConcurrentAccess(t *testing.T) {
	s := NewSessionState("sess1", "/tmp/ws", StageInitialGeneration)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			s.RecordToolCall("write_file", map[string]any{"path": "test.html", "content": "x"}, i%3 != 0, nil)
			s.RecordToolCall("read_file", map[string]any{"path": "index.html"}, false, map[string]any{
				"read_mtime_unix_ns": int64(i),
			})
		}
	}()

	// Concurrent reads through accessors while writer goroutine is active.
	for i := 0; i < 1000; i++ {
		_ = s.GetToolFailureCount("write_file")
		_ = s.GetCallHistoryCount("write_file:" + HashToolArgs(map[string]any{"path": "test.html", "content": "x"}))
		_ = s.GetHTMLPublishCount()
		_ = s.GetHTMLPatchCount()
		_ = s.GetConsecutiveRoundsNoWrite()
		_, _ = s.WasFileRead(resolveStatePath("/tmp/ws", "index.html"))
		_ = s.IsSkillLoaded("grapesjs-html-compliance")
		_ = s.GetMaxConsecutiveFailures()
		_ = s.HasLoadedSkill()
		_ = s.AllLastRoundReadOnly()
		_ = s.MemoryModified()
	}

	<-done
}
