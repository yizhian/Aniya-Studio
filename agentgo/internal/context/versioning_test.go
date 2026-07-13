package context

import (
	"os"
	"testing"

	"agentgo/internal/model"
)

func TestRevertVersion_RemovesCorruptedVersion(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte(`<html><head><title>V1</title></head><body>
		<section class="slide"><h1>S1</h1></section>
	</body></html>`), 0644)

	store := NewSnapshotStore(ws)
	snap, _ := ExtractDesignSnapshot("deck.html")

	// Create version 1.
	v1, err := store.CreateVersion("deck.html", "sess1", "V1", snap, nil)
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}
	if v1 != 1 {
		t.Fatalf("expected v1=1, got %d", v1)
	}

	// Load it through ContextManager.
	cm := NewContextManager(ws, "sess1", "deck.html")
	if cm.CurrentVersion() != 1 {
		t.Fatalf("expected CurrentVersion=1, got %d", cm.CurrentVersion())
	}

	// Revert version 1 — should mark invalid and roll back currentVersion.
	err = cm.RevertVersion(1)
	if err != nil {
		t.Fatalf("RevertVersion failed: %v", err)
	}
	// After revert, currentVersion should be 0 (no other versions exist).
	if cm.CurrentVersion() != 0 {
		t.Errorf("expected CurrentVersion=0 after revert, got %d", cm.CurrentVersion())
	}
}

func TestRevertVersion_RollsToPreviousVersion(t *testing.T) {
	ws := chdirTemp(t)
	os.WriteFile("deck.html", []byte(`<html><head><title>V2</title></head><body>
		<section class="slide"><h1>S2</h1></section>
	</body></html>`), 0644)

	store := NewSnapshotStore(ws)
	snap, _ := ExtractDesignSnapshot("deck.html")

	// Create version 1.
	store.CreateVersion("deck.html", "sess1", "V1", snap, nil)
	// Create version 2.
	store.CreateVersion("deck.html", "sess1", "V2", snap, nil)

	cm := NewContextManager(ws, "sess1", "deck.html")
	if cm.CurrentVersion() != 2 {
		t.Fatalf("expected CurrentVersion=2, got %d", cm.CurrentVersion())
	}

	// Revert version 2 — should roll back to version 1.
	err := cm.RevertVersion(2)
	if err != nil {
		t.Fatalf("RevertVersion failed: %v", err)
	}
	if cm.CurrentVersion() != 1 {
		t.Errorf("expected CurrentVersion=1 after reverting v2, got %d", cm.CurrentVersion())
	}
}

func TestCompressMessages_WithSystemPrompt(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "sess1", "")

	messages := []model.Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Round 1"},
		{Role: "assistant", Content: "Response 1"},
		{Role: "user", Content: "Round 2"},
		{Role: "assistant", Content: "Response 2"},
		{Role: "user", Content: "Round 3"},
		{Role: "assistant", Content: "Response 3"},
		{Role: "user", Content: "Round 4"},
		{Role: "assistant", Content: "Response 4"},
		{Role: "user", Content: "Round 5"},
		{Role: "assistant", Content: "Response 5"},
	}

	result := cm.CompressMessages(messages, "Compression summary")

	// First message should be the system prompt.
	if result[0].Role != "system" {
		t.Errorf("expected first message to be system, got %s", result[0].Role)
	}
	// Second message should be the compression summary.
	if result[1].Role != "user" || result[1].Content != "Compression summary" {
		t.Errorf("expected second message to be compression summary, got %s", result[1].Content)
	}
	// No duplicate system messages after compression.
	for _, msg := range result[1:] {
		if msg.Role == "system" {
			t.Error("expected no system messages after compression summary")
		}
	}
}

func TestCompressMessages_NoSystemPrompt(t *testing.T) {
	ws := chdirTemp(t)
	cm := NewContextManager(ws, "sess1", "")

	messages := []model.Message{
		{Role: "user", Content: "Round 1"},
		{Role: "assistant", Content: "Response 1"},
	}

	result := cm.CompressMessages(messages, "Summary")
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	// First message should be the compression summary (user role).
	if result[0].Role != "user" || result[0].Content != "Summary" {
		t.Errorf("expected first message to be compression summary (user role), got %s: %s", result[0].Role, result[0].Content)
	}
}
