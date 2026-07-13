package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotStore_CreateFirstVersion(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	if err := os.WriteFile(htmlPath, []byte("<html><title>Test</title></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewSnapshotStore(ws)
	snapshot := &DesignSnapshot{Title: "Test", SlideCount: 1, SlideHeadings: []string{"Slide 1"}}
	todos := []TodoItemRecord{{Content: "Task 1", Status: "pending", ActiveForm: "Doing task 1"}}

	version, err := store.CreateVersion(htmlPath, "sess-1", "", snapshot, todos)
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected version 1, got %d", version)
	}

	// Check manifest exists.
	manifest, err := store.readManifest()
	if err != nil {
		t.Fatal(err)
	}
	if manifest.CurrentVersion != 1 {
		t.Fatalf("expected manifest current_version=1, got %d", manifest.CurrentVersion)
	}

	// Check version directory exists with all files.
	vDir := store.versionDir(1)
	if _, err := os.Stat(filepath.Join(vDir, "context.json")); err != nil {
		t.Fatalf("context.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vDir, "todolist.json")); err != nil {
		t.Fatalf("todolist.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vDir, "deck.html")); err != nil {
		t.Fatalf("deck.html copy missing: %v", err)
	}
}

func TestSnapshotStore_CreateSecondVersion(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	os.WriteFile(htmlPath, []byte("<html></html>"), 0o644)

	store := NewSnapshotStore(ws)
	snap := &DesignSnapshot{SlideCount: 1}

	v1, _ := store.CreateVersion(htmlPath, "s", "", snap, nil)
	if v1 != 1 {
		t.Fatalf("expected v1=1, got %d", v1)
	}
	v2, err := store.CreateVersion(htmlPath, "s", "", snap, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v2 != 2 {
		t.Fatalf("expected v2=2, got %d", v2)
	}

	manifest, _ := store.readManifest()
	if manifest.CurrentVersion != 2 {
		t.Fatalf("expected current_version=2, got %d", manifest.CurrentVersion)
	}
	if len(manifest.Versions) != 2 {
		t.Fatalf("expected 2 versions in manifest, got %d", len(manifest.Versions))
	}
}

func TestSnapshotStore_LoadLatest_Empty(t *testing.T) {
	ws := t.TempDir()
	store := NewSnapshotStore(ws)
	ctx, err := store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if ctx != nil {
		t.Fatalf("expected nil context for empty store, got %+v", ctx)
	}
}

func TestSnapshotStore_LoadLatest_Populated(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	os.WriteFile(htmlPath, []byte("<html></html>"), 0o644)

	store := NewSnapshotStore(ws)
	snap := &DesignSnapshot{Title: "My Deck", SlideCount: 3}
	store.CreateVersion(htmlPath, "sess-1", "", snap, nil)

	ctx, err := store.LoadLatest()
	if err != nil {
		t.Fatal(err)
	}
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.Version != 1 {
		t.Fatalf("expected version 1, got %d", ctx.Version)
	}
	if ctx.DesignSnapshot.Title != "My Deck" {
		t.Fatalf("expected title 'My Deck', got %q", ctx.DesignSnapshot.Title)
	}
	if ctx.SessionID != "sess-1" {
		t.Fatalf("expected session ID sess-1, got %q", ctx.SessionID)
	}
}

func TestSnapshotStore_LoadTodo_Exists(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	os.WriteFile(htmlPath, []byte("<html></html>"), 0o644)

	store := NewSnapshotStore(ws)
	snap := &DesignSnapshot{SlideCount: 1}
	todos := []TodoItemRecord{{Content: "Task 1", Status: "completed", ActiveForm: "Doing task 1"}}
	store.CreateVersion(htmlPath, "s", "", snap, todos)

	loaded, err := store.LoadTodo(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(loaded))
	}
	if loaded[0].Content != "Task 1" {
		t.Fatalf("expected content 'Task 1', got %q", loaded[0].Content)
	}
	if loaded[0].Status != "completed" {
		t.Fatalf("expected status completed, got %q", loaded[0].Status)
	}
}

func TestSnapshotStore_LoadTodo_NotExists(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	os.WriteFile(htmlPath, []byte("<html></html>"), 0o644)

	store := NewSnapshotStore(ws)
	snap := &DesignSnapshot{SlideCount: 1}
	store.CreateVersion(htmlPath, "s", "", snap, nil) // no todos

	todos, err := store.LoadTodo(1)
	if err != nil {
		t.Fatal(err)
	}
	if todos != nil {
		t.Fatalf("expected nil todos when todolist.json doesn't exist, got %+v", todos)
	}
}

func TestSnapshotStore_DiscoverVersion(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "deck.html")
	os.WriteFile(htmlPath, []byte("<html></html>"), 0o644)

	store := NewSnapshotStore(ws)
	snap := &DesignSnapshot{SlideCount: 1}
	store.CreateVersion(htmlPath, "s", "", snap, nil)

	// Remove manifest to simulate corruption.
	os.Remove(filepath.Join(store.baseDir, "manifest.json"))

	v := store.DiscoverVersion()
	if v != 1 {
		t.Fatalf("expected DiscoverVersion to find version 1, got %d", v)
	}
}

func TestSnapshotStore_LoadTodo_NonexistentVersion(t *testing.T) {
	ws := t.TempDir()
	store := NewSnapshotStore(ws)
	todos, err := store.LoadTodo(999)
	if err != nil {
		t.Fatal(err)
	}
	if todos != nil {
		t.Fatalf("expected nil for nonexistent version, got %+v", todos)
	}
}

func TestSnapshotStore_PersistTodo_EmptyItems(t *testing.T) {
	ws := t.TempDir()
	store := NewSnapshotStore(ws)
	// PersistTodo with empty items is a no-op, should not create directories.
	err := store.PersistTodo(1, nil)
	if err != nil {
		t.Fatalf("expected nil error for empty items, got %v", err)
	}
	// Verify no version directory was created.
	if _, err := os.Stat(store.versionDir(1)); !os.IsNotExist(err) {
		t.Fatal("expected no version directory for empty todo persist")
	}
}

func TestSnapshotStore_VersionDirName(t *testing.T) {
	ws := t.TempDir()
	store := NewSnapshotStore(ws)
	dir := store.versionDir(42)
	if !strings.HasSuffix(dir, "v042") {
		t.Fatalf("expected zero-padded version dir name, got %s", dir)
	}
}
