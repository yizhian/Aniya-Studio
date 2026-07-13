package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// SessionStore
// ---------------------------------------------------------------------------

func TestSessionStore_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)

	err := s.Save("test-session", []byte(`{"hello":"world"}`))
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := s.Load("test-session")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if string(data) != `{"hello":"world"}` {
		t.Errorf("expected '{\"hello\":\"world\"}', got %q", string(data))
	}
}

func TestSessionStore_LoadMissing(t *testing.T) {
	s := NewSessionStore(t.TempDir())
	_, err := s.Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestSessionStore_Overwrite(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)

	s.Save("sess", []byte("v1"))
	s.Save("sess", []byte("v2"))

	data, _ := s.Load("sess")
	if string(data) != "v2" {
		t.Errorf("expected 'v2', got %q", string(data))
	}
}

func TestSessionStore_List(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)

	// Empty list should return nil (not panic).
	infos, err := s.List()
	if err != nil {
		t.Fatalf("List empty failed: %v", err)
	}
	if infos != nil {
		t.Fatal("expected nil for empty list")
	}

	s.Save("sess1", []byte(`{"round":1,"messages":[]}`))
	time.Sleep(10 * time.Millisecond) // ensure mod time difference
	s.Save("sess2", []byte(`{"round":3,"messages":[{},{"role":"user","content":"hi"}]}`))

	infos, err = s.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(infos))
	}

	// Most recently modified should be first.
	if infos[0].ID != "sess2" {
		t.Errorf("expected sess2 first, got %s", infos[0].ID)
	}
	if infos[0].Rounds != 3 {
		t.Errorf("expected 3 rounds for sess2, got %d", infos[0].Rounds)
	}
	if infos[0].Messages != 2 {
		t.Errorf("expected 2 messages for sess2, got %d", infos[0].Messages)
	}
}

func TestSessionStore_List_IgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)

	os.MkdirAll(filepath.Join(dir, "not-a-session"), 0755)
	s.Save("real-one", []byte(`{}`))

	infos, _ := s.List()
	if len(infos) != 1 {
		t.Errorf("expected 1 session, got %d", len(infos))
	}
}

func TestSessionStore_List_IgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)

	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0644)
	s.Save("real", []byte(`{}`))

	infos, _ := s.List()
	if len(infos) != 1 {
		t.Errorf("expected 1 session, got %d", len(infos))
	}
}

func TestSessionStore_List_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)

	// Write corrupt JSON.
	os.WriteFile(filepath.Join(dir, "corrupt.json"), []byte(`{bad}`), 0644)
	// Save a valid one.
	s.Save("valid", []byte(`{}`))

	// Should not error — just skip corrupt.
	infos, err := s.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	// The corrupt file skips, the valid one shows.
	if len(infos) != 1 {
		t.Errorf("expected 1 valid session, got %d", len(infos))
	}
}

// ---------------------------------------------------------------------------
// Dir
// ---------------------------------------------------------------------------

func TestDir_EnvOverride(t *testing.T) {
	old := os.Getenv("AGENTGO_DATA_DIR")
	defer os.Setenv("AGENTGO_DATA_DIR", old)

	os.Setenv("AGENTGO_DATA_DIR", "/custom/data/dir")
	d := Dir()
	if d != "/custom/data/dir" {
		t.Errorf("expected '/custom/data/dir', got %q", d)
	}
}

func TestDir_Default(t *testing.T) {
	old := os.Getenv("AGENTGO_DATA_DIR")
	defer os.Setenv("AGENTGO_DATA_DIR", old)

	os.Unsetenv("AGENTGO_DATA_DIR")
	d := Dir()
	// Should end with .agentgo under current working directory.
	if filepath.Base(d) != ".agentgo" {
		t.Errorf("expected '.agentgo' suffix, got %q", d)
	}
}

func TestSessionStore_EmptyDirParam(t *testing.T) {
	// When dir is "", it should default to Dir()/sessions.
	old := os.Getenv("AGENTGO_DATA_DIR")
	defer os.Setenv("AGENTGO_DATA_DIR", old)
	os.Setenv("AGENTGO_DATA_DIR", t.TempDir())

	s := NewSessionStore("")
	if s.dir == "" {
		t.Fatal("expected non-empty dir from default")
	}

	// Verify we can save and load.
	if err := s.Save("test", []byte(`{}`)); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	data, _ := s.Load("test")
	if string(data) != `{}` {
		t.Errorf("unexpected data: %s", data)
	}
}

// ---------------------------------------------------------------------------
// SessionStore: session info parsing
// ---------------------------------------------------------------------------

func TestSessionStore_List_RoundParsing(t *testing.T) {
	dir := t.TempDir()
	s := NewSessionStore(dir)

	raw := struct {
		Round    int                     `json:"round"`
		Messages []map[string]any        `json:"messages"`
		Version  int                     `json:"version"`
	}{
		Round: 5,
		Messages: []map[string]any{
			{"role": "system", "content": "sys"},
			{"role": "user", "content": "hi"},
			{"role": "assistant", "content": "hello"},
		},
		Version: 2,
	}
	data, _ := json.Marshal(raw)
	s.Save("parsed", data)

	infos, _ := s.List()
	if len(infos) != 1 {
		t.Fatalf("expected 1 session, got %d", len(infos))
	}
	if infos[0].Rounds != 5 {
		t.Errorf("expected 5 rounds, got %d", infos[0].Rounds)
	}
	if infos[0].Messages != 3 {
		t.Errorf("expected 3 messages, got %d", infos[0].Messages)
	}
}

// ---------------------------------------------------------------------------
// FileMemoryStore
// ---------------------------------------------------------------------------

func TestFileMemoryStore_GetBasePath(t *testing.T) {
	store := NewFileMemoryStore()
	base := store.GetBasePath("/workspace/project")
	expected := filepath.Join("/workspace/project", ".agentgo", "memory")
	if base != expected {
		t.Errorf("expected %q, got %q", expected, base)
	}
}

func TestFileMemoryStore_LoadUserMemory_Empty(t *testing.T) {
	store := NewFileMemoryStore()
	content, err := store.LoadUserMemory(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string, got %q", content)
	}
}

func TestFileMemoryStore_LoadUserMemory_WithFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()

	// Create user memory files.
	userDir := filepath.Join(store.GetBasePath(dir), "user")
	os.MkdirAll(userDir, 0755)
	os.WriteFile(filepath.Join(userDir, "preferences.md"), []byte(`---
type: user
name: preferences
updated_at: 2026-04-30T10:00:00Z
summary: Language and style preferences
---

Use Chinese. Prefer oklch colors.`), 0644)
	os.WriteFile(filepath.Join(userDir, "constraints.md"), []byte(`---
type: user
name: constraints
updated_at: 2026-04-30T11:00:00Z
summary: Output constraints
---

Never use animations.`), 0644)

	content, err := store.LoadUserMemory(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "Use Chinese") {
		t.Errorf("expected preferences content, got: %s", content)
	}
	if !strings.Contains(content, "Never use animations") {
		t.Errorf("expected constraints content, got: %s", content)
	}
}

func TestFileMemoryStore_LoadUserMemory_IgnoresDirectories(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()

	userDir := filepath.Join(store.GetBasePath(dir), "user")
	os.MkdirAll(filepath.Join(userDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(userDir, "valid.md"), []byte(`---
type: user
name: valid
updated_at: 2026-04-30T10:00:00Z
summary: Valid memory
---

Valid body.`), 0644)

	content, err := store.LoadUserMemory(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "Valid body") {
		t.Errorf("expected valid memory content, got: %s", content)
	}
}

func TestFileMemoryStore_LoadUserMemory_EmptyBodyFile(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()

	userDir := filepath.Join(store.GetBasePath(dir), "user")
	os.MkdirAll(userDir, 0755)
	// File with only frontmatter, no body.
	os.WriteFile(filepath.Join(userDir, "empty.md"), []byte("---\ntype: user\nname: empty\nupdated_at: 2026-04-30T10:00:00Z\nsummary: Empty body file\n---\n"), 0644)

	content, err := store.LoadUserMemory(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty body files should not add blank lines.
	if strings.Contains(content, "Empty body") {
		t.Error("should not contain frontmatter content in body")
	}
}

func TestFileMemoryStore_WriteMemory(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()

	err := store.WriteMemory(dir, "feedback/no-animation.md", `---
type: feedback
name: no-animation
updated_at: 2026-04-30T10:00:00Z
summary: User banned animations
---

No CSS animations allowed.`)
	if err != nil {
		t.Fatalf("WriteMemory failed: %v", err)
	}

	// Verify file exists and is readable.
	fullPath := filepath.Join(store.GetBasePath(dir), "feedback", "no-animation.md")
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read back failed: %v", err)
	}
	if !strings.Contains(string(data), "No CSS animations allowed") {
		t.Errorf("unexpected file content: %s", string(data))
	}
}


func TestFileMemoryStore_WriteMemory_RejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()

	err := store.WriteMemory(dir, "/etc/passwd", "malicious")
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestFileMemoryStore_WriteMemory_RejectsPathEscape(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()

	err := store.WriteMemory(dir, "../../../etc/passwd", "malicious")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}
func TestFileMemoryStore_LoadRecalled(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()

	// Create memory files.
	base := store.GetBasePath(dir)
	for _, entry := range []struct {
		relPath string
		content string
	}{
		{"design/theme.md", "---\ntype: design\nname: theme\nupdated_at: 2026-04-29T10:00:00Z\nsummary: Dark theme choice\n---\n\nChose dark theme for readability."},
		{"feedback/no-video.md", "---\ntype: feedback\nname: no-video\nupdated_at: 2026-04-30T10:00:00Z\nsummary: User rejected auto-play video\n---\n\nUser rejected auto-play video backgrounds."},
	} {
		os.MkdirAll(filepath.Dir(filepath.Join(base, entry.relPath)), 0755)
		os.WriteFile(filepath.Join(base, entry.relPath), []byte(entry.content), 0644)
	}

	recalled, err := store.LoadRecalled(dir, []string{"design/theme.md", "feedback/no-video.md"})
	if err != nil {
		t.Fatalf("LoadRecalled failed: %v", err)
	}
	if len(recalled) != 2 {
		t.Fatalf("expected 2 recalled, got %d", len(recalled))
	}

	if recalled[0].Type != MemoryTypeDesign {
		t.Errorf("expected design type, got %s", recalled[0].Type)
	}
	if recalled[0].Summary != "Dark theme choice" {
		t.Errorf("expected 'Dark theme choice', got %s", recalled[0].Summary)
	}
	if !strings.Contains(recalled[0].Content, "Chose dark theme") {
		t.Errorf("expected content, got: %s", recalled[0].Content)
	}
	if recalled[0].DaysAgo < 0 {
		t.Errorf("expected non-negative daysAgo, got %d", recalled[0].DaysAgo)
	}
}

func TestFileMemoryStore_LoadRecalled_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()

	// Attempt to read a file outside the memory directory via .. traversal.
	// Create a file outside the memory base.
	outsideFile := filepath.Join(dir, "secret.txt")
	os.WriteFile(outsideFile, []byte("should not be readable"), 0644)

	// Try to escape via relative path.
	_, err := store.LoadRecalled(dir, []string{"../../../secret.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The malicious path should be silently skipped (safeJoin rejects it).
	// No panic, no file read outside base.
}

func TestFileMemoryStore_LoadRecalled_Truncation(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()

	base := store.GetBasePath(dir)
	os.MkdirAll(filepath.Join(base, "design"), 0755)

	// Write a file with body exceeding MaxMemoryBytesPerFile.
	bigBody := strings.Repeat("x", MaxMemoryBytesPerFile+100)
	content := "---\ntype: design\nname: big\nupdated_at: 2026-04-30T10:00:00Z\nsummary: Big file\n---\n\n" + bigBody
	os.WriteFile(filepath.Join(base, "design", "big.md"), []byte(content), 0644)

	recalled, err := store.LoadRecalled(dir, []string{"design/big.md"})
	if err != nil {
		t.Fatalf("LoadRecalled failed: %v", err)
	}
	if len(recalled) != 1 {
		t.Fatalf("expected 1 recalled, got %d", len(recalled))
	}
	if !strings.Contains(recalled[0].Content, "[truncated]") {
		t.Error("expected truncation marker in body")
	}
	if len(recalled[0].Content) > MaxMemoryBytesPerFile+50 {
		t.Errorf("body not properly truncated: len=%d", len(recalled[0].Content))
	}
}

func TestFileMemoryStore_LoadRecalled_CorruptFrontmatterSkip(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()
	base := store.GetBasePath(dir)
	os.MkdirAll(filepath.Join(base, "design"), 0755)

	// File with a valid path but unparseable YAML frontmatter.
	os.WriteFile(filepath.Join(base, "design", "bad.md"), []byte(`---
type: design
{{{ invalid yaml :::
---
Body text.`), 0644)

	// Also create a valid file to ensure the valid one is still returned.
	os.WriteFile(filepath.Join(base, "design", "good.md"), []byte(`---
type: design
name: good
updated_at: 2026-04-30T10:00:00Z
summary: A good file
---

Good content.`), 0644)

	recalled, err := store.LoadRecalled(dir, []string{"design/bad.md", "design/good.md"})
	if err != nil {
		t.Fatalf("LoadRecalled failed: %v", err)
	}
	if len(recalled) != 1 {
		t.Fatalf("expected 1 recalled (bad one skipped), got %d", len(recalled))
	}
	if recalled[0].Path != "design/good.md" {
		t.Errorf("expected good file, got %s", recalled[0].Path)
	}
}

func TestParseFrontmatter_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("---\n: [[ bad yaml\n---\n\nBody text."), 0644)

	_, _, err := parseFrontmatter(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML frontmatter")
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.md")
	os.WriteFile(path, []byte("Just plain text without any frontmatter."), 0644)

	fm, body, err := parseFrontmatter(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Type != "" {
		t.Errorf("expected empty type, got %s", fm.Type)
	}
	if body != "Just plain text without any frontmatter." {
		t.Errorf("expected body unchanged, got: %s", body)
	}
}

func TestParseFrontmatter_FileNotFound(t *testing.T) {
	_, _, err := parseFrontmatter("/nonexistent/path/file.md")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestParseFrontmatter_UnclosedFrontmatter(t *testing.T) {
	// Has opening --- but no closing ---.
	dir := t.TempDir()
	path := filepath.Join(dir, "unclosed.md")
	os.WriteFile(path, []byte("---\ntype: user\n\nBody without closing frontmatter."), 0644)

	fm, body, err := parseFrontmatter(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Type != "" {
		t.Errorf("expected empty type when no closing ---, got %q", fm.Type)
	}
	if body != "---\ntype: user\n\nBody without closing frontmatter." {
		t.Errorf("expected original text as body, got: %s", body)
	}
}

func TestSafeJoin_RejectsAbsolute(t *testing.T) {
	_, err := safeJoin("/base", "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestSafeJoin_RejectsEscape(t *testing.T) {
	base := filepath.Join(t.TempDir(), ".agentgo", "memory")
	_, err := safeJoin(base, "../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path escape")
	}
}

func TestSafeJoin_AllowsNormal(t *testing.T) {
	base := filepath.Join(t.TempDir(), ".agentgo", "memory")
	os.MkdirAll(base, 0755)

	result, err := safeJoin(base, "design/theme.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(base, "design", "theme.md")
	// On macOS /var is a symlink to /private/var, so resolve both.
	expected, _ = filepath.EvalSymlinks(expected)
	result, _ = filepath.EvalSymlinks(result)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSafeJoin_RejectsSymlinkParent_ExistingTarget(t *testing.T) {
	// memory/design/link -> /tmp/outside, and file.md already exists there.
	dir := t.TempDir()
	memBase := filepath.Join(dir, ".agentgo", "memory")
	designDir := filepath.Join(memBase, "design")
	os.MkdirAll(designDir, 0755)

	// Create a directory outside the memory tree.
	outsideDir := filepath.Join(dir, "outside")
	os.MkdirAll(outsideDir, 0755)
	os.WriteFile(filepath.Join(outsideDir, "file.md"), []byte("secret"), 0644)

	// Create symlink: memory/design/link -> outside/
	symlinkPath := filepath.Join(designDir, "link")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Fatal(err)
	}

	_, err := safeJoin(memBase, "design/link/file.md")
	if err == nil {
		t.Fatal("expected escape error for parent symlink with existing target")
	}
}

func TestSafeJoin_RejectsSymlinkParent_NewTarget(t *testing.T) {
	// Same as above but the target file doesn't exist yet.
	dir := t.TempDir()
	memBase := filepath.Join(dir, ".agentgo", "memory")
	designDir := filepath.Join(memBase, "design")
	os.MkdirAll(designDir, 0755)

	outsideDir := filepath.Join(dir, "outside")
	os.MkdirAll(outsideDir, 0755)

	symlinkPath := filepath.Join(designDir, "link")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Fatal(err)
	}

	_, err := safeJoin(memBase, "design/link/newfile.md")
	if err == nil {
		t.Fatal("expected escape error for parent symlink with new target")
	}
}

func TestSafeJoin_SymlinkPointingInsideIsAllowed(t *testing.T) {
	// A symlink that stays within the memory tree should be resolved but allowed.
	dir := t.TempDir()
	memBase := filepath.Join(dir, ".agentgo", "memory")
	designDir := filepath.Join(memBase, "design")
	os.MkdirAll(designDir, 0755)

	themeDir := filepath.Join(memBase, "design", "theme")
	os.MkdirAll(themeDir, 0755)

	// Symlink: memory/design/latest -> memory/design/theme (inside the tree).
	symlinkPath := filepath.Join(designDir, "latest")
	if err := os.Symlink(themeDir, symlinkPath); err != nil {
		t.Fatal(err)
	}

	result, err := safeJoin(memBase, "design/latest/file.md")
	if err != nil {
		t.Fatalf("symlink staying inside memory tree should be allowed: %v", err)
	}
	expected, _ := filepath.EvalSymlinks(filepath.Join(themeDir, "file.md"))
	result, _ = filepath.EvalSymlinks(result)
	if result != expected {
		t.Errorf("expected resolved path %q, got %q", expected, result)
	}
}

// ---------------------------------------------------------------------------
// FileMemoryIndex
// ---------------------------------------------------------------------------

func TestFileMemoryIndex_LoadIndex_Empty(t *testing.T) {
	idx := NewFileMemoryIndex(t.TempDir())
	entries, err := idx.LoadIndex()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil for empty index, got %v", entries)
	}
}

func TestFileMemoryIndex_RebuildAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewFileMemoryStore()
	base := store.GetBasePath(dir)

	// Create memory files across categories.
	createMemoryFile(t, base, "design", "theme.md", "---\ntype: design\nname: theme\nupdated_at: 2026-04-30T10:00:00Z\nsummary: Dark theme decision\n---\n\nBody.")
	createMemoryFile(t, base, "feedback", "no-anim.md", "---\ntype: feedback\nname: no-anim\nupdated_at: 2026-04-30T10:00:00Z\nsummary: No animations\n---\n\nBody.")
	createMemoryFile(t, base, "component", "hero.md", "---\ntype: component\nname: hero\nupdated_at: 2026-04-30T10:00:00Z\nsummary: Hero banner pattern\n---\n\nBody.")

	idx := NewFileMemoryIndex(base)
	if err := idx.RebuildIndex(); err != nil {
		t.Fatalf("RebuildIndex failed: %v", err)
	}

	// Verify MEMORY.md exists.
	indexPath := filepath.Join(base, "MEMORY.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("MEMORY.md not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Memory Index") {
		t.Error("missing header")
	}
	if !strings.Contains(content, "Dark theme decision") {
		t.Error("missing design entry")
	}
	if !strings.Contains(content, "No animations") {
		t.Error("missing feedback entry")
	}
	if !strings.Contains(content, "Hero banner pattern") {
		t.Error("missing component entry")
	}

	// Load and parse.
	entries, err := idx.LoadIndex()
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Type != MemoryTypeComponent || entries[0].Summary != "Hero banner pattern" {
		t.Errorf("unexpected first entry: %s/%s", entries[0].Type, entries[0].Summary)
	}
}

func TestFileMemoryIndex_Rebuild_EmptyDirectories(t *testing.T) {
	base := filepath.Join(t.TempDir(), ".agentgo", "memory")
	idx := NewFileMemoryIndex(base)

	// No memory files at all — should succeed with just the header.
	if err := idx.RebuildIndex(); err != nil {
		t.Fatalf("RebuildIndex on empty dirs failed: %v", err)
	}

	entries, err := idx.LoadIndex()
	if err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestFileMemoryIndex_ParseEntries(t *testing.T) {
	indexContent := `# Memory Index
- design/theme.md: Dark theme decision
- feedback/no-anim.md: No animations
- component/hero.md: Hero banner pattern`

	entries := parseIndex(indexContent)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Path != "design/theme.md" || entries[0].Type != MemoryTypeDesign {
		t.Errorf("unexpected entry: %s/%s", entries[0].Type, entries[0].Path)
	}
	if entries[1].Path != "feedback/no-anim.md" || entries[1].Type != MemoryTypeFeedback {
		t.Errorf("unexpected entry: %s/%s", entries[1].Type, entries[1].Path)
	}
}

func TestFileMemoryIndex_ParseEntries_SkipsInvalid(t *testing.T) {
	indexContent := `# Memory Index
- not-a-valid-entry
- design/theme.md: Valid entry
Some random text
- /no-slash.md: Missing type`

	entries := parseIndex(indexContent)
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(entries))
	}
	if entries[0].Path != "design/theme.md" {
		t.Errorf("expected 'design/theme.md', got %s", entries[0].Path)
	}
}

func createMemoryFile(t *testing.T, base, category, filename, content string) {
	t.Helper()
	dir := filepath.Join(base, category)
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644)
}

// ---------------------------------------------------------------------------
// atomicWrite
// ---------------------------------------------------------------------------

func TestAtomicWrite_Concurrency(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.md")
	initial := []byte("initial")
	if err := os.WriteFile(path, initial, 0644); err != nil {
		t.Fatal(err)
	}

	const workers = 10
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		i := i
		go func() {
			content := []byte(fmt.Sprintf("worker-%d", i))
			errs <- AtomicWrite(path, content)
		}()
	}

	for i := 0; i < workers; i++ {
		if err := <-errs; err != nil {
			t.Errorf("worker %d: %v", i, err)
		}
	}

	// File should exist and contain one of the worker payloads (not corrupt).
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("final read: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("file is empty after concurrent writes")
	}
	// Each write is atomic, so content should be exactly one worker's output,
	// not a corrupted concatenation.
	got := string(data)
	if !strings.HasPrefix(got, "worker-") {
		t.Fatalf("corrupt content after concurrent writes: %q", got)
	}
}

func TestAtomicWrite_NoTempFilesLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "final.md")

	if err := AtomicWrite(path, []byte("hello")); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestAtomicWrite_RejectsSymlinkTarget(t *testing.T) {
	dir := t.TempDir()

	realPath := filepath.Join(dir, "real.md")
	if err := os.WriteFile(realPath, []byte("real"), 0644); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(dir, "link.md")
	if err := os.Symlink("real.md", symlinkPath); err != nil {
		t.Fatal(err)
	}

	err := AtomicWrite(symlinkPath, []byte("should not write"))
	if err == nil {
		t.Fatal("expected error when target is symlink")
	}
	content, _ := os.ReadFile(realPath)
	if string(content) != "real" {
		t.Errorf("real file was modified through symlink: %q", content)
	}
}
