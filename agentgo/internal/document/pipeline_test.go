package document

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// cleanText
// ---------------------------------------------------------------------------

func TestCleanText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no change", "hello world", "hello world"},
		{"CRLF to LF", "line1\r\nline2\r\nline3", "line1\nline2\nline3"},
		{"CR to LF", "line1\rline2", "line1\nline2"},
		{"collapse 3+ newlines", "a\n\n\n\nb", "a\n\nb"},
		{"trim space", "  hello  ", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanText(tt.in)
			if got != tt.want {
				t.Errorf("cleanText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// type detection
// ---------------------------------------------------------------------------

func TestTypeByExtension(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"doc.md", "markdown"},
		{"notes.txt", "text"},
		{"report.pdf", "pdf"},
		{"document.docx", "docx"},
		{"logo.png", "image"},
		{"photo.jpg", "image"},
		{"hero.webp", "image"},
		{"icon.svg", "image"},
		{"data.xlsx", ""},
		{"slides.pptx", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := TypeByExtension(tt.filename)
			if got != tt.want {
				t.Errorf("TypeByExtension(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsSupportedExtension(t *testing.T) {
	if !IsSupportedExtension("doc.md") {
		t.Error("expected .md to be supported")
	}
	if IsSupportedExtension("data.xlsx") {
		t.Error("expected .xlsx to be unsupported")
	}
}

func TestIsProbablyText(t *testing.T) {
	if !isProbablyText([]byte("hello world\nline2")) {
		t.Error("expected text to be detected as text")
	}
	if isProbablyText([]byte{0x00, 0x01, 0x02, 0xFF}) {
		t.Error("expected binary NOT to be detected as text")
	}
}

// ---------------------------------------------------------------------------
// TextParser
// ---------------------------------------------------------------------------

func TestTextParser_Markdown(t *testing.T) {
	dir := t.TempDir()
	p := &TextParser{}

	content := "# Hello\n\nThis is **markdown** content.\n\n- item 1\n- item 2"
	doc, err := p.Parse(File{Name: "test.md", Data: []byte(content)}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Type != "markdown" {
		t.Errorf("expected type=markdown, got %s", doc.Type)
	}
	if doc.CharCount == 0 {
		t.Error("expected non-zero char count")
	}

	saved, err := os.ReadFile(doc.SavedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != cleanText(content) {
		t.Errorf("saved content mismatch")
	}
}

func TestTextParser_Empty(t *testing.T) {
	p := &TextParser{}
	doc, err := p.Parse(File{Name: "empty.md", Data: []byte("")}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if doc.CharCount != 0 {
		t.Errorf("expected 0 chars for empty file, got %d", doc.CharCount)
	}
}

// ---------------------------------------------------------------------------
// ImageParser
// ---------------------------------------------------------------------------

func TestImageParser_PNG(t *testing.T) {
	p := &ImageParser{}
	pngData := minimalPNG()
	doc, err := p.Parse(File{Name: "test.png", Data: pngData, Size: int64(len(pngData))}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Type != "image" {
		t.Errorf("expected type=image, got %s", doc.Type)
	}
	if doc.Width != 1 || doc.Height != 1 {
		t.Errorf("expected 1x1, got %dx%d", doc.Width, doc.Height)
	}
	if !strings.Contains(doc.SavedPath, "assets") {
		t.Errorf("expected saved path in assets, got %s", doc.SavedPath)
	}
}

func TestImageParser_Invalid(t *testing.T) {
	p := &ImageParser{}
	_, err := p.Parse(File{Name: "fake.png", Data: []byte("not an image")}, t.TempDir())
	if err == nil {
		t.Error("expected error for invalid image")
	}
}

// ---------------------------------------------------------------------------
// DocxParser
// ---------------------------------------------------------------------------

func TestDocxParser_Valid(t *testing.T) {
	p := &DocxParser{}
	docxData := createMinimalDocx("Hello world")
	doc, err := p.Parse(File{Name: "test.docx", Data: docxData, Size: int64(len(docxData))}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Type != "docx" {
		t.Errorf("expected type=docx, got %s", doc.Type)
	}
	if doc.CharCount == 0 {
		t.Error("expected non-zero char count")
	}
	if !strings.HasSuffix(doc.SavedPath, ".docx.txt") {
		t.Errorf("expected saved path to end with .docx.txt, got %s", doc.SavedPath)
	}
}

func TestDocxParser_InvalidZIP(t *testing.T) {
	p := &DocxParser{}
	_, err := p.Parse(File{Name: "bad.docx", Data: []byte("not a zip")}, t.TempDir())
	if err == nil {
		t.Error("expected error for invalid ZIP")
	}
}

func TestDocxParser_NoDocumentXML(t *testing.T) {
	p := &DocxParser{}
	docxData := createZipWithoutDocXML()
	_, err := p.Parse(File{Name: "empty.docx", Data: docxData, Size: int64(len(docxData))}, t.TempDir())
	if err == nil {
		t.Error("expected error for missing word/document.xml")
	}
}

// ---------------------------------------------------------------------------
// PDFParser
// ---------------------------------------------------------------------------

func TestPDFParser_Valid(t *testing.T) {
	p := &PDFParser{}
	pdfData := minimalPDF()
	doc, err := p.Parse(File{Name: "report.pdf", Data: pdfData, Size: int64(len(pdfData))}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Type != "pdf" {
		t.Errorf("expected type=pdf, got %s", doc.Type)
	}
	if doc.Pages != 1 {
		t.Errorf("expected 1 page, got %d", doc.Pages)
	}
	if doc.CharCount == 0 {
		t.Error("expected non-zero char count")
	}
	if !strings.HasSuffix(doc.SavedPath, ".pdf.txt") {
		t.Errorf("expected saved path to end with .pdf.txt, got %s", doc.SavedPath)
	}
}

func TestPDFParser_Invalid(t *testing.T) {
	p := &PDFParser{}
	_, err := p.Parse(File{Name: "bad.pdf", Data: []byte("not a pdf")}, t.TempDir())
	if err == nil {
		t.Error("expected error for invalid PDF")
	}
}

// ---------------------------------------------------------------------------
// UnsupportedParser
// ---------------------------------------------------------------------------

func TestUnsupportedParser(t *testing.T) {
	p := &UnsupportedParser{}
	doc, err := p.Parse(File{Name: "data.xlsx", Data: nil}, "")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Type != "unsupported" {
		t.Errorf("expected type=unsupported, got %s", doc.Type)
	}
}

// ---------------------------------------------------------------------------
// FormatForUserMessage
// ---------------------------------------------------------------------------

func TestFormatForUserMessage_Empty(t *testing.T) {
	if s := FormatForUserMessage(nil); s != "" {
		t.Errorf("expected empty for nil, got %q", s)
	}
	if s := FormatForUserMessage([]ParsedDocument{}); s != "" {
		t.Errorf("expected empty for empty slice, got %q", s)
	}
}

func TestFormatForUserMessage_Success(t *testing.T) {
	docs := []ParsedDocument{
		{OriginalName: "report.md", Type: "markdown", SavedPath: "/tmp/docs/report.md", Summary: "文本文档，约 800 字符"},
		{OriginalName: "logo.png", Type: "image", SavedPath: "/tmp/assets/logo.png", Summary: "png（240×80），可在 HTML 中引用"},
	}
	result := FormatForUserMessage(docs)
	if !strings.Contains(result, "report.md") {
		t.Error("expected report.md in output")
	}
	if !strings.Contains(result, "read_file") {
		t.Error("expected read_file hint")
	}
}

func TestFormatForUserMessage_Mixed(t *testing.T) {
	docs := []ParsedDocument{
		{OriginalName: "ok.md", Type: "markdown", SavedPath: "/tmp/docs/ok.md", Summary: "文本文档"},
		{OriginalName: "bad.pdf", Type: "error", Error: "PDF parse failed"},
		{OriginalName: "ok.png", Type: "image", SavedPath: "/tmp/assets/ok.png", Summary: "png (1x1)"},
	}
	result := FormatForUserMessage(docs)
	if !strings.Contains(result, "ok.md") {
		t.Error("expected ok.md")
	}
	if !strings.Contains(result, "解析失败") {
		t.Error("expected error indication for bad.pdf")
	}
}

// ---------------------------------------------------------------------------
// Pipeline
// ---------------------------------------------------------------------------

func TestPipeline_EmptyFiles(t *testing.T) {
	p := NewPipeline()
	if results, _ := p.ParseAll(context.Background(), nil, t.TempDir(), "test-upload"); len(results) != 0 {
		t.Errorf("expected 0 results for nil input")
	}
	if results, _ := p.ParseAll(context.Background(), []File{}, t.TempDir(), "test-upload"); len(results) != 0 {
		t.Errorf("expected 0 results for empty input")
	}
}

func TestPipeline_AllSameType(t *testing.T) {
	p := NewPipeline()
	var files []File
	for i := 0; i < 10; i++ {
		files = append(files, File{
			Name: fmt.Sprintf("doc%d.md", i),
			Data: []byte(fmt.Sprintf("# Doc %d", i)),
		})
	}
	results, _ := p.ParseAll(context.Background(), files, t.TempDir(), "test-upload")
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Type != "markdown" {
			t.Errorf("expected markdown, got %s for %s", r.Type, r.OriginalName)
		}
		if r.Error != "" {
			t.Errorf("unexpected error for %s: %s", r.OriginalName, r.Error)
		}
	}
}

func TestPipeline_Heterogeneous(t *testing.T) {
	p := NewPipeline()
	pngData := minimalPNG()

	files := []File{
		{Name: "readme.md", Data: []byte("# Readme")},
		{Name: "notes.txt", Data: []byte("Plain text")},
		{Name: "logo.png", Data: pngData, Size: int64(len(pngData))},
		{Name: "guide.md", Data: []byte("# Guide")},
		{Name: "data.txt", Data: []byte("data")},
		{Name: "unknown.xyz", Data: []byte("some text content")},
	}

	results, _ := p.ParseAll(context.Background(), files, t.TempDir(), "test-upload")
	if len(results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(results))
	}

	typeCounts := map[string]int{}
	for _, r := range results {
		typeCounts[r.Type]++
	}
	if typeCounts["markdown"] != 2 {
		t.Errorf("expected 2 markdown, got %d", typeCounts["markdown"])
	}
	if typeCounts["text"] != 3 { // 2 .txt + 1 .xyz via content sniffing
		t.Errorf("expected 3 text, got %d", typeCounts["text"])
	}
	if typeCounts["image"] != 1 {
		t.Errorf("expected 1 image, got %d", typeCounts["image"])
	}
}

func TestPipeline_UnsupportedFallback(t *testing.T) {
	p := NewPipeline()
	files := []File{
		{Name: "ok.md", Data: []byte("# OK")},
		{Name: "data.xlsx", Data: []byte{0x50, 0x4B}},
		{Name: "slides.pptx", Data: []byte{0x50, 0x4B}},
	}
	results, _ := p.ParseAll(context.Background(), files, t.TempDir(), "test-upload")
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Type == "unsupported" && r.Error != "" {
			t.Errorf("unsupported should not have error: %s", r.Error)
		}
	}
}

func TestPipeline_PartialFailure(t *testing.T) {
	p := NewPipeline()
	pngData := minimalPNG()

	files := []File{
		{Name: "ok1.md", Data: []byte("# OK")},
		{Name: "bad.png", Data: []byte("not an image")},
		{Name: "ok2.md", Data: []byte("# Also OK")},
		{Name: "ok3.png", Data: pngData, Size: int64(len(pngData))},
	}

	results, _ := p.ParseAll(context.Background(), files, t.TempDir(), "test-upload")
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Error != "" {
			errorCount++
		} else if r.Type != "unsupported" {
			successCount++
		}
	}
	if successCount != 3 {
		t.Errorf("expected 3 successes, got %d", successCount)
	}
	if errorCount != 1 {
		t.Errorf("expected 1 error, got %d", errorCount)
	}
}

func TestPipeline_ConcurrentWrites(t *testing.T) {
	p := NewPipeline()
	pngData := minimalPNG()

	files := []File{
		{Name: "a.md", Data: []byte("# A")},
		{Name: "b.md", Data: []byte("# B")},
		{Name: "c.txt", Data: []byte("C")},
		{Name: "img1.png", Data: pngData, Size: int64(len(pngData))},
		{Name: "img2.png", Data: pngData, Size: int64(len(pngData))},
		{Name: "d.md", Data: []byte("# D")},
		{Name: "e.txt", Data: []byte("E")},
		{Name: "img3.png", Data: pngData, Size: int64(len(pngData))},
	}

	dir := t.TempDir()
	results, _ := p.ParseAll(context.Background(), files, dir, "test-upload")
	if len(results) != 8 {
		t.Fatalf("expected 8 results, got %d", len(results))
	}

	docs, _ := os.ReadDir(filepath.Join(dir, "docs"))
	assets, _ := os.ReadDir(filepath.Join(dir, "assets"))

	if len(docs) != 5 {
		t.Errorf("expected 5 docs, got %d", len(docs))
	}
	if len(assets) != 3 {
		t.Errorf("expected 3 assets, got %d", len(assets))
	}
}

func TestPipeline_DocxTextNoCollision(t *testing.T) {
	// Verify a.docx → docs/a.docx.txt and a.txt → docs/a.txt don't collide.
	dir := t.TempDir()
	p := NewPipeline()

	docxData := createMinimalDocx("Word content")
	files := []File{
		{Name: "a.docx", Data: docxData, Size: int64(len(docxData))},
		{Name: "a.txt", Data: []byte("Text content")},
	}

	results, _ := p.ParseAll(context.Background(), files, dir, "test-upload")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Both should succeed.
	for _, r := range results {
		if r.Error != "" {
			t.Errorf("unexpected error for %s: %s", r.OriginalName, r.Error)
		}
	}

	// Verify both files exist and have different paths.
	if results[0].SavedPath == results[1].SavedPath {
		t.Error("docx and txt should have different output paths")
	}
}

func TestPipeline_ResolveParser(t *testing.T) {
	p := NewPipeline()

	if parser := p.resolveParser("test.md", []byte("# md")); parser == nil {
		t.Error("expected non-nil parser for .md")
	}
	if parser := p.resolveParser("test.png", minimalPNG()); parser == nil {
		t.Error("expected non-nil parser for .png")
	}
	if parser := p.resolveParser("test.pdf", []byte("fake")); parser == nil {
		t.Error("expected non-nil parser for .pdf")
	}
	if parser := p.resolveParser("test.docx", []byte("fake")); parser == nil {
		t.Error("expected non-nil parser for .docx")
	}
	// Content sniffing: no extension, text content → TextParser
	if parser := p.resolveParser("noext", []byte("hello world")); parser == nil {
		t.Error("expected non-nil parser for content-sniffed text")
	}
}

func TestSummarizeText(t *testing.T) {
	s := summarizeText("test.md", 1500)
	if !strings.Contains(s, "1500") {
		t.Errorf("expected char count in summary, got %q", s)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func minimalPNG() []byte {
	// Valid 1x1 red PNG.
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xD7, 0x63, 0x60, 0x60, 0x60, 0x00,
		0x00, 0x00, 0x04, 0x00, 0x01, 0x27, 0x34, 0x27,
		0x0A, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E,
		0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}

func createMinimalDocx(text string) []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	ct, _ := w.Create("[Content_Types].xml")
	ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`))

	rels, _ := w.Create("_rels/.rels")
	rels.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`))

	doc, _ := w.Create("word/document.xml")
	doc.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:body>
</w:document>`))

	w.Close()
	return buf.Bytes()
}

func createZipWithoutDocXML() []byte {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("other.txt")
	f.Write([]byte("not a docx"))
	w.Close()
	return buf.Bytes()
}

func TestWriteUniqueFile_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	path, err := writeUniqueFile(dir, "doc.txt", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "doc.txt" {
		t.Errorf("expected doc.txt, got %q", filepath.Base(path))
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestWriteUniqueFile_CreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new-subdir")
	path, err := writeUniqueFile(dir, "file.txt", []byte("content"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should exist: %v", err)
	}
}

func TestWriteUniqueFile_Collision(t *testing.T) {
	dir := t.TempDir()
	// Write first file.
	os.WriteFile(filepath.Join(dir, "doc.txt"), []byte("first"), 0644)
	// Second write should get _2 suffix.
	path, err := writeUniqueFile(dir, "doc.txt", []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "doc_2.txt" {
		t.Errorf("expected doc_2.txt, got %q", filepath.Base(path))
	}
	data, _ := os.ReadFile(path)
	if string(data) != "second" {
		t.Errorf("expected 'second', got %q", string(data))
	}
}

func TestWriteUniqueFile_MultipleCollisions(t *testing.T) {
	dir := t.TempDir()
	// Create doc.txt, doc_2.txt, doc_3.txt.
	for _, name := range []string{"doc.txt", "doc_2.txt", "doc_3.txt"} {
		os.WriteFile(filepath.Join(dir, name), []byte(name), 0644)
	}
	// Should get doc_4.txt.
	path, err := writeUniqueFile(dir, "doc.txt", []byte("new"))
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(path)
	if base != "doc_4.txt" {
		t.Errorf("expected doc_4.txt, got %q", base)
	}
}


func TestFormatForUserMessage_SuccessAndError(t *testing.T) {
	docs := []ParsedDocument{
		{OriginalName: "deck.pptx", SavedPath: "deck/converted.md", Summary: "Converted presentation, 200 chars"},
		{OriginalName: "broken.bin", Error: "unsupported format"},
	}
	result := FormatForUserMessage(docs)
	if !strings.Contains(result, "deck/converted.md") {
		t.Error("missing deck/converted.md")
	}
	if !strings.Contains(result, "broken.bin") {
		t.Error("missing broken.bin")
	}
	if !strings.Contains(result, "解析失败") {
		t.Error("missing error indicator for broken file")
	}
	if !strings.Contains(result, "read_file") {
		t.Error("missing read_file instruction")
	}
}

func TestFormatForUserMessage_NoSavedPath(t *testing.T) {
	docs := []ParsedDocument{
		{OriginalName: "upload.pdf", Summary: "PDF document, 500 chars"},
	}
	result := FormatForUserMessage(docs)
	if !strings.Contains(result, "upload.pdf") {
		t.Error("should use OriginalName when SavedPath is empty")
	}
}

func TestWriteUniqueFile_ReadOnlyDir(t *testing.T) {
	dir := t.TempDir()
	// Make the dir read-only.
	os.Chmod(dir, 0555)
	defer os.Chmod(dir, 0755)
	_, err := writeUniqueFile(dir, "file.txt", []byte("x"))
	if err == nil {
		t.Error("expected error writing to read-only directory")
	}
}

func TestTryWriteExclusive_Success(t *testing.T) {
	dir := t.TempDir()
	path, err := tryWriteExclusive(dir, "new.txt", []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "new.txt" {
		t.Errorf("expected new.txt, got %q", filepath.Base(path))
	}
}

func TestPDFParser_ValidPipeline(t *testing.T) {
	p := NewPipeline()
	pdfData := minimalPDF()
	results, _ := p.ParseAll(context.Background(), []File{
		{Name: "doc.pdf", Data: pdfData, Size: int64(len(pdfData))},
	}, t.TempDir(), "test-upload")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Type != "pdf" {
		t.Errorf("expected type=pdf, got %s", results[0].Type)
	}
	if results[0].Pages != 1 {
		t.Errorf("expected 1 page, got %d", results[0].Pages)
	}
}

func TestDocxParser_DeepNesting(t *testing.T) {
	// Construct deeply nested XML to verify depth protection.
	openTags := strings.Repeat("<w:p><w:r><w:t>", maxXMLNestDepth+10)
	closeTags := strings.Repeat("</w:t></w:r></w:p>", maxXMLNestDepth+10)
	deep := openTags + "x" + closeTags
	_, err := extractTextFromDocxXML([]byte(deep))
	if err == nil {
		t.Error("expected depth limit error for deeply nested XML")
	}
}

func TestTextParser_GBKEncoding(t *testing.T) {
	// "你好世界" in GBK encoding.
	gbk := []byte{0xC4, 0xE3, 0xBA, 0xC3, 0xCA, 0xC0, 0xBD, 0xE7}
	result, err := decodeToUTF8(gbk)
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if !strings.Contains(result, "你好") {
		t.Errorf("expected decoded Chinese text, got %q", result)
	}
}

func TestImageParser_SVGInvalid(t *testing.T) {
	p := &ImageParser{}
	_, err := p.Parse(File{Name: "bad.svg", Data: []byte("<unclosed>")}, t.TempDir())
	if err == nil {
		t.Error("expected error for invalid SVG")
	}
}

func TestImageParser_SVGValid(t *testing.T) {
	p := &ImageParser{}
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>`)
	doc, err := p.Parse(File{Name: "icon.svg", Data: svg, Size: int64(len(svg))}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Type != "image" {
		t.Errorf("expected type=image, got %s", doc.Type)
	}
}

func TestWriteUniqueFile_MaxRetries(t *testing.T) {
	dir := t.TempDir()
	// Fill all slots from doc.txt through doc_100.txt.
	for _, name := range []string{"doc.txt"} {
		os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644)
	}
	for i := 2; i <= maxWriteRetries; i++ {
		name := fmt.Sprintf("doc_%d.txt", i)
		os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644)
	}
	_, err := writeUniqueFile(dir, "doc.txt", []byte("overflow"))
	if err == nil {
		t.Error("expected error after exceeding max retries")
	}
}

// minimalPDF returns a valid single-page PDF with "Hello World" text.
func minimalPDF() []byte {
	// Build the body objects, then insert accurate xref offsets.
	header := "%PDF-1.4\n"
	objs := []string{
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n",
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n",
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n",
		"4 0 obj\n<< /Length 44 >>\nstream\nBT /F1 12 Tf 100 700 Td (Hello World) Tj ET\nendstream\nendobj\n",
		"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n",
	}

	var body strings.Builder
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = body.Len() + len(header)
		body.WriteString(o)
	}
	bodyStr := body.String()
	bodyEnd := len(header) + len(bodyStr)

	// Build xref section.
	var xref strings.Builder
	xref.WriteString("xref\n")
	fmt.Fprintf(&xref, "0 %d\n", len(objs)+1)
	fmt.Fprintf(&xref, "0000000000 65535 f \n")
	for _, off := range offsets {
		xref.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}

	// Build trailer.
	trailer := fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\n", len(objs)+1)
	startxref := bodyEnd

	footer := fmt.Sprintf("%sstartxref\n%d\n%%%%EOF", trailer, startxref)

	return []byte(header + bodyStr + xref.String() + footer)
}
