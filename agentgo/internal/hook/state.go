package hook

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// SessionState is the cross-tool-call memory that Hook decisions depend on.
// It is automatically maintained by the Hook engine at post_tool_use time
// and persisted across rounds via StreamingLoopConfig.
type SessionState struct {
	mu sync.RWMutex

	// Stage and identity
	Stage         Stage
	SessionID     string
	WorkspacePath string

	// Tool call tracking
	ToolsCalled map[string]int // tool name → invocation count
	ToolsFailed map[string]int // tool name → consecutive failures (reset on success)
	CallHistory map[string]int // "toolName:argHash" → count (for duplicate detection)

	// File state tracking
	FilesRead    map[string]int64 // absolute path → mtime_unix_ns (recorded on read)
	FilesWritten map[string]bool  // absolute path → written/edited this session
	HTMLWritten  bool             // any .html file written this session

	// Skill tracking
	SkillsLoaded   map[string]bool // skill name → loaded via skill get
	SkillGetCalled bool

	// HTML write tracking — used by ComplianceReviewTrigger for degradation.
	// Publish = write_file (full deployment), Patch = edit_file (incremental fix).
	HTMLPublishCount int
	HTMLPatchCount   int

	// Todo tracking
	HasUsedTodoWrite bool
	LastTodoCount    int

	// Execution guards
	DestructiveInProgress      bool
	LastRoundTools             []string // tool names called in the most recent round
	RoundHadHTMLWrite          bool     // at least one HTML write_file/edit_file this round
	ConsecutiveRoundsNoWrite   int

	// Quality feedback — warnings accumulated from post_tool_use checks,
	// drained at PreContextAssemble and injected into the LLM context.
	PendingWarnings []string

	// Configuration
	MaxConsecutiveFailures int
	PlanningThreshold      int
}

// NewSessionState creates a fresh SessionState with default thresholds.
func NewSessionState(sessionID, workspacePath string, stage Stage) *SessionState {
	return &SessionState{
		Stage:          stage,
		SessionID:      sessionID,
		WorkspacePath:  workspacePath,
		ToolsCalled:    make(map[string]int),
		ToolsFailed:    make(map[string]int),
		CallHistory:    make(map[string]int),
		FilesRead:      make(map[string]int64),
		FilesWritten:   make(map[string]bool),
		SkillsLoaded:   make(map[string]bool),
		MaxConsecutiveFailures: 3,
		PlanningThreshold:      3,
	}
}

// HasLoadedSkill reports whether any skill has been loaded via skill get.
func (s *SessionState) HasLoadedSkill() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.SkillGetCalled {
		return true
	}
	return len(s.SkillsLoaded) > 0
}

// RecordToolCall updates session state based on a completed tool call.
// Called automatically by the engine at post_tool_use time.
func (s *SessionState) RecordToolCall(toolName string, args map[string]any, isError bool, metadata map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. General tracking
	s.ToolsCalled[toolName]++

	// 2. Per-tool state updates
	switch toolName {
	case "read_file":
		if !isError {
			path, _ := args["path"].(string)
			if path != "" {
				absPath := resolveStatePath(s.WorkspacePath, path)
				var mtime int64
				if metadata != nil {
					if m, ok := metadata["read_mtime_unix_ns"].(int64); ok {
						mtime = m
					}
				}
				s.FilesRead[absPath] = mtime
			}
		}
	case "write_file":
		if !isError {
			path, _ := args["path"].(string)
			if path != "" {
				absPath := resolveStatePath(s.WorkspacePath, path)
				s.FilesWritten[absPath] = true
				if IsHTMLPath(path) {
					s.HTMLWritten = true
					s.HTMLPublishCount++
					s.RoundHadHTMLWrite = true
				}
			}
		}
	case "edit_file":
		if !isError {
			path, _ := args["path"].(string)
			if path != "" {
				absPath := resolveStatePath(s.WorkspacePath, path)
				s.FilesWritten[absPath] = true
				if IsHTMLPath(path) {
					s.HTMLWritten = true
					s.HTMLPatchCount++
					s.RoundHadHTMLWrite = true
				}
			}
		}
	case "skill":
		op, _ := args["operation"].(string)
		name, _ := args["name"].(string)
		switch op {
		case "get":
			s.SkillGetCalled = true
			if name != "" && !isError {
				s.SkillsLoaded[name] = true
			}
		}
	case "todo_write":
		s.HasUsedTodoWrite = true
		if !isError {
			if metadata != nil {
				if count, ok := metadata["count"].(int); ok {
					s.LastTodoCount = count
				}
			}
		}
	}

	// 3. Failure tracking
	if isError {
		s.ToolsFailed[toolName]++
	} else {
		s.ToolsFailed[toolName] = 0
	}

	// 4. Call history (duplicate detection)
	key := toolName + ":" + HashToolArgs(args)
	s.CallHistory[key]++
}

// RecordRoundStart resets per-round state at the beginning of a new round.
func (s *SessionState) RecordRoundStart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastRoundTools = nil
	s.RoundHadHTMLWrite = false
}

// RecordToolStartCalled marks a tool as being called this round.
func (s *SessionState) RecordToolStartCalled(toolName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastRoundTools = append(s.LastRoundTools, toolName)
}

// AllLastRoundReadOnly reports whether all tools called in the last round were read-only.
func (s *SessionState) AllLastRoundReadOnly() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.LastRoundTools) == 0 {
		return false
	}
	readOnlySet := map[string]bool{
		"read_file": true, "list_files": true, "grep_search": true,
		"web_fetch": true, "tool_search": true, "skill": true,
	}
	for _, name := range s.LastRoundTools {
		if !readOnlySet[name] {
			return false
		}
	}
	return true
}

// ToHookContext creates a HookContext populated from this SessionState.
func (s *SessionState) ToHookContext() *HookContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &HookContext{
		SessionID:     s.SessionID,
		WorkspacePath: s.WorkspacePath,
		Stage:         s.Stage,
		SessionState:  s,
	}
}

// resolveStatePath resolves a potentially relative tool path to an absolute
// workspace-relative key suitable for session state maps.
func resolveStatePath(workspacePath, toolPath string) string {
	clean := filepath.Clean(toolPath)
	if filepath.IsAbs(clean) {
		return clean
	}
	return filepath.Join(workspacePath, clean)
}

// HashToolArgs produces a short stable hash for duplicate-call detection.
func HashToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return "0"
	}
	data, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("%d", len(args))
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

// TrackRoundEnd is called after a round to update round-level metrics.
func (s *SessionState) TrackRoundEnd() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.RoundHadHTMLWrite {
		s.ConsecutiveRoundsNoWrite++
	} else {
		s.ConsecutiveRoundsNoWrite = 0
	}
}

// AddPendingWarnings appends quality warnings to be injected at the next PreContextAssemble.
func (s *SessionState) AddPendingWarnings(warnings []string) {
	if len(warnings) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PendingWarnings = append(s.PendingWarnings, warnings...)
}

// DrainPendingWarnings returns and clears all accumulated quality warnings.
func (s *SessionState) DrainPendingWarnings() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.PendingWarnings) == 0 {
		return nil
	}
	w := s.PendingWarnings
	s.PendingWarnings = nil
	return w
}

// MemoryModified is derived from file operations targeting .agentgo/memory/ paths.
func (s *SessionState) MemoryModified() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for path := range s.FilesWritten {
		if strings.Contains(path, "/.agentgo/memory/") {
			return true
		}
	}
	return false
}

// GetToolFailureCount returns the consecutive failure count for a tool (thread-safe).
func (s *SessionState) GetToolFailureCount(toolName string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ToolsFailed[toolName]
}

// GetMaxConsecutiveFailures returns the configured failure threshold (thread-safe).
func (s *SessionState) GetMaxConsecutiveFailures() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MaxConsecutiveFailures
}

// GetCallHistoryCount returns the invocation count for a tool+args key (thread-safe).
func (s *SessionState) GetCallHistoryCount(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CallHistory[key]
}

// GetHTMLPublishCount returns the HTMLPublishCount (thread-safe).
func (s *SessionState) GetHTMLPublishCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.HTMLPublishCount
}

// GetHTMLPatchCount returns the HTMLPatchCount (thread-safe).
func (s *SessionState) GetHTMLPatchCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.HTMLPatchCount
}

// GetConsecutiveRoundsNoWrite returns ConsecutiveRoundsNoWrite (thread-safe).
func (s *SessionState) GetConsecutiveRoundsNoWrite() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ConsecutiveRoundsNoWrite
}

// WasFileRead returns whether a file was read and its mtime (thread-safe).
func (s *SessionState) WasFileRead(absPath string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mtime, ok := s.FilesRead[absPath]
	return mtime, ok
}

// IsSkillLoaded reports whether a named skill has been loaded (thread-safe).
func (s *SessionState) IsSkillLoaded(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SkillsLoaded[name]
}
