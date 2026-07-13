package context

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agentgo/internal/persistence"
)

// DefaultSlidecraftDir is the hidden directory under the workspace for all version data.
const DefaultSlidecraftDir = ".slidecraft"

// SnapshotStore handles reading and writing version snapshots under
// .slidecraft/versions/vN/ in the workspace directory.
type SnapshotStore struct {
	baseDir string // e.g., "/workspace/.slidecraft"
}

// NewSnapshotStore creates a SnapshotStore rooted at the workspace directory.
func NewSnapshotStore(workspaceDir string) *SnapshotStore {
	return &SnapshotStore{
		baseDir: filepath.Join(workspaceDir, DefaultSlidecraftDir),
	}
}

// BaseDir returns the .slidecraft directory path.
func (s *SnapshotStore) BaseDir() string {
	return s.baseDir
}

// versionDir returns the path to a specific version directory.
func (s *SnapshotStore) versionDir(v int) string {
	return filepath.Join(s.baseDir, "versions", fmt.Sprintf("v%03d", v))
}

// LoadLatest loads the most recent ContextJSON by reading manifest.json.
// Returns nil, nil if no versions exist yet.
func (s *SnapshotStore) LoadLatest() (*ContextJSON, error) {
	manifest, err := s.readManifest()
	if err != nil || manifest == nil {
		return nil, nil
	}
	return s.LoadVersion(manifest.CurrentVersion)
}

// LoadVersion loads a specific version's context.json.
func (s *SnapshotStore) LoadVersion(version int) (*ContextJSON, error) {
	ctxPath := filepath.Join(s.versionDir(version), "context.json")
	data, err := os.ReadFile(ctxPath)
	if err != nil {
		return nil, fmt.Errorf("read context.json v%d: %w", version, err)
	}
	var ctx ContextJSON
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("unmarshal context.json v%d: %w", version, err)
	}
	return &ctx, nil
}

// LoadTodo loads the todolist.json for a specific version.
func (s *SnapshotStore) LoadTodo(version int) ([]TodoItemRecord, error) {
	todoPath := filepath.Join(s.versionDir(version), "todolist.json")
	data, err := os.ReadFile(todoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read todolist.json v%d: %w", version, err)
	}
	var todos []TodoItemRecord
	if err := json.Unmarshal(data, &todos); err != nil {
		return nil, fmt.Errorf("unmarshal todolist.json v%d: %w", version, err)
	}
	return todos, nil
}

// CreateVersion creates a new version directory, copies the HTML file into it,
// generates and writes context.json, and updates the manifest.
func (s *SnapshotStore) CreateVersion(htmlPath string, sessionID string, title string, snapshot *DesignSnapshot, todos []TodoItemRecord) (int, error) {
	manifest, _ := s.readManifest()
	newVersion := 1
	if manifest != nil {
		newVersion = manifest.CurrentVersion + 1
	}

	// If this is the first version, record the primary HTML file.
	htmlFile := filepath.Base(htmlPath)
	if manifest == nil {
		manifest = &VersionManifest{HTMLFile: htmlFile}
	}
	manifest.CurrentVersion = newVersion
	manifest.Versions = append(manifest.Versions, Version{
		Number:    newVersion,
		CreatedAt: time.Now().UTC(),
		HTMLFile:  htmlFile,
	})

	vDir := s.versionDir(newVersion)
	if err := os.MkdirAll(vDir, 0o755); err != nil {
		return 0, fmt.Errorf("create version dir: %w", err)
	}

	// Copy HTML into the version directory.
	if err := copyFile(htmlPath, filepath.Join(vDir, htmlFile)); err != nil {
		return 0, fmt.Errorf("copy html: %w", err)
	}

	// Write context.json.
	if title == "" {
		title = snapshot.Title
	}
	if title == "" && len(snapshot.SlideHeadings) > 0 {
		title = snapshot.SlideHeadings[0]
	}
	if title == "" {
		title = fmt.Sprintf("Version %d", newVersion)
	}
	ctx := ContextJSON{
		Version:        newVersion,
		CreatedAt:      time.Now().UTC(),
		SessionID:      sessionID,
		HTMLPath:       htmlPath,
		Title:          title,
		DesignSnapshot: *snapshot,
		Todos:          todos,
	}
	ctxData, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("marshal context: %w", err)
	}
	if err := persistence.AtomicWrite(filepath.Join(vDir, "context.json"), ctxData); err != nil {
		return 0, fmt.Errorf("write context.json: %w", err)
	}

	// Write todolist.json.
	if todos != nil {
		todoData, err := json.MarshalIndent(todos, "", "  ")
		if err != nil {
			return 0, fmt.Errorf("marshal todos: %w", err)
		}
		if err := persistence.AtomicWrite(filepath.Join(vDir, "todolist.json"), todoData); err != nil {
			return 0, fmt.Errorf("write todolist.json: %w", err)
		}
	}

	// Update manifest.
	if err := s.writeManifest(manifest); err != nil {
		return 0, fmt.Errorf("write manifest: %w", err)
	}

	return newVersion, nil
}

// readManifest reads .slidecraft/manifest.json.
func (s *SnapshotStore) readManifest() (*VersionManifest, error) {
	manifestPath := filepath.Join(s.baseDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read manifest.json: %w", err)
	}
	var m VersionManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest.json: %w", err)
	}
	return &m, nil
}

// writeManifest writes .slidecraft/manifest.json.
func (s *SnapshotStore) writeManifest(m *VersionManifest) error {
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return fmt.Errorf("mkdir slidecraft dir: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return persistence.AtomicWrite(filepath.Join(s.baseDir, "manifest.json"), data)
}

// DiscoverVersion scans the versions directory for the latest version number.
// This is a fallback when manifest.json is missing or corrupt.
func (s *SnapshotStore) DiscoverVersion() int {
	versionsDir := filepath.Join(s.baseDir, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return 0
	}
	maxVersion := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "v") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(name, "v"))
		if err != nil {
			continue
		}
		if n > maxVersion {
			maxVersion = n
		}
	}
	return maxVersion
}

// MarkInvalid removes a corrupted version directory and updates the manifest.
func (s *SnapshotStore) MarkInvalid(version int) error {
	vDir := s.versionDir(version)
	if err := os.RemoveAll(vDir); err != nil {
		return fmt.Errorf("remove invalid version dir: %w", err)
	}

	manifest, err := s.readManifest()
	if err != nil || manifest == nil {
		return nil
	}

	filtered := make([]Version, 0, len(manifest.Versions))
	for _, v := range manifest.Versions {
		if v.Number != version {
			filtered = append(filtered, v)
		}
	}
	manifest.Versions = filtered
	if len(filtered) > 0 {
		manifest.CurrentVersion = filtered[len(filtered)-1].Number
	} else {
		manifest.CurrentVersion = 0
	}
	return s.writeManifest(manifest)
}

// VersionDir returns the path to a version's directory.
func (s *SnapshotStore) VersionDir(version int) string {
	return s.versionDir(version)
}

// sessionTodoPath returns the path to the session-level todolist file.
// Session-level storage lives at .agentgo/sessions/{sessionID}/todolist.json,
// decoupled from version directories so todos persist even before the first version.
func (s *SnapshotStore) sessionTodoPath(sessionID string) string {
	return filepath.Join(filepath.Dir(s.baseDir), ".agentgo", "sessions", sessionID, "todolist.json")
}

// PersistSessionTodo writes todo items to session-level storage.
// Always available regardless of version state.
func (s *SnapshotStore) PersistSessionTodo(sessionID string, items []TodoItemRecord) error {
	if len(items) == 0 {
		return nil
	}
	todoPath := s.sessionTodoPath(sessionID)
	if err := os.MkdirAll(filepath.Dir(todoPath), 0o755); err != nil {
		return fmt.Errorf("mkdir session dir: %w", err)
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session todos: %w", err)
	}
	return persistence.AtomicWrite(todoPath, data)
}

// LoadSessionTodo reads todo items from session-level storage.
// Returns nil, nil if the file does not exist.
func (s *SnapshotStore) LoadSessionTodo(sessionID string) ([]TodoItemRecord, error) {
	data, err := os.ReadFile(s.sessionTodoPath(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session todolist: %w", err)
	}
	var items []TodoItemRecord
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("unmarshal session todolist: %w", err)
	}
	return items, nil
}

// PersistTodo writes the todo items to todolist.json in the version directory.
func (s *SnapshotStore) PersistTodo(version int, items []TodoItemRecord) error {
	if len(items) == 0 {
		return nil
	}
	vDir := s.versionDir(version)
	if err := os.MkdirAll(vDir, 0o755); err != nil {
		return fmt.Errorf("mkdir version dir: %w", err)
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal version todos: %w", err)
	}
	return persistence.AtomicWrite(filepath.Join(vDir, "todolist.json"), data)
}

// ParseTodoArgs extracts TodoItemRecord from the JSON arguments of a todo_write tool call.
func ParseTodoArgs(argsJSON string) ([]TodoItemRecord, error) {
	var parsed struct {
		Todos []struct {
			Content    string `json:"content"`
			Status     string `json:"status"`
			ActiveForm string `json:"activeForm"`
		} `json:"todos"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &parsed); err != nil {
		return nil, fmt.Errorf("parse todo args: %w", err)
	}
	items := make([]TodoItemRecord, len(parsed.Todos))
	for i, t := range parsed.Todos {
		items[i] = TodoItemRecord{
			Content:    t.Content,
			Status:     t.Status,
			ActiveForm: t.ActiveForm,
		}
	}
	return items, nil
}

func copyFile(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer srcF.Close()

	dstF, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	defer dstF.Close()

	if _, err := io.Copy(dstF, srcF); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return dstF.Sync()
}
