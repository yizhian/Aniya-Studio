package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileMemoryIndex manages the MEMORY.md index file.
type FileMemoryIndex struct {
	basePath string
}

// NewFileMemoryIndex creates a new FileMemoryIndex for the given memory base path.
func NewFileMemoryIndex(basePath string) *FileMemoryIndex {
	return &FileMemoryIndex{basePath: basePath}
}

// LoadIndex reads MEMORY.md, parses entries, and applies truncation.
func (idx *FileMemoryIndex) LoadIndex() ([]MemoryIndexEntry, error) {
	indexPath := filepath.Join(idx.basePath, "MEMORY.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseIndex(string(data)), nil
}

// RebuildIndex scans all memory directories and rebuilds MEMORY.md from file frontmatters.
func (idx *FileMemoryIndex) RebuildIndex() error {
	if err := os.MkdirAll(idx.basePath, 0755); err != nil {
		return err
	}
	files, err := idx.scanMemoryFiles()
	if err != nil {
		return err
	}

	var lines []string
	lines = append(lines, "# Memory Index")
	for _, f := range files {
		lines = append(lines, fmt.Sprintf("- %s/%s.md: %s", f.Type, f.Name, f.Summary))
	}

	content := strings.Join(lines, "\n")
	if len(content) > MaxIndexBytes {
		content = content[:MaxIndexBytes] + "\n[... truncated ...]"
	}

	indexPath := filepath.Join(idx.basePath, "MEMORY.md")
	return AtomicWrite(indexPath, []byte(content))
}

// scanMemoryFiles walks the memory directories and collects MemoryFile entries.
func (idx *FileMemoryIndex) scanMemoryFiles() ([]MemoryFile, error) {
	var files []MemoryFile

	types := []MemoryType{MemoryTypeUser, MemoryTypeDesign, MemoryTypeComponent, MemoryTypeFeedback, MemoryTypeTask}
	for _, t := range types {
		dir := filepath.Join(idx.basePath, string(t))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			fullPath := filepath.Join(dir, e.Name())
			fm, _, err := parseFrontmatter(fullPath)
			if err != nil {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			files = append(files, MemoryFile{
				Type:    t,
				Name:    name,
				Summary: fm.Summary,
			})
		}
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].Type != files[j].Type {
			return files[i].Type < files[j].Type
		}
		return files[i].Name < files[j].Name
	})

	return files, nil
}

// parseIndex parses MEMORY.md content into MemoryIndexEntry values.
func parseIndex(content string) []MemoryIndexEntry {
	var entries []MemoryIndexEntry
	lines := strings.Split(content, "\n")

	// Apply line truncation.
	if len(lines) > MaxIndexLines {
		lines = lines[:MaxIndexLines]
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		// Format: "- type/name.md: summary"
		entry := line[2:]
		colonIdx := strings.Index(entry, ": ")
		if colonIdx == -1 {
			continue
		}
		relPath := entry[:colonIdx]
		summary := entry[colonIdx+2:]

		slashIdx := strings.Index(relPath, "/")
		if slashIdx <= 0 {
			continue
		}
		memType := MemoryType(relPath[:slashIdx])

		entries = append(entries, MemoryIndexEntry{
			Path:    relPath,
			Type:    memType,
			Summary: summary,
		})
	}
	return entries
}
