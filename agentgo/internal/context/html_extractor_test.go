package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractDesignSnapshot_Basic(t *testing.T) {
	ws := t.TempDir()
	htmlContent := `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>My Slide Deck</title>
	<link rel="stylesheet" href="https://fonts.googleapis.com/css?family=Roboto|Open+Sans">
	<style>
	 :root {
	  }
	</style>
	<style>
	   --bg: #ffffff;
	   --fg: #333333;
	   --accent: #0066cc;
	</style>
</head>
<body>
<h1>Welcome</h1>
<h2>Agenda</h2>
<h3>Details</h3>
<section class="slide slide-1">
	<p>Content</p>
</section>
<section class="slide slide-2">
	<p>More content</p>
</section>
</body>
</html>`
	htmlPath := filepath.Join(ws, "deck.html")
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	snap, err := ExtractDesignSnapshot(htmlPath)
	if err != nil {
		t.Fatalf("ExtractDesignSnapshot failed: %v", err)
	}
	if snap.Title != "My Slide Deck" {
		t.Fatalf("expected title 'My Slide Deck', got %q", snap.Title)
	}
	if snap.SlideCount != 2 {
		t.Fatalf("expected SlideCount=2, got %d", snap.SlideCount)
	}
	if len(snap.SlideHeadings) < 3 {
		t.Fatalf("expected at least 3 headings, got %d: %v", len(snap.SlideHeadings), snap.SlideHeadings)
	}

	// Check fonts.
	if len(snap.Fonts) < 1 {
		t.Fatalf("expected at least 1 font, got %d", len(snap.Fonts))
	}
	foundRoboto := false
	for _, f := range snap.Fonts {
		if strings.Contains(f.Family, "Roboto") || strings.Contains(f.Family, "Open Sans") {
			foundRoboto = true
		}
	}
	if !foundRoboto {
		t.Fatalf("expected Roboto or Open Sans in fonts, got %+v", snap.Fonts)
	}

	// Check colors.
	if len(snap.ColorPalette) < 3 {
		t.Fatalf("expected at least 3 color vars, got %d: %v", len(snap.ColorPalette), snap.ColorPalette)
	}
	if snap.ColorPalette["--bg"] != "#ffffff" {
		t.Fatalf("expected --bg=#ffffff, got %q", snap.ColorPalette["--bg"])
	}

	// Check file size.
	if snap.FileSizeBytes <= 0 {
		t.Fatalf("expected positive file size, got %d", snap.FileSizeBytes)
	}
}

func TestExtractDesignSnapshot_Empty(t *testing.T) {
	ws := t.TempDir()
	htmlPath := filepath.Join(ws, "empty.html")
	os.WriteFile(htmlPath, []byte("<html></html>"), 0o644)

	snap, err := ExtractDesignSnapshot(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	if snap.SlideCount != 0 {
		t.Fatalf("expected SlideCount=0 for empty HTML, got %d", snap.SlideCount)
	}
}

func TestExtractDesignSnapshot_MissingFile(t *testing.T) {
	_, err := ExtractDesignSnapshot("/nonexistent/file.html")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestExtractDesignSnapshot_SlideCounting(t *testing.T) {
	ws := t.TempDir()
	htmlContent := `<html><body>
		<section class="slide"></section>
		<section class="slide-1"></section>
		<div class="my-slide"></div>
		<section class="not-a-slide"></section>
		<div class="slide-wrapper"></div>
	</body></html>`
	htmlPath := filepath.Join(ws, "slides.html")
	os.WriteFile(htmlPath, []byte(htmlContent), 0o644)

	snap, err := ExtractDesignSnapshot(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	// "slide", "slide-1" match; "my-slide" matches (contains "slide-"); "not-a-slide" matches "slide-"?
	// "not-a-slide" contains "slide-"? Let me check: "not-a-slide" -> lower "not-a-slide" contains "slide-" -> YES
	// "slide-wrapper" -> contains "slide-" -> YES
	// So all 5 should match except the one that doesn't have class containing "slide"
	// Actually: countSlides checks strings.Contains(secClass, "slide") || strings.Contains(secClass, "slide-")
	// "slide" -> contains "slide" -> yes
	// "slide-1" -> contains "slide" and "slide-" -> yes
	// "my-slide" -> contains "slide" -> yes (my-slide contains "slide")
	// "not-a-slide" -> contains "slide" -> wait, "not-a-slide" contains "slide" as substring -> YES
	// "slide-wrapper" -> contains "slide" and "slide-" -> yes
	// So all 5 sections count as slides
	if snap.SlideCount != 5 {
		t.Fatalf("expected SlideCount=5, got %d", snap.SlideCount)
	}
}

func TestExtractDesignSnapshot_CSSClasses(t *testing.T) {
	ws := t.TempDir()
	htmlContent := `<html><body>
		<div class="container main"></div>
		<section class="slide hero"></section>
		<footer class="footer"></footer>
	</body></html>`
	htmlPath := filepath.Join(ws, "classes.html")
	os.WriteFile(htmlPath, []byte(htmlContent), 0o644)

	snap, err := ExtractDesignSnapshot(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	// Collect unique classes: container, main, slide, hero, footer
	expected := map[string]bool{"container": true, "main": true, "slide": true, "hero": true, "footer": true}
	seen := make(map[string]bool)
	for _, c := range snap.CSSClasses {
		seen[c] = true
	}
	for exp := range expected {
		if !seen[exp] {
			t.Fatalf("expected CSS class %q in output, got %v", exp, snap.CSSClasses)
		}
	}
}

func TestExtractDesignSnapshot_GoogleFontImport(t *testing.T) {
	ws := t.TempDir()
	htmlContent := `<html><head>
	<style>
	 @import url('https://fonts.googleapis.com/css?family=Lato');
	</style></head><body></body></html>`
	htmlPath := filepath.Join(ws, "import.html")
	os.WriteFile(htmlPath, []byte(htmlContent), 0o644)

	snap, err := ExtractDesignSnapshot(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range snap.Fonts {
		if strings.Contains(f.Family, "Lato") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Lato font from @import, got %+v", snap.Fonts)
	}
}
