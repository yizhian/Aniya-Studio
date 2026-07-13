package document

// Parser is the interface that all document parsers implement.
// Each parser is stateless and safe for concurrent use.
type Parser interface {
	// Parse processes a single file and returns a parsed document.
	// sessionDir is the session-specific directory where output files are written
	// (e.g., .agentgo/sessions/{id}/).
	Parse(file File, sessionDir string) (*ParsedDocument, error)

	// SupportedExtensions returns the file extensions this parser handles,
	// including the leading dot (e.g., ".md", ".txt").
	SupportedExtensions() []string
}
