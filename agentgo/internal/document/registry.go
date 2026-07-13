package document

import (
	"log"
	"path/filepath"
	"strings"
	"time"
)

// ParserRegistry holds a set of document parsers keyed by extension (including dot).
// It is safe for concurrent use after initial registration.
type ParserRegistry struct {
	parsers map[string]Parser
}

// NewParserRegistry creates a registry with all built-in parsers pre-registered.
func NewParserRegistry() *ParserRegistry {
	r := &ParserRegistry{parsers: make(map[string]Parser)}
	r.Register(&TextParser{})
	r.Register(&ImageParser{})
	r.Register(&PDFParser{})
	r.Register(&DocxParser{})
	return r
}

// Register adds a parser to the registry. Each extension returned by
// SupportedExtensions is mapped to this parser. The last parser registered
// for an extension wins.
func (r *ParserRegistry) Register(parser Parser) {
	for _, ext := range parser.SupportedExtensions() {
		r.parsers[ext] = parser
	}
}

// Get returns the parser registered for the given extension (with leading dot).
// Returns nil, false if no parser is registered.
func (r *ParserRegistry) Get(ext string) (Parser, bool) {
	p, ok := r.parsers[ext]
	return p, ok
}

// Resolve finds the appropriate parser for a file: extension match first,
// then content sniffing fallback (text if no null bytes).
func (r *ParserRegistry) Resolve(name string, data []byte) Parser {
	ext := strings.ToLower(filepath.Ext(name))
	if parser, ok := r.parsers[ext]; ok {
		return parser
	}
	if isProbablyText(data) {
		if parser, ok := r.parsers[".txt"]; ok {
			return parser
		}
	}
	log.Printf(`{"ts":"%s","type":"document:unsupported","file":"%s","ext":"%s"}`,
		time.Now().Format(time.RFC3339), name, ext)
	return &UnsupportedParser{}
}
