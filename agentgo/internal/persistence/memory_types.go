package persistence

import "time"

// MemoryType enumerates the five supported memory categories.
// The "user" type is system-managed; models must not create user memories.
type MemoryType string

const (
	MemoryTypeUser      MemoryType = "user"
	MemoryTypeDesign    MemoryType = "design"
	MemoryTypeComponent MemoryType = "component"
	MemoryTypeFeedback  MemoryType = "feedback"
	MemoryTypeTask      MemoryType = "task"
)

// MemoryFrontmatter is the YAML header of every memory file.
type MemoryFrontmatter struct {
	Type      MemoryType `yaml:"type"`
	Name      string     `yaml:"name"`
	UpdatedAt time.Time  `yaml:"updated_at"`
	Summary   string     `yaml:"summary"`
	ExpiresAt *time.Time `yaml:"expires_at,omitempty"`
}

// MemoryIndexEntry is a single line parsed from MEMORY.md.
type MemoryIndexEntry struct {
	Path    string
	Type    MemoryType
	Summary string
}

// RecalledMemory is a memory file loaded for context injection.
type RecalledMemory struct {
	Path      string
	Type      MemoryType
	Summary   string
	Content   string
	UpdatedAt time.Time
	ExpiresAt *time.Time
	DaysAgo   int
}

// MemoryFile is a lightweight file reference used during index rebuild.
type MemoryFile struct {
	Type    MemoryType
	Name    string
	Summary string
}

// Per-file and index size limits.
const (
	MaxMemoryBytesPerFile = 4 * 1024
	MaxIndexLines         = 200
	MaxIndexBytes         = 25000
)
