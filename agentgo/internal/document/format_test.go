package document

import (
	"strings"
	"testing"
)

func TestFormatUploadedFilesForSystemPrompt_NilMeta(t *testing.T) {
	if got := FormatUploadedFilesForSystemPrompt(nil); got != "" {
		t.Errorf("expected empty string for nil meta, got %q", got)
	}
}

func TestFormatUploadedFilesForSystemPrompt_EmptyFiles(t *testing.T) {
	meta := &UploadMeta{Files: []UploadMetaFile{}}
	if got := FormatUploadedFilesForSystemPrompt(meta); got != "" {
		t.Errorf("expected empty string for empty files, got %q", got)
	}
}

func TestFormatUploadedFilesForSystemPrompt_NormalFiles(t *testing.T) {
	meta := &UploadMeta{
		Files: []UploadMetaFile{
			{OriginalName: "brief.pdf", SavedName: "brief.pdf", SavedPathRel: "uploads/docs/brief.pdf", Type: "pdf", Pages: 3, CharCount: 5000},
			{OriginalName: "notes.txt", SavedName: "notes.txt", SavedPathRel: "uploads/docs/notes.txt", Type: "text", CharCount: 1200},
		},
	}
	got := FormatUploadedFilesForSystemPrompt(meta)

	if !strings.Contains(got, "uploads/docs/brief.pdf") {
		t.Error("expected brief.pdf path in output")
	}
	if !strings.Contains(got, "3 页") {
		t.Error("expected page count in output")
	}
	if !strings.Contains(got, "5000 字符") {
		t.Errorf("expected char count in output, got: %s", got)
	}
	if !strings.Contains(got, "【项目参考文档】") {
		t.Error("expected header in output")
	}
}

func TestFormatUploadedFilesForSystemPrompt_FileWithError(t *testing.T) {
	meta := &UploadMeta{
		Files: []UploadMetaFile{
			{OriginalName: "corrupt.pdf", SavedName: "corrupt.pdf", SavedPathRel: "", Type: "error", Error: "PDF parsing failed"},
		},
	}
	got := FormatUploadedFilesForSystemPrompt(meta)

	if !strings.Contains(got, "解析问题：PDF parsing failed") {
		t.Errorf("expected error detail in output, got: %s", got)
	}
}

func TestFormatUploadedFilesForSystemPrompt_ImageFile(t *testing.T) {
	meta := &UploadMeta{
		Files: []UploadMetaFile{
			{OriginalName: "logo.png", SavedName: "logo.png", SavedPathRel: "uploads/assets/logo.png", Type: "image", Width: 800, Height: 600, Format: "png"},
		},
	}
	got := FormatUploadedFilesForSystemPrompt(meta)

	if !strings.Contains(got, "uploads/assets/logo.png") {
		t.Error("expected image path in output")
	}
	if !strings.Contains(got, "800×600") {
		t.Error("expected dimensions in output")
	}
}

func TestFormatUploadedFilesForSystemPrompt_FallbackToSavedName(t *testing.T) {
	meta := &UploadMeta{
		Files: []UploadMetaFile{
			{OriginalName: "doc.pdf", SavedName: "doc.pdf", SavedPathRel: "", Type: "pdf", Pages: 1},
		},
	}
	got := FormatUploadedFilesForSystemPrompt(meta)

	if !strings.Contains(got, "doc.pdf") {
		t.Error("expected fallback to saved_name in output")
	}
}

func TestFormatUploadedFilesForSystemPrompt_MixedTypes(t *testing.T) {
	meta := &UploadMeta{
		Files: []UploadMetaFile{
			{OriginalName: "a.md", SavedName: "a.md", SavedPathRel: "uploads/docs/a.md", Type: "markdown", CharCount: 300},
			{OriginalName: "b.jpg", SavedName: "b.jpg", SavedPathRel: "uploads/assets/b.jpg", Type: "image", Width: 400, Height: 300, Format: "jpeg"},
			{OriginalName: "c.xyz", SavedName: "c.xyz", SavedPathRel: "", Type: "unsupported"},
		},
	}
	got := FormatUploadedFilesForSystemPrompt(meta)

	if !strings.Contains(got, "markdown") {
		t.Error("expected markdown type")
	}
	if !strings.Contains(got, "image") {
		t.Error("expected image type")
	}
	if !strings.Contains(got, "unsupported") {
		t.Error("expected unsupported type")
	}
}
