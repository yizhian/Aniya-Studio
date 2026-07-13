package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePreviewRel(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "example.html"), []byte("<html></html>"), 0644)

	sk := Skill{Name: "presenter-mode", DirPath: dir, PreviewEntry: "index.html"}
	rel := ResolvePreviewRel(sk.DirPath, sk.PreviewEntry)
	if rel != "example.html" {
		t.Fatalf("expected example.html fallback, got %q", rel)
	}
	if !HasPreview(sk) {
		t.Fatal("expected HasPreview true")
	}
}

func TestResolvePreviewRel_NameDirMismatch(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "guizang-ppt")
	os.MkdirAll(filepath.Join(skillDir, "assets"), 0755)
	os.WriteFile(filepath.Join(skillDir, "assets", "example-slides.html"), []byte("<html></html>"), 0644)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: magazine-web-ppt\nod:\n  preview:\n    entry: index.html\n---\n"), 0644)

	skills := LoadSkills(dir, "/nonexistent")
	sk, ok := skills.ByName("magazine-web-ppt")
	if !ok {
		t.Fatal("expected magazine-web-ppt skill")
	}
	if !sk.HasPreview {
		t.Fatal("expected HasPreview true for assets/example-slides.html")
	}
	path := ResolvePreviewPath(sk)
	if filepath.Base(path) != "example-slides.html" {
		t.Fatalf("unexpected preview path %q", path)
	}
}

func TestLoadSkills_RealDeckPreviews(t *testing.T) {
	skillsDir := filepath.Join("..", "..", "..", "..", "skills")
	if _, err := os.Stat(skillsDir); err != nil {
		t.Skip("real skills directory not available")
	}
	idx := LoadSkills(skillsDir, "/nonexistent")
	cases := map[string]bool{
		"magazine-web-ppt":        true,
		"html-ppt-presenter-mode": true,
		"replit-deck":             true,
		"open-design-landing-deck": true,
		"html-ppt":                false,
	}
	for name, want := range cases {
		sk, ok := idx.ByName(name)
		if !ok {
			t.Fatalf("skill %q not loaded", name)
		}
		if sk.HasPreview != want {
			t.Errorf("%s: HasPreview=%v, want %v (path=%q)", name, sk.HasPreview, want, ResolvePreviewPath(sk))
		}
	}
}

func TestResolvePreviewRel_ReplitDeck(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "replit-deck")
	os.MkdirAll(filepath.Join(skillDir, "examples"), 0755)
	os.WriteFile(filepath.Join(skillDir, "examples", "example-helix.html"), []byte("<html></html>"), 0644)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: replit-deck\n---\n"), 0644)

	skills := LoadSkills(dir, "/nonexistent")
	sk, ok := skills.ByName("replit-deck")
	if !ok {
		t.Fatal("expected replit-deck skill")
	}
	if !sk.HasPreview {
		t.Fatal("expected HasPreview true")
	}
}
