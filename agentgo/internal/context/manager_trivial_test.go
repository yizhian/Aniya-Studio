package context

import (
	"os"
	"testing"

	"agentgo/internal/hook"
)

func TestContextManager_HTMLFilePath(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte(`<html><head><title>Test</title></head><body>
		<section class="slide"><h1>S1</h1></section>
	</body></html>`), 0644)

	cm := NewContextManager(ws, "sess", "")

	// Initially empty — no HTML file detected yet.
	if cm.HTMLFilePath() != "" {
		t.Errorf("expected empty HTMLFilePath initially, got %q", cm.HTMLFilePath())
	}
}

func TestContextManager_SessionID(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "my-session-123", "")
	if cm.SessionID() != "my-session-123" {
		t.Errorf("expected 'my-session-123', got %q", cm.SessionID())
	}
}

func TestContextManager_ClassifyStage_InitialGeneration(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "sess", "")
	stage := cm.ClassifyStage()
	if stage != hook.StageInitialGeneration {
		t.Errorf("expected StageInitialGeneration for fresh workspace, got %q", stage)
	}
}

func TestContextManager_ClassifyStage_IterativeEdit(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte(`<html><head><title>Test</title></head><body>
		<section class="slide"><h1>S1</h1></section>
	</body></html>`), 0644)

	// Create a version so the context manager finds htmlFilePath + snapshot.
	store := NewSnapshotStore(ws)
	snap, _ := ExtractDesignSnapshot("deck.html")
	_, err := store.CreateVersion("deck.html", "sess1", "My Deck", snap, nil)
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}

	cm := NewContextManager(ws, "sess1", "deck.html")
	stage := cm.ClassifyStage()
	if stage != hook.StageIterativeEdit {
		t.Errorf("expected StageIterativeEdit for workspace with version, got %q", stage)
	}
}

func TestContextManager_VersionDir_Zero(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "sess", "")
	if cm.VersionDir() != "" {
		t.Errorf("expected empty VersionDir for version 0, got %q", cm.VersionDir())
	}
}

func TestContextManager_VersionDir_NonZero(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte(`<html><head><title>Test</title></head><body>
		<section class="slide"><h1>S1</h1></section>
	</body></html>`), 0644)

	store := NewSnapshotStore(ws)
	snap, _ := ExtractDesignSnapshot("deck.html")
	_, err := store.CreateVersion("deck.html", "sess1", "My Deck", snap, nil)
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}

	cm := NewContextManager(ws, "sess1", "deck.html")
	vdir := cm.VersionDir()
	if vdir == "" {
		t.Error("expected non-empty VersionDir for version > 0")
	}
}

func TestContextManager_LatestTodos_InitiallyNil(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "sess", "")
	if cm.LatestTodos() != nil {
		t.Errorf("expected nil LatestTodos initially, got %v", cm.LatestTodos())
	}
}

func TestContextManager_CurrentVersion_InitiallyZero(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "sess", "")
	if cm.CurrentVersion() != 0 {
		t.Errorf("expected CurrentVersion=0, got %d", cm.CurrentVersion())
	}
}

func TestContextManager_LatestSnapshot_InitiallyNil(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "sess", "")
	if cm.LatestSnapshot() != nil {
		t.Errorf("expected nil LatestSnapshot initially, got %v", cm.LatestSnapshot())
	}
}

