package main

import (
	"testing"

	"agentgo/internal/document"
)

func TestMergeUploadMeta_EmptyExisting(t *testing.T) {
	existing := document.UploadMeta{
		UploadID: "upl_old",
		Files:    []document.UploadMetaFile{},
	}
	incoming := document.UploadMeta{
		UploadID: "upl_new",
		Files: []document.UploadMetaFile{
			{OriginalName: "a.md", SavedName: "a.md", SavedPathRel: "uploads/docs/a.md", Type: "markdown"},
		},
	}
	merged := mergeUploadMeta(existing, incoming)

	if len(merged.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(merged.Files))
	}
	if merged.Files[0].OriginalName != "a.md" {
		t.Errorf("expected a.md, got %s", merged.Files[0].OriginalName)
	}
	if merged.UploadID != "upl_old" {
		t.Errorf("expected upload_id preserved, got %s", merged.UploadID)
	}
	if merged.UpdatedAt == "" {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestMergeUploadMeta_OverwriteByName(t *testing.T) {
	existing := document.UploadMeta{
		UploadID: "upl_old",
		Files: []document.UploadMetaFile{
			{OriginalName: "a.md", SavedName: "a.md", SavedPathRel: "uploads/docs/a.md", Type: "markdown", CharCount: 100},
			{OriginalName: "b.txt", SavedName: "b.txt", SavedPathRel: "uploads/docs/b.txt", Type: "text", CharCount: 200},
		},
	}
	incoming := document.UploadMeta{
		UploadID: "upl_new",
		Files: []document.UploadMetaFile{
			{OriginalName: "a.md", SavedName: "a.md", SavedPathRel: "uploads/docs/a.md", Type: "markdown", CharCount: 999}, // overwrite
		},
	}
	merged := mergeUploadMeta(existing, incoming)

	if len(merged.Files) != 2 {
		t.Fatalf("expected 2 files after merge, got %d", len(merged.Files))
	}

	var aFile document.UploadMetaFile
	for _, f := range merged.Files {
		if f.SavedName == "a.md" {
			aFile = f
		}
	}
	if aFile.CharCount != 999 {
		t.Errorf("expected overwritten char_count=999, got %d", aFile.CharCount)
	}
}

func TestMergeUploadMeta_AppendNew(t *testing.T) {
	existing := document.UploadMeta{
		UploadID: "upl_old",
		Files: []document.UploadMetaFile{
			{OriginalName: "a.md", SavedName: "a.md", SavedPathRel: "uploads/docs/a.md", Type: "markdown"},
		},
	}
	incoming := document.UploadMeta{
		UploadID: "upl_new",
		Files: []document.UploadMetaFile{
			{OriginalName: "b.pdf", SavedName: "b.pdf", SavedPathRel: "uploads/docs/b.pdf", Type: "pdf", Pages: 5},
		},
	}
	merged := mergeUploadMeta(existing, incoming)

	if len(merged.Files) != 2 {
		t.Fatalf("expected 2 files after merge, got %d", len(merged.Files))
	}

	names := make(map[string]bool)
	for _, f := range merged.Files {
		names[f.SavedName] = true
	}
	if !names["a.md"] || !names["b.pdf"] {
		t.Errorf("expected both a.md and b.pdf in merged, got %v", names)
	}
}

func TestMergeUploadMeta_UpdatedAtChanges(t *testing.T) {
	existing := document.UploadMeta{
		UploadID: "upl_old",
		Files:    []document.UploadMetaFile{},
	}
	incoming := document.UploadMeta{
		UploadID: "upl_new",
		Files: []document.UploadMetaFile{
			{OriginalName: "x.txt", SavedName: "x.txt", SavedPathRel: "uploads/docs/x.txt", Type: "text"},
		},
	}
	merged := mergeUploadMeta(existing, incoming)

	if merged.UpdatedAt == existing.UpdatedAt {
		t.Error("expected UpdatedAt to differ from existing")
	}
}
