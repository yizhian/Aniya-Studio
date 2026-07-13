package context

import "time"

// DesignSnapshot captures the structural essence of an HTML slide deck.
// This is what enters the LLM context instead of raw HTML.
type DesignSnapshot struct {
	Title         string            `json:"title,omitempty"`
	Theme         string            `json:"theme,omitempty"`
	SlideCount    int               `json:"slide_count"`
	SlideHeadings []string          `json:"slide_headings,omitempty"`
	CSSClasses    []string          `json:"css_classes_used,omitempty"`
	Fonts         []FontInfo        `json:"fonts,omitempty"`
	ColorPalette  map[string]string `json:"color_palette,omitempty"`
	Sections      []HTMLSection     `json:"html_sections,omitempty"`
	FileSizeBytes int64             `json:"total_size_bytes"`
	ActiveFile    string            `json:"active_file,omitempty"`
}

// FontInfo describes a font used in the HTML.
type FontInfo struct {
	Family string `json:"family"`
	Source string `json:"source"`
}

// HTMLSection is a structural element extracted from the HTML.
type HTMLSection struct {
	Tag     string `json:"tag"`
	Class   string `json:"class,omitempty"`
	ID      string `json:"id,omitempty"`
	Heading string `json:"heading,omitempty"`
}

// ContextJSON is the on-disk schema for .slidecraft/versions/vN/context.json
type ContextJSON struct {
	Version        int              `json:"version"`
	CreatedAt      time.Time        `json:"created_at"`
	SessionID      string           `json:"session_id"`
	HTMLPath       string           `json:"html_path"`
	Title          string           `json:"title,omitempty"`
	DesignSnapshot DesignSnapshot   `json:"design_snapshot"`
	Todos          []TodoItemRecord `json:"todos_at_version,omitempty"`
}

// TodoItemRecord mirrors the TodoWrite tool's input schema.
type TodoItemRecord struct {
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"activeForm"`
}

// VersionManifest tracks all versions in the project.
type VersionManifest struct {
	CurrentVersion int       `json:"current_version"`
	HTMLFile       string    `json:"html_file"`
	Versions       []Version `json:"versions"`
}

// Version records metadata for a single version.
type Version struct {
	Number    int       `json:"number"`
	CreatedAt time.Time `json:"created_at"`
	HTMLFile  string    `json:"html_file"`
}

// ToolExecSummary captures whether a tool call succeeded.
// ToolCallID matches model.ToolCall.ID.
type ToolExecSummary struct {
	ToolCallID string
	Success    bool
}
