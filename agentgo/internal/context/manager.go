package context

import (
	"fmt"

	"agentgo/internal/model"
	"agentgo/internal/observability"
	"agentgo/internal/persistence"
)

// ContextManager orchestrates context assembly, versioning, and todo persistence
// across an agent loop session. Created once per chat turn.
type ContextManager struct {
	store          *SnapshotStore
	sessionID      string
	currentVersion int
	latestSnapshot *DesignSnapshot
	latestTodos    []TodoItemRecord
	htmlFilePath   string
	activeFile     string
	keepRounds     int
	recalled       []persistence.RecalledMemory
	emitter        *observability.Emitter
}

// NewContextManager creates a ContextManager for a session.
func NewContextManager(workspaceDir, sessionID, activeFile string) *ContextManager {
	store := NewSnapshotStore(workspaceDir)

	ctx, err := store.LoadLatest()
	if err != nil || ctx == nil {
		var todos []TodoItemRecord
		if t, err := store.LoadSessionTodo(sessionID); err == nil {
			todos = t
		}
		return &ContextManager{
			store:          store,
			sessionID:      sessionID,
			currentVersion: 0,
			latestTodos:    todos,
			activeFile:     activeFile,
			keepRounds:     KeepRounds,
		}
	}

	var todos []TodoItemRecord
	if t, err := store.LoadTodo(ctx.Version); err == nil {
		todos = t
	} else if t, err := store.LoadSessionTodo(sessionID); err == nil {
		todos = t
	}

	return &ContextManager{
		store:          store,
		sessionID:      sessionID,
		currentVersion: ctx.Version,
		latestSnapshot: &ctx.DesignSnapshot,
		latestTodos:    todos,
		htmlFilePath:   ctx.HTMLPath,
		activeFile:     activeFile,
		keepRounds:     KeepRounds,
	}
}

// CurrentVersion returns the latest version number (0 if none).
func (m *ContextManager) CurrentVersion() int {
	return m.currentVersion
}

// HTMLFilePath returns the path to the primary HTML file being edited.
func (m *ContextManager) HTMLFilePath() string {
	return m.htmlFilePath
}

// ClassifyStage determines the current conversation stage for hook filtering.
func (m *ContextManager) ClassifyStage() model.Stage {
	if m.htmlFilePath == "" && m.currentVersion == 0 {
		return model.StageInitialGeneration
	}
	if m.htmlFilePath != "" && m.latestSnapshot != nil {
		return model.StageIterativeEdit
	}
	return model.StageInitialGeneration
}

// SessionID returns the session identifier.
func (m *ContextManager) SessionID() string {
	return m.sessionID
}

// VersionDir returns the version directory path for the current version (empty if no version).
func (m *ContextManager) VersionDir() string {
	if m.currentVersion == 0 {
		return ""
	}
	return m.store.VersionDir(m.currentVersion)
}

// LatestSnapshot returns the cached design snapshot (nil if none).
func (m *ContextManager) LatestSnapshot() *DesignSnapshot {
	return m.latestSnapshot
}

// LatestTodos returns the cached todo items (nil if none).
func (m *ContextManager) LatestTodos() []TodoItemRecord {
	return m.latestTodos
}

// SetEmitter sets an optional observability emitter for context operation events.
func (m *ContextManager) SetEmitter(emitter *observability.Emitter) {
	m.emitter = emitter
}

// SetRecalledMemories sets the memories to inject into the assembled context.
func (m *ContextManager) SetRecalledMemories(recalled []persistence.RecalledMemory) {
	m.recalled = recalled
}

// UpdateTodos persists the current todo list to session-level storage
// and to the version directory when a version exists.
func (m *ContextManager) UpdateTodos(todos []TodoItemRecord) error {
	m.latestTodos = todos

	if err := m.store.PersistSessionTodo(m.sessionID, todos); err != nil {
		return fmt.Errorf("persist session todo: %w", err)
	}

	if m.currentVersion > 0 {
		if err := m.store.PersistTodo(m.currentVersion, todos); err != nil {
			return fmt.Errorf("persist version todo: %w", err)
		}
	}
	return nil
}

// KeepRounds returns the configured number of rounds to retain.
func (m *ContextManager) KeepRounds() int {
	return m.keepRounds
}

// SetKeepRounds overrides the default round retention count.
func (m *ContextManager) SetKeepRounds(n int) {
	if n > 0 {
		m.keepRounds = n
	}
}
