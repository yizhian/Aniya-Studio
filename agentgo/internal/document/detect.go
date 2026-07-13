package document

import (
	"path/filepath"
	"strings"
)

// supportedExtensions maps lowercase extensions (with leading dot) to type identifiers.
var supportedExtensions = map[string]string{
	".md":   "markdown",
	".txt":  "text",
	".pdf":  "pdf",
	".docx": "docx",
	".png":  "image",
	".jpg":  "image",
	".jpeg": "image",
	".gif":  "image",
	".webp": "image",
	".svg":  "image",
}

// TypeByExtension returns the document type for a given extension, or empty string if unsupported.
func TypeByExtension(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	return supportedExtensions[ext]
}

// IsSupportedExtension returns true if the file extension is in the whitelist.
func IsSupportedExtension(name string) bool {
	return TypeByExtension(name) != ""
}

// isProbablyText checks whether the file header looks like text (no null bytes).
func isProbablyText(data []byte) bool {
	limit := len(data)
	if limit > 512 {
		limit = 512
	}
	for _, b := range data[:limit] {
		if b == 0 {
			return false
		}
	}
	return true
}
