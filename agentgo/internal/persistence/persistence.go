// Package persistence saves sessions, memory, and generated files as JSON/text
// on the local filesystem. No database required.
//
// This package does NOT import any agentgo internal packages to avoid circular
// dependencies. The agent level handles marshaling before calling Save.
package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Dir returns the default persistence directory.
//
// Priority:
// 1) AGENTGO_DATA_DIR (explicit override)
// 2) ./.agentgo under current working directory (project-local by default)
// 3) ~/.agentgo (fallback only if cwd cannot be resolved)
func Dir() string {
	if d := os.Getenv("AGENTGO_DATA_DIR"); d != "" {
		return d
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		return filepath.Join(wd, ".agentgo")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agentgo")
}

// ---------------------------------------------------------------------------
// SessionStore
// ---------------------------------------------------------------------------

// SessionStore persists conversation history as JSON files.
type SessionStore struct {
	dir     string
	emitter interface{} // *observability.Emitter, typed as interface{} to avoid circular import
}

// SetEmitter sets an optional observability emitter for persistence events.
// The parameter is typed as interface{} to avoid a circular import.
func (s *SessionStore) SetEmitter(emitter interface{}) {
	s.emitter = emitter
}

func NewSessionStore(dir string) *SessionStore {
	if dir == "" {
		dir = filepath.Join(Dir(), "sessions")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		logMkdirError(dir, err)
		dir = filepath.Join(Dir(), "sessions")
		os.MkdirAll(dir, 0755)
	}
	return &SessionStore{dir: dir}
}

// Save writes raw JSON to a session file atomically.
func (s *SessionStore) Save(id string, data []byte) error {
	path := filepath.Join(s.dir, id+".json")
	return AtomicWrite(path, data)
}

// Load reads raw JSON from a session file.
func (s *SessionStore) Load(id string) ([]byte, error) {
	path := filepath.Join(s.dir, id+".json")
	return os.ReadFile(path)
}

// SessionInfo is a lightweight summary for listing sessions.
type SessionInfo struct {
	ID        string    `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
	Rounds    int       `json:"rounds"`
	Messages  int       `json:"messages"`
}

// List returns metadata for all saved sessions, newest first.
func (s *SessionStore) List() ([]SessionInfo, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("session list: %w", err)
	}
	var infos []SessionInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-5]
		info, err := e.Info()
		if err != nil {
			continue
		}
		data, err := s.Load(id)
		if err != nil {
			logSessionCorrupt(fmt.Sprintf("session list: load %s: %v", id, err), s.emitter)
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			logSessionCorrupt(fmt.Sprintf("session list: unmarshal %s: %v", id, err), s.emitter)
			continue
		}
		rounds, _ := raw["round"].(float64)
		msgs, _ := raw["messages"].([]any)
		infos = append(infos, SessionInfo{
			ID:        id,
			UpdatedAt: info.ModTime(),
			Rounds:    int(rounds),
			Messages:  len(msgs),
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].UpdatedAt.After(infos[j].UpdatedAt)
	})
	return infos, nil
}

// ---------------------------------------------------------------------------
// MemoryStore (interface + legacy JSON implementation)
// ---------------------------------------------------------------------------

// MemoryStore is the interface for persisting and retrieving agent memory.
// FileMemoryStore (directory tree) is the primary implementation.
type MemoryStore interface {
	// LoadUserMemory returns all user-scoped memories concatenated.
	LoadUserMemory(workspacePath string) (string, error)
	// WriteMemory atomically writes a memory file at the given path under the memory base.
	WriteMemory(workspacePath, relPath, content string) error
	// LoadRecalled reads and parses the memory files at the given relative paths.
	LoadRecalled(workspacePath string, paths []string) ([]RecalledMemory, error)
	// GetBasePath returns the absolute memory directory for the given workspace.
	GetBasePath(workspacePath string) string
}



// logMkdirError logs MkdirAll failures to stderr.
func logMkdirError(dir string, err error) {
	fmt.Fprintf(os.Stderr, `{"ts":"%s","type":"persistence:mkdir_error","dir":"%s","error":"%s"}`+"\n",
		time.Now().Format(time.RFC3339), dir, err.Error())
}

// logSessionCorrupt logs corrupt/inaccessible session data.
func logSessionCorrupt(msg string, emitter interface{}) {
	fmt.Fprintf(os.Stderr, `{"ts":"%s","type":"session:corrupt","message":"%s"}`+"\n",
		time.Now().Format(time.RFC3339), msg)
}
