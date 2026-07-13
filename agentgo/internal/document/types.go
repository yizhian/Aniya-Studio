package document

import "path/filepath"

// ParsedDocument is the unified output structure for all parsers.
type ParsedDocument struct {
	OriginalName string `json:"original_name"`
	Type         string `json:"type"` // markdown | text | pdf | docx | image | unsupported | error
	SavedPath    string `json:"saved_path,omitempty"`
	OriginalPath string `json:"original_path,omitempty"` // path to the saved original binary file
	Summary      string `json:"summary,omitempty"`
	CharCount    int    `json:"char_count,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	Format       string `json:"format,omitempty"` // image format: png, jpeg, etc.
	Pages        int    `json:"pages,omitempty"`
	Error        string `json:"error,omitempty"`
}

// File represents an uploaded file to be parsed.
type File struct {
	Name string // original filename including extension
	Data []byte // raw file content
	Size int64  // file size in bytes
}

// ParseStats summarises the outcome of a ParseAll run.
type ParseStats struct {
	TotalFiles      int   `json:"total_files"`
	Succeeded       int   `json:"succeeded"`
	Unsupported     int   `json:"unsupported"`
	Errors          int   `json:"errors"`
	TotalDurationMs int64 `json:"total_duration_ms"`
}

// UploadMeta is written to upload_meta.json inside an upload target directory.
type UploadMeta struct {
	UploadID  string           `json:"upload_id"`
	CreatedAt string           `json:"created_at"`
	UpdatedAt string           `json:"updated_at,omitempty"`
	Files     []UploadMetaFile `json:"files"`
}

// UploadMetaFile describes one file in the upload manifest.
type UploadMetaFile struct {
	OriginalName    string `json:"original_name"`
	SavedName       string `json:"saved_name"`
	SavedPathRel    string `json:"saved_path_rel"`
	OriginalPathRel string `json:"original_path_rel,omitempty"` // path to original binary (relative to project dir)
	Type            string `json:"type"`
	CharCount       int    `json:"char_count,omitempty"`
	Pages           int    `json:"pages,omitempty"`
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
	Format          string `json:"format,omitempty"`
	Error           string `json:"error,omitempty"`
}

// BuildSavedPathRel returns the Agent-visible relative path for a parsed file.
// Text-like files go under uploads/docs/; images go under uploads/assets/.
func BuildSavedPathRel(doc ParsedDocument) string {
	savedName := filepath.Base(doc.SavedPath)
	switch doc.Type {
	case "pdf", "docx", "markdown", "text":
		return "uploads/docs/" + savedName
	case "image":
		return "uploads/assets/" + savedName
	default:
		return ""
	}
}

// BuildOriginalPathRel returns the relative path to the original file stored in uploads/originals/.
func BuildOriginalPathRel(originalName string) string {
	return "uploads/originals/" + originalName
}
