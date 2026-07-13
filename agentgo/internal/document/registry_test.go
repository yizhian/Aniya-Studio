package document

import (
	"testing"
)

func TestNewParserRegistry_NotEmpty(t *testing.T) {
	r := NewParserRegistry()
	if r == nil {
		t.Fatal("NewParserRegistry returned nil")
	}
	if len(r.parsers) == 0 {
		t.Error("expected non-empty registry")
	}
}

func TestParserRegistry_Get_KnownExtension(t *testing.T) {
	r := NewParserRegistry()
	p, ok := r.Get(".txt")
	if !ok {
		t.Error("expected parser for .txt")
	}
	if p == nil {
		t.Error("expected non-nil parser for .txt")
	}
	_, isText := p.(*TextParser)
	if !isText {
		t.Errorf("expected TextParser for .txt, got %T", p)
	}
}

func TestParserRegistry_Get_KnownExtension_Markdown(t *testing.T) {
	r := NewParserRegistry()
	p, ok := r.Get(".md")
	if !ok {
		t.Error("expected parser for .md")
	}
	if p == nil {
		t.Error("expected non-nil parser for .md")
	}
}

func TestParserRegistry_Get_UnknownExtension(t *testing.T) {
	r := NewParserRegistry()
	p, ok := r.Get(".xyz")
	if ok {
		t.Error("expected no parser for .xyz")
	}
	if p != nil {
		t.Errorf("expected nil parser for .xyz, got %T", p)
	}
}

func TestParserRegistry_Get_EmptyExtension(t *testing.T) {
	r := NewParserRegistry()
	p, ok := r.Get("")
	if ok {
		t.Error("expected no parser for empty extension")
	}
	if p != nil {
		t.Error("expected nil parser for empty extension")
	}
}

func TestParserRegistry_Get_AllBuiltins(t *testing.T) {
	r := NewParserRegistry()
	tests := []struct {
		ext      string
		wantType string
	}{
		{".txt", "*document.TextParser"},
		{".md", "*document.TextParser"},
		{".png", "*document.ImageParser"},
		{".jpg", "*document.ImageParser"},
		{".jpeg", "*document.ImageParser"},
		{".webp", "*document.ImageParser"},
		{".svg", "*document.ImageParser"},
		{".pdf", "*document.PDFParser"},
		{".docx", "*document.DocxParser"},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			p, ok := r.Get(tt.ext)
			if !ok {
				t.Errorf("expected parser for %s", tt.ext)
				return
			}
			if p == nil {
				t.Errorf("expected non-nil parser for %s", tt.ext)
			}
		})
	}
}

func TestParserRegistry_Resolve_ExtensionMatch(t *testing.T) {
	r := NewParserRegistry()
	p := r.Resolve("notes.txt", []byte("hello world"))
	if p == nil {
		t.Fatal("expected parser for .txt")
	}
	_, isText := p.(*TextParser)
	if !isText {
		t.Errorf("expected TextParser, got %T", p)
	}
}

func TestParserRegistry_Resolve_UnknownExtTextContent(t *testing.T) {
	r := NewParserRegistry()
	p := r.Resolve("data.xyz", []byte("hello world"))
	if p == nil {
		t.Fatal("expected fallback parser for text content")
	}
	_, isText := p.(*TextParser)
	if !isText {
		t.Errorf("expected TextParser fallback for text content, got %T", p)
	}
}

func TestParserRegistry_Resolve_UnknownExtBinaryContent(t *testing.T) {
	r := NewParserRegistry()
	p := r.Resolve("data.xyz", []byte{0x00, 0x01, 0x02})
	if p == nil {
		t.Fatal("expected unsupported parser for binary content")
	}
	_, isUnsupported := p.(*UnsupportedParser)
	if !isUnsupported {
		t.Errorf("expected UnsupportedParser for binary content, got %T", p)
	}
}

func TestParserRegistry_Register_Custom(t *testing.T) {
	r := NewParserRegistry()
	custom := &TextParser{}
	r.Register(custom)
	// TextParser re-registers .txt and .md — verify they still work.
	p, ok := r.Get(".txt")
	if !ok {
		t.Error("expected .txt parser after re-register")
	}
	if p == nil {
		t.Error("expected non-nil parser")
	}
}

func TestParserRegistry_Register_Overwrites(t *testing.T) {
	r := &ParserRegistry{parsers: make(map[string]Parser)}
	r.Register(&TextParser{}) // registers .txt, .md
	p1, _ := r.Get(".txt")

	// Register a different parser for .txt
	r.Register(&PDFParser{}) // PDFParser doesn't register .txt, so no overwrite
	p2, _ := r.Get(".txt")

	if p1 != p2 {
		t.Error("expected same parser after non-overlapping register")
	}
}

func TestParserRegistry_Resolve_CaseInsensitiveExt(t *testing.T) {
	r := NewParserRegistry()
	p := r.Resolve("IMAGE.PNG", []byte{0x89, 0x50, 0x4E, 0x47})
	if p == nil {
		t.Fatal("expected parser for .PNG")
	}
	_, isImage := p.(*ImageParser)
	if !isImage {
		t.Errorf("expected ImageParser for .PNG, got %T", p)
	}
}
