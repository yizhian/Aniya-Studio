package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FileMemoryStore implements MemoryStore using a directory tree under .agentgo/memory/.
type FileMemoryStore struct {
	debugf func(format string, args ...interface{})
}

// FileMemoryStoreOption configures a FileMemoryStore.
type FileMemoryStoreOption func(*FileMemoryStore)

// WithDebugf sets a debug logger for non-critical diagnostics.
func WithDebugf(fn func(format string, args ...interface{})) FileMemoryStoreOption {
	return func(s *FileMemoryStore) {
		s.debugf = fn
	}
}

// NewFileMemoryStore creates a new FileMemoryStore.
func NewFileMemoryStore(opts ...FileMemoryStoreOption) *FileMemoryStore {
	s := &FileMemoryStore{}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *FileMemoryStore) debug(format string, args ...interface{}) {
	if s.debugf != nil {
		s.debugf(format, args...)
	}
}

// GetBasePath returns the absolute path to the memory directory.
func (s *FileMemoryStore) GetBasePath(workspacePath string) string {
	return filepath.Join(workspacePath, ".agentgo", "memory")
}

// LoadUserMemory reads all .md files under user/ and returns their concatenated bodies
// (frontmatter stripped). Returns empty string if the directory doesn't exist.
func (s *FileMemoryStore) LoadUserMemory(workspacePath string) (string, error) {
	userDir := filepath.Join(s.GetBasePath(workspacePath), "user")
	entries, err := os.ReadDir(userDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read user memory dir: %w", err)
	}

	var parts []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		content, err := s.readBody(filepath.Join(userDir, e.Name()))
		if err != nil {
			s.debug("memory: skip user file %s: %v", e.Name(), err)
			continue
		}
		if content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

// WriteMemory atomically writes a memory file at basePath/relPath.
// relPath is validated via safeJoin to prevent traversal outside the memory directory.
func (s *FileMemoryStore) WriteMemory(workspacePath, relPath, content string) error {
	fullPath, err := safeJoin(s.GetBasePath(workspacePath), relPath)
	if err != nil {
		return fmt.Errorf("write memory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("create memory dir: %w", err)
	}
	return AtomicWrite(fullPath, []byte(content))
}

// LoadRecalled reads and parses the memory files at the given relative paths.
// Paths are validated to prevent traversal outside the memory base directory.
func (s *FileMemoryStore) LoadRecalled(workspacePath string, paths []string) ([]RecalledMemory, error) {
	base := s.GetBasePath(workspacePath)
	var result []RecalledMemory
	for _, p := range paths {
		fullPath, err := safeJoin(base, p)
		if err != nil {
			s.debug("memory: skip recalled path %s: %v", p, err)
			continue
		}
		fm, body, err := parseFrontmatter(fullPath)
		if err != nil {
			s.debug("memory: skip recalled file %s: %v", p, err)
			continue
		}
		if len(body) > MaxMemoryBytesPerFile {
			body = body[:MaxMemoryBytesPerFile] + "\n... [truncated]"
		}
		result = append(result, RecalledMemory{
			Path:      p,
			Type:      fm.Type,
			Summary:   fm.Summary,
			Content:   body,
			UpdatedAt: fm.UpdatedAt,
			ExpiresAt: fm.ExpiresAt,
			DaysAgo:   int(time.Since(fm.UpdatedAt).Hours() / 24),
		})
	}
	return result, nil
}

// readBody reads a file and returns its content after the YAML frontmatter.
func (s *FileMemoryStore) readBody(path string) (string, error) {
	_, body, err := parseFrontmatter(path)
	return body, err
}

// parseFrontmatter reads a markdown file with YAML frontmatter (--- ... ---).
// Returns the parsed frontmatter and the body text.
func parseFrontmatter(path string) (MemoryFrontmatter, string, error) {
	var fm MemoryFrontmatter
	data, err := os.ReadFile(path)
	if err != nil {
		return fm, "", err
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return fm, text, nil
	}
	endIdx := strings.Index(text[4:], "\n---\n")
	if endIdx == -1 {
		return fm, text, nil
	}
	yamlBlock := text[4 : 4+endIdx]
	body := text[4+endIdx+5:]
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return fm, "", fmt.Errorf("parse frontmatter YAML in %s: %w", path, err)
	}
	return fm, body, nil
}

// safeJoin joins base with a relative path and verifies the result stays under base.
// It resolves symlinks on both base and any existing path segments to prevent
// escape via symlink indirection (e.g. memory/design/link → /etc).
func safeJoin(base, rel string) (string, error) {
	cleanRel := filepath.Clean(rel)
	if filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("memory path must be relative: %s", rel)
	}

	// Resolve base to canonical form. If base doesn't exist yet (fresh workspace),
	// use Abs — symlink checks are deferred until the tree actually exists.
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		realBase = base
		if realBase, err = filepath.Abs(realBase); err != nil {
			return "", err
		}
	}
	realBaseWithSep := realBase + string(filepath.Separator)

	fullPath := filepath.Join(realBase, cleanRel)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	if absPath != realBase && !strings.HasPrefix(absPath, realBaseWithSep) {
		return "", fmt.Errorf("memory path escapes base directory: %s", rel)
	}

	// Walk every segment from absPath up to realBase, checking for symlinks.
	// We must inspect ALL existing segments — not just the deepest one — because
	// a parent directory could be a symlink even if the target itself is not.
	existing := absPath
	for {
		if existing == realBase {
			break
		}
		info, err := os.Lstat(existing)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			// Resolve this symlink and rebuild the full path.
			realExisting, err := filepath.EvalSymlinks(existing)
			if err != nil {
				return "", fmt.Errorf("resolve symlinks at %s: %w", existing, err)
			}
			relSuffix, err := filepath.Rel(existing, absPath)
			if err != nil {
				return "", fmt.Errorf("compute relative suffix: %w", err)
			}
			rebuilt := filepath.Join(realExisting, relSuffix)
			rebuilt, err = filepath.Abs(rebuilt)
			if err != nil {
				return "", err
			}
			if rebuilt != realBase && !strings.HasPrefix(rebuilt, realBaseWithSep) {
				return "", fmt.Errorf("memory path escapes base via symlink: %s", rel)
			}
			return rebuilt, nil
		}
		// Walk up one level (segment either doesn't exist yet, or exists and is safe).
		parent := filepath.Dir(existing)
		if parent == existing || (parent != realBase && !strings.HasPrefix(parent, realBaseWithSep)) {
			break
		}
		existing = parent
	}

	return absPath, nil
}

// AtomicWrite writes data to a unique temp file and renames it over the target.
func AtomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, base+".*.tmp")
	if err != nil {
		logAtomicWriteError(path, err)
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		logAtomicWriteError(path, err)
		return err
	}
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	// Defend against TOCTOU symlink swap between safeJoin check and write.
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		os.Remove(tmpPath)
		logMemorySecurity(path, "symlink_blocked")
		return fmt.Errorf("refusing to overwrite symlink at %s", path)
	}
	return os.Rename(tmpPath, path)
}


func logMemorySecurity(path, event string) {
	fmt.Fprintf(os.Stderr, `{"ts":"%s","type":"memory:security","path":"%s","event":"%s"}`+"\n",
		time.Now().Format(time.RFC3339), path, event)
}

func logAtomicWriteError(path string, err error) {
	fmt.Fprintf(os.Stderr, `{"ts":"%s","type":"atomic_write:error","path":"%s","error":"%s"}`+"\n",
		time.Now().Format(time.RFC3339), path, err.Error())
}
