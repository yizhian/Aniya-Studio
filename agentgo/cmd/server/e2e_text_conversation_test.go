package main

import (
	"strings"
	"testing"

	p "agentgo/internal/provider"
)

// TestE2E_TextConversation verifies a simple text-only conversation flow.
func TestE2E_TextConversation(t *testing.T) {
	script := p.SingleTurnScript(
		p.TextFrame("Hello! How can I help you today?"),
		p.DoneFrame(),
	)
	h := newE2EHarness(t, script)
	defer h.Close()

	// Send a message.
	events := h.Chat("test-session", "Hi there!")
	if len(events) == 0 {
		t.Fatal("expected at least one SSE event")
	}

	// Verify we get text content.
	hasContent := false
	for _, ev := range events {
		if ev.Type == "text" {
			hasContent = true
			if data, ok := ev.Data["delta"].(string); ok {
				if !strings.Contains(data, "Hello") {
					t.Errorf("expected 'Hello' in text, got %q", data)
				}
			}
		}
	}
	if !hasContent {
		t.Error("expected text events")
	}

	// Verify session is persisted.
	sessions := h.ListSessions()
	if len(sessions) == 0 {
		t.Fatal("expected at least one session in history")
	}
	found := false
	for _, s := range sessions {
		if s.ID == "test-session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected test-session in history list")
	}

	// Verify session data can be loaded.
	sessionData, err := h.GetSession("test-session")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if sessionData == nil {
		t.Fatal("expected session data")
	}
}

// TestE2E_AutoSessionID verifies that a session ID is auto-generated.
func TestE2E_AutoSessionID(t *testing.T) {
	script := p.SingleTurnScript(
		p.TextFrame("Hello!"),
		p.DoneFrame(),
	)
	h := newE2EHarness(t, script)
	defer h.Close()

	events, sessID := h.ChatAutoSession("Hello")
	if sessID == "" {
		t.Fatal("expected auto-generated session ID")
	}
	if !strings.HasPrefix(sessID, "sess_") {
		t.Errorf("expected session ID to start with 'sess_', got %q", sessID)
	}
	_ = events
}

// TestE2E_MultiTurnConversation verifies history is preserved across turns.
func TestE2E_MultiTurnConversation(t *testing.T) {
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{p.TextFrame("Nice to meet you, Xiao Ming!"), p.DoneFrame()},
			{p.TextFrame("Your name is Xiao Ming."), p.DoneFrame()},
		},
	}
	h := newE2EHarness(t, script)
	defer h.Close()

	// First turn.
	h.Chat("multi-session", "My name is Xiao Ming.")

	// Second turn — verify session is loaded and history is used.
	events := h.Chat("multi-session", "What is my name?")
	if len(events) == 0 {
		t.Fatal("expected events in second turn")
	}

	// Verify session data includes both rounds.
	data, err := h.GetSession("multi-session")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	messages, _ := data["messages"].([]any)
	if messages == nil {
		t.Fatal("expected messages in session")
	}
	// We should have more messages in round 2 than round 1.
	if len(messages) < 3 {
		t.Errorf("expected at least 3 messages, got %d", len(messages))
	}
}

// TestE2E_HealthEndpoint tests the health endpoint through the full server.
func TestE2E_Health(t *testing.T) {
	script := p.SingleTurnScript(p.TextFrame("ok"), p.DoneFrame())
	h := newE2EHarness(t, script)
	defer h.Close()

	if !h.Health() {
		t.Fatal("health check failed")
	}
}
