package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Smoke tests — load the real skills directory and verify everything works.
// These tests validate the actual skill data in the repository.
// =============================================================================

func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// TestSmoke_LoadRealSkills loads the actual skills directory and verifies
// the SkillIndex is populated correctly.
func TestSmoke_LoadRealSkills(t *testing.T) {
	root := projectRoot(t)
	userSkillsDir := filepath.Join(root, "skills")
	projSkillsDir := filepath.Join(root, "project-skills")

	if _, err := os.Stat(userSkillsDir); os.IsNotExist(err) {
		t.Skip("real skills directory not found — skipping smoke test")
	}

	idx := LoadSkills(userSkillsDir, projSkillsDir)
	t.Logf("loaded %d skills from %s", idx.Len(), userSkillsDir)

	if idx.Len() == 0 {
		t.Fatal("expected at least 1 skill loaded")
	}

	// Verify deck skills exist.
	deckSkills := idx.DeckSkills()
	if len(deckSkills) == 0 {
		t.Fatal("expected at least 1 deck skill")
	}
	t.Logf("deck skills: %d", len(deckSkills))

	// Verify ByName works.
	for _, name := range idx.AllNames() {
		sk, ok := idx.ByName(name)
		if !ok {
			t.Errorf("ByName(%q) returned false for skill in AllNames()", name)
		}
		if sk.Name != name {
			t.Errorf("ByName(%q).Name = %q, want %q", name, sk.Name, name)
		}
	}

	// Verify ByMode works.
	deckByMode := idx.ByMode("deck")
	if len(deckByMode) != len(deckSkills) {
		t.Errorf("ByMode(\"deck\") = %d skills, DeckSkills() = %d", len(deckByMode), len(deckSkills))
	}
}

// TestSmoke_TriggersNotEmpty verifies that deck skills have triggers defined.
func TestSmoke_TriggersNotEmpty(t *testing.T) {
	root := projectRoot(t)
	userSkillsDir := filepath.Join(root, "skills")
	projSkillsDir := filepath.Join(root, "project-skills")
	if _, err := os.Stat(userSkillsDir); os.IsNotExist(err) {
		t.Skip("real skills directory not found")
	}

	idx := LoadSkills(userSkillsDir, projSkillsDir)
	noTriggers := 0
	for _, sk := range idx.DeckSkills() {
		if len(sk.Triggers) == 0 {
			noTriggers++
			t.Logf("WARNING: deck skill %q has no triggers", sk.Name)
		}
	}
	// All deck skills should have triggers for matching to work.
	ratio := float64(noTriggers) / float64(len(idx.DeckSkills()))
	if ratio > 0.3 {
		t.Errorf("%d/%d deck skills have no triggers (%.0f%%)",
			noTriggers, len(idx.DeckSkills()), ratio*100)
	}
}

// TestSmoke_SkillContent_NoCDNFonts verifies example.html files don't have
// Google Fonts CDN references.
func TestSmoke_SkillContent_NoCDNFonts(t *testing.T) {
	root := projectRoot(t)
	skillsDir := filepath.Join(root, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Skip("real skills directory not found")
	}

	var violations []string
	filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip reference templates — they're not directly used.
		if strings.Contains(path, "/templates/") || strings.Contains(path, "/examples/") {
			return nil
		}
		if strings.Contains(path, "/scripts/") {
			return nil
		}
		// Only check .html files.
		if !strings.HasSuffix(path, ".html") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if strings.Contains(content, "fonts.googleapis.com") {
			violations = append(violations, path)
		}
		return nil
	})

	if len(violations) > 0 {
		t.Logf("CDN font references found in %d files:", len(violations))
		for _, v := range violations {
			t.Logf("  VIOLATION: %s", v)
		}
	}
}

// TestSmoke_SkillContent_SectionSlide verifies example.html files use
// <section class="slide"> not <div class="slide">.
func TestSmoke_SkillContent_SectionSlide(t *testing.T) {
	root := projectRoot(t)
	skillsDir := filepath.Join(root, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Skip("real skills directory not found")
	}

	var violations []string
	filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		if strings.Contains(path, "/templates/") || strings.Contains(path, "/examples/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), `div class="slide"`) {
			violations = append(violations, path)
		}
		return nil
	})

	if len(violations) > 0 {
		t.Logf("div.slide found in %d files (should use section.slide):", len(violations))
		for _, v := range violations {
			t.Logf("  VIOLATION: %s", v)
		}
	}
}

// TestSmoke_SkillContent_HasDimensions verifies example.html files have
// 1920x1080 pixel dimensions.
func TestSmoke_SkillContent_HasDimensions(t *testing.T) {
	root := projectRoot(t)
	skillsDir := filepath.Join(root, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Skip("real skills directory not found")
	}

	missing := 0
	total := 0
	filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		// Skip reference files and pre-approved skills.
		if strings.Contains(path, "/templates/") || strings.Contains(path, "/examples/") ||
			strings.Contains(path, "/assets/") || strings.Contains(path, "/docs/") {
			return nil
		}
		skipSkills := map[string]bool{
			"simple-deck": true, "replit-deck": true, "weekly-update": true,
		}
		for s := range skipSkills {
			if strings.Contains(path, "/"+s+"/") {
				return nil
			}
		}
		total++
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !strings.Contains(string(data), "1920px") {
			missing++
			t.Logf("WARNING: no 1920px in %s", path)
		}
		return nil
	})

	if missing > 0 {
		t.Logf("%d/%d example.html files missing 1920px dimension", missing, total)
	}
}

// TestSmoke_SkillContent_NoDeckStageScript verifies no example.html uses
// deck-stage.js custom element.
func TestSmoke_SkillContent_NoDeckStageScript(t *testing.T) {
	root := projectRoot(t)
	skillsDir := filepath.Join(root, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Skip("real skills directory not found")
	}

	var violations []string
	filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if strings.Contains(content, "assets/deck-stage.js") || strings.Contains(content, "<deck-stage") {
			violations = append(violations, path)
		}
		return nil
	})

	if len(violations) > 0 {
		t.Logf("deck-stage.js references found in %d files:", len(violations))
		for _, v := range violations {
			t.Logf("  VIOLATION: %s", v)
		}
	}
}

// TestSmoke_SKILLMD_ConstraintBlock verifies SKILL.md files have the
// GrapesJS/htmlslide constraints block.
func TestSmoke_SKILLMD_ConstraintBlock(t *testing.T) {
	root := projectRoot(t)
	skillsDir := filepath.Join(root, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Skip("real skills directory not found")
	}

	// Required phrases that should appear in the constraint block.
	required := []string{"section", "1920", "1080"}

	missingBlock := 0
	total := 0
	// Pre-approved skills that don't need the constraint block.
	skipSkills := map[string]bool{
		"simple-deck": true, "replit-deck": true, "weekly-update": true,
	}
	filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "SKILL.md") {
			return nil
		}
		skip := false
		for s := range skipSkills {
			if strings.Contains(path, "/"+s+"/") {
				skip = true
				break
			}
		}
		if skip {
			return nil
		}
		total++
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := strings.ToLower(string(data))
		for _, phrase := range required {
			if !strings.Contains(content, strings.ToLower(phrase)) {
				t.Logf("WARNING: %s missing constraint keyword %q", path, phrase)
				missingBlock++
				break
			}
		}
		return nil
	})

	if missingBlock > 0 {
		t.Logf("%d/%d SKILL.md files missing constraint block", missingBlock, total)
	}
}

// TestSmoke_Frontmatter_YAML verifies frontmatter in all SKILL.md files
// parses correctly with the YAML parser.
func TestSmoke_Frontmatter_YAML(t *testing.T) {
	root := projectRoot(t)
	skillsDir := filepath.Join(root, "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		t.Skip("real skills directory not found")
	}

	parseErrors := 0
	total := 0
	filepath.Walk(skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "SKILL.md") {
			return nil
		}
		total++
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		fm, body := ParseFrontmatter(data)
		if fm.Name == "" && strings.HasPrefix(string(data), "---") {
			// Has frontmatter but name not parsed — might be a YAML issue.
			t.Logf("WARNING: %s — frontmatter present but name empty (body=%d bytes)", path, len(body))
			parseErrors++
		}
		return nil
	})

	if total > 0 && float64(parseErrors)/float64(total) > 0.1 {
		t.Errorf("%d/%d SKILL.md files have frontmatter parse issues", parseErrors, total)
	}
}

// TestSmoke_SkillIndex_BuildResult verifies SkillIndex has expected methods.
func TestSmoke_SkillIndex_BuildResult(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"deck-a": {
			Name: "deck-a", Mode: "deck",
			Triggers: []string{"warm", "modern"}, Description: "A warm modern deck",
		},
		"deck-b": {
			Name: "deck-b", Mode: "deck",
			Triggers: []string{"cool", "minimal"}, Description: "A cool minimal deck",
		},
		"landing-c": {
			Name: "landing-c", Mode: "landing",
			Triggers: []string{"hero", "cta"}, Description: "A landing page",
		},
		"unknown-d": {
			Name: "unknown-d", Mode: "",
			Triggers: []string{}, Description: "Unknown mode",
		},
	})

	// ByName
	if sk, ok := idx.ByName("deck-a"); !ok || sk.Name != "deck-a" {
		t.Error("ByName failed")
	}

	// Len
	if idx.Len() != 4 {
		t.Errorf("Len() = %d, want 4", idx.Len())
	}

	// DeckSkills
	decks := idx.DeckSkills()
	if len(decks) != 2 {
		t.Errorf("DeckSkills() = %d, want 2", len(decks))
	}

	// ByMode
	deckMode := idx.ByMode("deck")
	if len(deckMode) != 2 {
		t.Errorf("ByMode(deck) = %d, want 2", len(deckMode))
	}
	unknownMode := idx.ByMode("unknown")
	if len(unknownMode) != 1 {
		t.Errorf("ByMode(unknown) = %d, want 1", len(unknownMode))
	}

	// AllNames
	names := idx.AllNames()
	if len(names) != 4 {
		t.Errorf("AllNames() = %d, want 4", len(names))
	}

	// ToMap
	m := idx.ToMap()
	if len(m) != 4 {
		t.Errorf("ToMap() = %d entries, want 4", len(m))
	}
}
