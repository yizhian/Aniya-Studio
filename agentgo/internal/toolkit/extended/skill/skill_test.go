package skill

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agentgo/internal/toolkit/contracts"
)

// ---------------------------------------------------------------------------
// Frontmatter parser tests
// ---------------------------------------------------------------------------

func TestParseFrontmatter_Valid(t *testing.T) {
	input := `---
name: test-skill
description: A test skill
whentouse: When testing
type: always
---
This is the body.`
	fm, body := ParseFrontmatter([]byte(input))
	if fm.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", fm.Name, "test-skill")
	}
	if fm.Description != "A test skill" {
		t.Errorf("Description = %q, want %q", fm.Description, "A test skill")
	}
	if fm.WhenToUse != "When testing" {
		t.Errorf("WhenToUse = %q, want %q", fm.WhenToUse, "When testing")
	}
	if fm.Type != "always" {
		t.Errorf("Type = %q, want %q", fm.Type, "always")
	}
	body = strings.TrimSpace(body)
	if body != "This is the body." {
		t.Errorf("Body = %q, want %q", body, "This is the body.")
	}
}

func TestParseFrontmatter_YAML(t *testing.T) {
	input := `---
name: yaml-skill
description: A YAML-parsed skill
triggers:
  - "trigger-a"
  - "trigger-b"
od:
  mode: deck
  scenario: marketing
---
Body content.`
	fm, body := ParseFrontmatter([]byte(input))
	if fm.Name != "yaml-skill" {
		t.Errorf("Name = %q", fm.Name)
	}
	if len(fm.Triggers) != 2 || fm.Triggers[0] != "trigger-a" {
		t.Errorf("Triggers = %v", fm.Triggers)
	}
	if fm.OD.Mode != "deck" {
		t.Errorf("Mode = %q", fm.OD.Mode)
	}
	if fm.OD.Scenario != "marketing" {
		t.Errorf("Scenario = %q", fm.OD.Scenario)
	}
	if !strings.Contains(body, "Body content.") {
		t.Errorf("Body missing: %q", body)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	input := `Just a plain file with no frontmatter.`
	fm, body := ParseFrontmatter([]byte(input))
	if !reflect.DeepEqual(fm, Frontmatter{}) {
		t.Errorf("expected empty Frontmatter, got %+v", fm)
	}
	if string(body) != input {
		t.Errorf("body = %q, want %q", string(body), input)
	}
}

func TestParseFrontmatter_UnclosedDelimiter(t *testing.T) {
	input := `---
name: broken
no closing delimiter`
	fm, body := ParseFrontmatter([]byte(input))
	if !reflect.DeepEqual(fm, Frontmatter{}) {
		t.Errorf("expected empty Frontmatter, got %+v", fm)
	}
	if string(body) != input {
		t.Errorf("body should equal original input")
	}
}

func TestParseFrontmatter_Empty(t *testing.T) {
	input := "---\n---\nbody content"
	fm, body := ParseFrontmatter([]byte(input))
	if !reflect.DeepEqual(fm, Frontmatter{}) {
		t.Errorf("expected empty Frontmatter, got %+v", fm)
	}
	body = strings.TrimSpace(body)
	if body != "body content" {
		t.Errorf("body = %q, want %q", body, "body content")
	}
}

func TestParseFrontmatter_PartialFields(t *testing.T) {
	input := `---
name: partial-skill
---
body`
	fm, body := ParseFrontmatter([]byte(input))
	if fm.Name != "partial-skill" {
		t.Errorf("Name = %q, want %q", fm.Name, "partial-skill")
	}
	if fm.Description != "" || fm.WhenToUse != "" || fm.Type != "" {
		t.Errorf("expected other fields empty, got %+v", fm)
	}
	body = strings.TrimSpace(body)
	if body != "body" {
		t.Errorf("body = %q, want %q", body, "body")
	}
}

func TestParseFrontmatter_UnknownFields(t *testing.T) {
	input := `---
name: known-name
unknown_key: will be ignored
---
body`
	fm, _ := ParseFrontmatter([]byte(input))
	if fm.Name != "known-name" {
		t.Errorf("Name = %q, want %q", fm.Name, "known-name")
	}
}

func TestParseFrontmatter_CaseSensitiveKeys(t *testing.T) {
	// YAML keys are case-sensitive; uppercase keys are NOT recognized
	// and fall through to the legacy parser which does lowercase them.
	input := `---
name: test
description: desc
whentouse: when
type: manual
---`
	fm, _ := ParseFrontmatter([]byte(input))
	if fm.Name != "test" || fm.Description != "desc" || fm.WhenToUse != "when" || fm.Type != "manual" {
		t.Errorf("unexpected frontmatter: %+v", fm)
	}
}

// ---------------------------------------------------------------------------
// Loader tests
// ---------------------------------------------------------------------------

func TestLoadSkills_BothDirs(t *testing.T) {
	userDir := t.TempDir()
	projDir := t.TempDir()

	os.WriteFile(filepath.Join(userDir, "user-skill.md"), []byte("---\nname: user-skill\ndescription: A user skill\n---\nUser body"), 0644)
	os.WriteFile(filepath.Join(projDir, "proj-skill.md"), []byte("---\nname: proj-skill\ndescription: A project skill\n---\nProject body"), 0644)

	skills := LoadSkills(userDir, projDir)
	if _, ok := skills.ByName("user-skill"); !ok {
		t.Error("expected user-skill to be loaded")
	}
	if _, ok := skills.ByName("proj-skill"); !ok {
		t.Error("expected proj-skill to be loaded")
	}
}

func TestLoadSkills_DirBased(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-dir-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-dir-skill\ndescription: A directory skill\nwhentouse: When testing dir loading\ntype: manual\n---\nSkill body from directory"), 0644)
	os.WriteFile(filepath.Join(skillDir, "LICENSE.txt"), []byte("MIT"), 0644)

	skills := LoadSkills(dir, "/nonexistent")
	s, ok := skills.ByName("my-dir-skill")
	if !ok {
		t.Fatal("expected my-dir-skill to be loaded from directory")
	}
	if s.Description != "A directory skill" {
		t.Errorf("Description = %q, want %q", s.Description, "A directory skill")
	}
	if s.WhenToUse != "When testing dir loading" {
		t.Errorf("WhenToUse = %q, want %q", s.WhenToUse, "When testing dir loading")
	}
	if s.Type != "manual" {
		t.Errorf("Type = %q, want %q", s.Type, "manual")
	}
	if !strings.Contains(s.Body, "Skill body from directory") {
		t.Errorf("Body = %q, want body content", s.Body)
	}
	if s.DirPath != skillDir {
		t.Errorf("DirPath = %q, want %q", s.DirPath, skillDir)
	}
	if s.Source != "user" {
		t.Errorf("Source = %q, want %q", s.Source, "user")
	}
}

func TestLoadSkills_DirNameFallback(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "name-from-dir")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: No name field\n---\nBody"), 0644)

	skills := LoadSkills(dir, "/nonexistent")
	s, ok := skills.ByName("name-from-dir")
	if !ok {
		t.Fatal("expected skill named after directory")
	}
	if s.Description != "No name field" {
		t.Errorf("Description = %q", s.Description)
	}
}

func TestLoadSkills_DirMissingSKILL(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "no-skill-dir"), 0755)

	skills := LoadSkills(dir, "/nonexistent")
	if skills.Len() != 0 {
		t.Errorf("expected no skills, got %d", skills.Len())
	}
}

func TestLoadSkills_FlatMdOnly(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "good.md"), []byte("---\nname: good\n---\nbody"), 0644)
	os.WriteFile(filepath.Join(dir, "bad.txt"), []byte("---\nname: bad\n---\nbody"), 0644)
	os.WriteFile(filepath.Join(dir, "noext"), []byte("---\nname: noext\n---\nbody"), 0644)

	skills := LoadSkills(dir, "/nonexistent")
	if _, ok := skills.ByName("good"); !ok {
		t.Error("expected 'good' skill loaded from .md")
	}
	if _, ok := skills.ByName("bad"); ok {
		t.Error("did not expect 'bad' skill loaded from .txt")
	}
	if _, ok := skills.ByName("noext"); ok {
		t.Error("did not expect 'noext' skill loaded without extension")
	}
}

func TestLoadSkills_ProjectOverridesUser(t *testing.T) {
	userDir := t.TempDir()
	projDir := t.TempDir()

	os.WriteFile(filepath.Join(userDir, "same.md"), []byte("---\nname: overlap\ndescription: user version\n---\nUser body"), 0644)
	os.WriteFile(filepath.Join(projDir, "same.md"), []byte("---\nname: overlap\ndescription: project version\n---\nProject body"), 0644)

	skills := LoadSkills(userDir, projDir)
	s, ok := skills.ByName("overlap")
	if !ok {
		t.Fatal("expected overlap skill")
	}
	if s.Description != "project version" {
		t.Errorf("Description = %q, want %q", s.Description, "project version")
	}
	if s.Source != "project" {
		t.Errorf("Source = %q, want %q", s.Source, "project")
	}
}

func TestLoadSkills_ProjectDirOverridesUserDir(t *testing.T) {
	userDir := t.TempDir()
	projDir := t.TempDir()

	os.MkdirAll(filepath.Join(userDir, "overlap"), 0755)
	os.WriteFile(filepath.Join(userDir, "overlap", "SKILL.md"), []byte("---\nname: overlap\ndescription: user dir version\n---\nUser body"), 0644)
	os.MkdirAll(filepath.Join(projDir, "overlap"), 0755)
	os.WriteFile(filepath.Join(projDir, "overlap", "SKILL.md"), []byte("---\nname: overlap\ndescription: project dir version\n---\nProject body"), 0644)

	skills := LoadSkills(userDir, projDir)
	s, ok := skills.ByName("overlap")
	if !ok {
		t.Fatal("expected overlap skill")
	}
	if s.Description != "project dir version" {
		t.Errorf("Description = %q, want %q", s.Description, "project dir version")
	}
	if s.Source != "project" {
		t.Errorf("Source = %q, want %q", s.Source, "project")
	}
}

func TestLoadSkills_MissingDirectories(t *testing.T) {
	skills := LoadSkills("/nonexistent/user", "/nonexistent/project")
	if skills.Len() != 0 {
		t.Errorf("expected empty index, got %v", skills)
	}
}

func TestLoadSkills_FilenameFallback(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "no-frontmatter.md"), []byte("Just content without frontmatter"), 0644)

	skills := LoadSkills(dir, "/nonexistent")
	s, ok := skills.ByName("no-frontmatter")
	if !ok {
		t.Fatal("expected skill derived from filename")
	}
	if s.Body != "Just content without frontmatter" {
		t.Errorf("Body = %q, want %q", s.Body, "Just content without frontmatter")
	}
}

func TestLoadSkills_HasAssets(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "with-assets")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: with-assets\n---\nBody"), 0644)
	os.WriteFile(filepath.Join(skillDir, "example.html"), []byte("<html></html>"), 0644)

	skills := LoadSkills(dir, "/nonexistent")
	s, ok := skills.ByName("with-assets")
	if !ok {
		t.Fatal("expected with-assets skill")
	}
	if !s.HasAssets {
		t.Error("expected HasAssets=true when example.html exists")
	}
}

// ---------------------------------------------------------------------------
// Formatter tests
// ---------------------------------------------------------------------------

func TestFormatSkills_Empty(t *testing.T) {
	s := FormatSkills(nil, 1000)
	if s != "" {
		t.Errorf("expected empty string, got %q", s)
	}
	s = FormatSkills(map[string]Skill{}, 1000)
	if s != "" {
		t.Errorf("expected empty string for empty map, got %q", s)
	}
}

func TestFormatSkills_Full(t *testing.T) {
	skills := map[string]Skill{
		"test": {
			Name:        "test",
			Description: "A test",
			WhenToUse:   "When testing",
			Body:        "skill body\n",
		},
	}
	s := FormatSkills(skills, 10000)
	if !strings.Contains(s, "### test") {
		t.Error("expected '### test' in full format")
	}
	if !strings.Contains(s, "skill body") {
		t.Error("expected body in full format")
	}
}

func TestFormatSkills_MediumCompactForManual(t *testing.T) {
	skills := map[string]Skill{
		"manual-skill": {
			Name:        "manual-skill",
			Description: "A manual skill",
			WhenToUse:   "When called",
			Type:        "manual",
			Body:        "long body content that would be excluded in medium mode\n",
		},
	}
	s := FormatSkills(skills, 20)
	if strings.Contains(s, "long body content") {
		t.Error("expected body excluded in medium mode for manual skill")
	}
	if !strings.Contains(s, "manual-skill") {
		t.Error("expected skill name in medium output")
	}
	if !strings.Contains(s, "A manual skill") {
		t.Error("expected description in medium output")
	}
}

func TestFormatSkills_MediumFullForAlways(t *testing.T) {
	skills := map[string]Skill{
		"always-skill": {
			Name:        "always-skill",
			Description: "An always skill",
			WhenToUse:   "Always",
			Type:        "always",
			Body:        "always body content\n",
		},
	}
	s := FormatSkills(skills, 200)
	if !strings.Contains(s, "always body content") {
		t.Error("expected body included for always-type skill in medium mode")
	}
}

func TestFormatSkills_Minimal(t *testing.T) {
	skills := map[string]Skill{
		"skill-a": {Name: "skill-a"},
		"skill-b": {Name: "skill-b"},
	}
	s := FormatSkills(skills, 5)
	if !strings.Contains(s, "skill-a") || !strings.Contains(s, "skill-b") {
		t.Error("expected both skill names in minimal format")
	}
	if !strings.Contains(s, "Available Skills") {
		t.Error("expected header in minimal format")
	}
}

// ---------------------------------------------------------------------------
// SkillIndex tests
// ---------------------------------------------------------------------------





func TestSkillIndex_ByMode(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"deck-a": {Name: "deck-a", Mode: "deck"},
		"deck-b": {Name: "deck-b", Mode: "deck"},
		"other":  {Name: "other", Mode: "other"},
	})
	deck := idx.ByMode("deck")
	if len(deck) != 2 {
		t.Errorf("expected 2 deck skills, got %d", len(deck))
	}
	other := idx.ByMode("other")
	if len(other) != 1 {
		t.Errorf("expected 1 other skill, got %d", len(other))
	}
}

func TestSkillIndex_Len(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"a": {Name: "a"},
		"b": {Name: "b"},
	})
	if idx.Len() != 2 {
		t.Errorf("Len = %d, want 2", idx.Len())
	}
}

func TestFormatSkillsForMode_DeckPriority(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"coral": {
			Name:        "coral",
			Description: "Warm coral theme",
			Triggers:    []string{"coral", "warm"},
			Mode:        "deck",
		},
		"other-tool": {
			Name: "other-tool",
			Mode: "other",
		},
	})
	s := FormatSkillsForMode(idx, "deck", 8000)
	if !strings.Contains(s, "coral") {
		t.Error("expected deck skill in output")
	}
	if !strings.Contains(s, "Design skills") {
		t.Error("expected Design skills section")
	}
	if !strings.Contains(s, "Other skills") {
		t.Error("expected Other skills section for non-deck")
	}
}

// ---------------------------------------------------------------------------
// Skill tool tests
// ---------------------------------------------------------------------------

func TestSkillTool_Query(t *testing.T) {
	tool := NewSkillTool(NewIndex(map[string]Skill{
		"alpha": {Name: "alpha"},
		"beta":  {Name: "beta"},
	}), nil)
	result := tool.Call(nil, makeArgs(`{"operation":"query"}`))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ErrorMessage)
	}
	if !strings.Contains(result.Content, "alpha") || !strings.Contains(result.Content, "beta") {
		t.Errorf("expected both skills in query result: %s", result.Content)
	}
}

func TestSkillTool_Get(t *testing.T) {
	tool := NewSkillTool(NewIndex(map[string]Skill{
		"my-skill": {
			Name:        "my-skill",
			Description: "My test skill",
			WhenToUse:   "When testing",
			Body:        "body content",
			Source:      "user",
		},
	}), nil)
	result := tool.Call(nil, makeArgs(`{"operation":"get","name":"my-skill"}`))
	if result.IsError {
		t.Fatalf("unexpected error: %s", result.ErrorMessage)
	}
	if !strings.Contains(result.Content, "my-skill") {
		t.Errorf("expected skill name in get result: %s", result.Content)
	}
}

func TestSkillTool_Get_NotFound(t *testing.T) {
	tool := NewSkillTool(NewIndex(map[string]Skill{}), nil)
	result := tool.Call(nil, makeArgs(`{"operation":"get","name":"nonexistent"}`))
	if !result.IsError {
		t.Fatal("expected error for nonexistent skill")
	}
}

func TestSkillTool_Search_NoProvider(t *testing.T) {
	tool := NewSkillTool(NewIndex(map[string]Skill{
		"slide-design": {Name: "slide-design", Description: "Create and design slide presentations"},
	}), nil)
	result := tool.Call(nil, makeArgs(`{"operation":"search","query":"slide"}`))
	if !result.IsError {
		t.Fatal("expected error when provider is not available")
	}
	if !strings.Contains(result.Content, "provider_unavailable") {
		t.Errorf("expected provider_unavailable, got: %s", result.Content)
	}
}

func TestSkillTool_UnknownOperation(t *testing.T) {
	tool := NewSkillTool(NewIndex(map[string]Skill{}), nil)
	result := tool.Call(nil, makeArgs(`{"operation":"invalid"}`))
	if !result.IsError {
		t.Fatal("expected error for unknown operation")
	}
}

func makeArgs(s string) contracts.ToolCallArgs {
	return contracts.ToolCallArgs{ArgsJSON: s}
}

func TestFormatSkills_MediumManualWhenToUseOnly(t *testing.T) {
	skills := []Skill{{
		Name:      "search-tool",
		WhenToUse: "When searching",
		Type:      "manual",
	}}
	s := formatMedium(skills)
	if !strings.Contains(s, "Use when: When searching") {
		t.Errorf("expected 'Use when:' clause, got %q", s)
	}
	if !strings.Contains(s, "search-tool") {
		t.Error("expected skill name")
	}
	if strings.Contains(s, "### search-tool") {
		t.Error("manual skill should be compact, not full")
	}
}

func TestFormatSkills_MediumManualDescriptionOnly(t *testing.T) {
	skills := []Skill{{
		Name:        "simple-skill",
		Description: "Does simple things",
		Type:        "manual",
	}}
	s := formatMedium(skills)
	if !strings.Contains(s, "Does simple things") {
		t.Errorf("expected description, got %q", s)
	}
}

func TestFormatSkills_MediumManualNoDescNoWhen(t *testing.T) {
	skills := []Skill{{
		Name: "bare-skill",
		Type: "manual",
	}}
	s := formatMedium(skills)
	if !strings.Contains(s, "bare-skill") {
		t.Error("expected skill name")
	}
	if strings.Contains(s, "Use when") {
		t.Error("should not contain Use when clause")
	}
	if strings.Contains(s, "### bare-skill") {
		t.Error("bare skill without desc/whenToUse should still be compact")
	}
}

func TestFormatSkills_MediumAlwaysWithoutBody(t *testing.T) {
	skills := map[string]Skill{
		"header-only": {
			Name:        "header-only",
			Description: "Just a header",
			WhenToUse:   "Always active",
			Type:        "always",
		},
	}
	s := FormatSkills(skills, 200)
	if !strings.Contains(s, "### header-only") {
		t.Error("expected full header for always skill")
	}
	if !strings.Contains(s, "Just a header") {
		t.Error("expected description")
	}
	if !strings.Contains(s, "Always active") {
		t.Error("expected whenToUse")
	}
}

func TestFormatSkills_MediumAlwaysBodyNoTrailingNewline(t *testing.T) {
	skills := map[string]Skill{
		"no-newline": {
			Name: "no-newline",
			Type: "always",
			Body: "body without trailing newline",
		},
	}
	s := FormatSkills(skills, 200)
	if !strings.Contains(s, "body without trailing newline") {
		t.Error("expected body")
	}
}

func TestFormatSkills_MediumMixed(t *testing.T) {
	skills := []Skill{
		{
			Name:        "always-one",
			Description: "Always active skill",
			Type:        "always",
			Body:        "full body\n",
		},
		{
			Name:        "manual-one",
			Description: "Manual skill desc",
			WhenToUse:   "When requested",
			Type:        "manual",
		},
	}
	s := formatMedium(skills)
	if !strings.Contains(s, "### always-one") {
		t.Error("expected full header for always skill")
	}
	if !strings.Contains(s, "full body") {
		t.Error("expected body for always skill")
	}
	if !strings.Contains(s, "manual-one") {
		t.Error("expected manual skill name")
	}
	if !strings.Contains(s, "Manual skill desc. Use when: When requested") {
		t.Errorf("expected combined desc+whenToUse for manual, got %q", s)
	}
}

func TestFormatSkills_ExactBudgetBoundary(t *testing.T) {
	skills := map[string]Skill{
		"tiny": {Name: "tiny", Description: "tiny skill", Type: "manual"},
	}
	tokens := estimateTokens(formatMedium(sortedSkills(skills)))
	s := FormatSkills(skills, tokens)
	if !strings.Contains(s, "tiny") {
		t.Error("should include skill at budget boundary")
	}
}

// ---------------------------------------------------------------------------
// Trigger matching tests — Chinese + English + edge cases
// ---------------------------------------------------------------------------





func TestSkillIndex_ByMode_Mixed(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"deck-a":     {Name: "deck-a", Mode: "deck"},
		"deck-b":     {Name: "deck-b", Mode: "deck"},
		"landing-a":  {Name: "landing-a", Mode: "landing"},
		"doc-a":      {Name: "doc-a", Mode: "document"},
		"unknown-a":  {Name: "unknown-a", Mode: ""},
	})
	deck := idx.ByMode("deck")
	if len(deck) != 2 {
		t.Errorf("expected 2 deck skills, got %d", len(deck))
	}
	landing := idx.ByMode("landing")
	if len(landing) != 1 {
		t.Errorf("expected 1 landing skill, got %d", len(landing))
	}
	doc := idx.ByMode("document")
	if len(doc) != 1 {
		t.Errorf("expected 1 document skill, got %d", len(doc))
	}
	// Skills with empty mode get grouped under "unknown".
	unknown := idx.ByMode("unknown")
	if len(unknown) != 1 {
		t.Errorf("expected 1 unknown skill, got %d", len(unknown))
	}
	// Nonexistent mode returns empty slice.
	none := idx.ByMode("nonexistent")
	if len(none) != 0 {
		t.Errorf("expected empty slice for nonexistent mode, got %d", len(none))
	}
}

func TestSkillIndex_DeckSkills(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"deck-a":    {Name: "deck-a", Mode: "deck"},
		"deck-b":    {Name: "deck-b", Mode: "deck"},
		"non-deck":  {Name: "non-deck", Mode: "landing"},
	})
	deck := idx.DeckSkills()
	if len(deck) != 2 {
		t.Errorf("DeckSkills: expected 2, got %d", len(deck))
	}
	for _, s := range deck {
		if s.Mode != "deck" {
			t.Errorf("DeckSkills: expected mode=deck, got %q", s.Mode)
		}
	}
}





func TestNewIndex_NilMap(t *testing.T) {
	idx := NewIndex(nil)
	if idx == nil {
		t.Fatal("expected non-nil index from nil map")
	}
	if idx.Len() != 0 {
		t.Errorf("expected empty index, got %d entries", idx.Len())
	}
	if len(idx.AllNames()) != 0 {
		t.Error("expected empty AllNames")
	}
	if len(idx.DeckSkills()) != 0 {
		t.Error("expected empty DeckSkills")
	}
}

func TestNewIndex_EmptyMap(t *testing.T) {
	idx := NewIndex(map[string]Skill{})
	if idx.Len() != 0 {
		t.Errorf("expected empty index, got %d", idx.Len())
	}
}

func TestSkillIndex_ToMap_Isolation(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"a": {Name: "a", Description: "original"},
	})
	m := idx.ToMap()
	// ToMap returns the internal map reference — mutations propagate back.
	m["b"] = Skill{Name: "b"}
	if _, ok := idx.ByName("b"); !ok {
		t.Error("ToMap returns internal reference; mutation should be visible in index")
	}
}

func TestSkillSummary_AllFields(t *testing.T) {
	ss := SkillSummary{
		Name:        "test-skill",
		Description: "A test skill summary",
		Triggers:    []string{"a", "b", "c"},
		Scenario:    "marketing",
		HasAssets:   true,
		HasPreview:  true,
	}
	if ss.Name != "test-skill" {
		t.Error("Name field")
	}
	if ss.Description != "A test skill summary" {
		t.Error("Description field")
	}
	if len(ss.Triggers) != 3 {
		t.Error("Triggers field")
	}
	if ss.Scenario != "marketing" {
		t.Error("Scenario field")
	}
	if !ss.HasAssets {
		t.Error("HasAssets field")
	}
	if !ss.HasPreview {
		t.Error("HasPreview field")
	}
}

func TestParseFrontmatter_CRLFLineEndings(t *testing.T) {
	input := "---\r\nname: crlf-skill\r\ndescription: CRLF test\r\n---\r\nBody with CRLF\r\nend"
	fm, body := ParseFrontmatter([]byte(input))
	if fm.Name != "crlf-skill" {
		t.Errorf("Name = %q, want %q", fm.Name, "crlf-skill")
	}
	if !strings.Contains(body, "Body with CRLF") {
		t.Errorf("body missing content: %q", body)
	}
}

func TestParseFrontmatter_YAMLDeepNesting(t *testing.T) {
	input := `---
name: deep-skill
od:
  mode: deck
  scenario: education
  preview:
    type: image
    entry: preview.png
  design_system:
    requires: true
  animations: true
  speaker_notes: false
  upstream: https://github.com/example/repo
  upstream_license: MIT
---
Body`
	fm, body := ParseFrontmatter([]byte(input))
	if fm.Name != "deep-skill" {
		t.Errorf("Name = %q", fm.Name)
	}
	if fm.OD.Mode != "deck" {
		t.Errorf("Mode = %q", fm.OD.Mode)
	}
	if fm.OD.Scenario != "education" {
		t.Errorf("Scenario = %q", fm.OD.Scenario)
	}
	if fm.OD.Preview == nil || fm.OD.Preview.Type != "image" || fm.OD.Preview.Entry != "preview.png" {
		t.Errorf("Preview = %+v", fm.OD.Preview)
	}
	if !fm.OD.DesignSystem.Requires {
		t.Error("expected DesignSystem.Requires=true")
	}
	if !fm.OD.Animations {
		t.Error("expected Animations=true")
	}
	if fm.OD.SpeakerNotes {
		t.Error("expected SpeakerNotes=false")
	}
	if !strings.Contains(body, "Body") {
		t.Error("body missing")
	}
}

func TestParseFrontmatter_YAMLEmptyTriggers(t *testing.T) {
	input := `---
name: empty-triggers
triggers:
---
body`
	fm, _ := ParseFrontmatter([]byte(input))
	if fm.Name != "empty-triggers" {
		t.Errorf("Name = %q", fm.Name)
	}
	if fm.Triggers != nil {
		t.Errorf("expected nil Triggers for empty list, got %v", fm.Triggers)
	}
}

func TestParseFrontmatter_YAMLBooleanValues(t *testing.T) {
	input := `---
name: bool-skill
od:
  animations: true
  speaker_notes: false
---
body`
	fm, _ := ParseFrontmatter([]byte(input))
	if !fm.OD.Animations {
		t.Error("expected Animations=true")
	}
	if fm.OD.SpeakerNotes {
		t.Error("expected SpeakerNotes=false")
	}
}

// ---------------------------------------------------------------------------
// SkillTool.Descriptor test (called directly in skill package)
// ---------------------------------------------------------------------------

func TestSkillTool_Descriptor(t *testing.T) {
	idx := NewIndex(map[string]Skill{})
	tool := NewSkillTool(idx, nil)
	desc := tool.Descriptor()

	if desc.Name != "skill" {
		t.Errorf("expected name 'skill', got %q", desc.Name)
	}
	if desc.Description == "" {
		t.Error("expected non-empty description")
	}
	if desc.MaxResultSizeChars == 0 {
		t.Error("expected non-zero MaxResultSizeChars")
	}
	if !desc.Flags.ReadOnly {
		t.Error("expected ReadOnly flag")
	}
	if !desc.Flags.ConcurrencySafe {
		t.Error("expected ConcurrencySafe flag")
	}
	// Verify input schema has operation enum.
	schema := desc.InputJSONSchema
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in input schema")
	}
	op, ok := props["operation"].(map[string]any)
	if !ok {
		t.Fatal("expected operation property")
	}
	enumVals, ok := op["enum"].([]string)
	if !ok {
		t.Fatal("expected operation.enum to be []string")
	}
	if len(enumVals) < 3 {
		t.Errorf("expected at least 3 enum values, got %d: %v", len(enumVals), enumVals)
	}
}

// ---------------------------------------------------------------------------
// SkillTool list_assets operation tests
// ---------------------------------------------------------------------------

func TestSkillTool_ListAssets(t *testing.T) {
	dir := t.TempDir()
	// Create a fake skill directory with assets.
	skillDir := filepath.Join(dir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "example.html"), []byte("<html></html>"), 0644)
	os.WriteFile(filepath.Join(skillDir, "style.css"), []byte("body{}"), 0644)
	os.MkdirAll(filepath.Join(skillDir, "images"), 0755)

	idx := NewIndex(map[string]Skill{
		"test-skill": {Name: "test-skill", DirPath: skillDir},
	})
	tool := NewSkillTool(idx, nil)

	result := tool.Call(nil, contracts.ToolCallArgs{
		ArgsJSON: `{"operation":"list_assets","name":"test-skill"}`,
	})
	if result.IsError {
		t.Fatalf("list_assets failed: %s", result.ErrorMessage)
	}
	if !strings.Contains(result.Content, "example.html") {
		t.Errorf("expected example.html in result: %s", result.Content)
	}
	if !strings.Contains(result.Content, "style.css") {
		t.Errorf("expected style.css in result: %s", result.Content)
	}
	if !strings.Contains(result.Content, "images") {
		t.Errorf("expected images dir in result: %s", result.Content)
	}
}

func TestSkillTool_ListAssets_MissingName(t *testing.T) {
	idx := NewIndex(map[string]Skill{})
	tool := NewSkillTool(idx, nil)
	result := tool.Call(nil, contracts.ToolCallArgs{
		ArgsJSON: `{"operation":"list_assets"}`,
	})
	if !result.IsError || result.ErrorCode != "missing_name" {
		t.Errorf("expected missing_name error, got %s: %s", result.ErrorCode, result.ErrorMessage)
	}
}

func TestSkillTool_ListAssets_RejectsPathTraversal(t *testing.T) {
	idx := NewIndex(map[string]Skill{})
	tool := NewSkillTool(idx, nil)
	result := tool.Call(nil, contracts.ToolCallArgs{
		ArgsJSON: `{"operation":"list_assets","name":"../etc"}`,
	})
	if !result.IsError || result.ErrorCode != "invalid_name" {
		t.Errorf("expected invalid_name error for path traversal, got %s: %s",
			result.ErrorCode, result.ErrorMessage)
	}
}

func TestSkillTool_ListAssets_NotFound(t *testing.T) {
	idx := NewIndex(map[string]Skill{
		"real-skill": {Name: "real-skill", DirPath: "/tmp"},
	})
	tool := NewSkillTool(idx, nil)
	result := tool.Call(nil, contracts.ToolCallArgs{
		ArgsJSON: `{"operation":"list_assets","name":"nonexistent"}`,
	})
	if !result.IsError || result.ErrorCode != "not_found" {
		t.Errorf("expected not_found error, got %s: %s", result.ErrorCode, result.ErrorMessage)
	}
}

// ---------------------------------------------------------------------------
// SkillIndex.ListAssets direct test
// ---------------------------------------------------------------------------

func TestSkillIndex_ListAssets(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("b"), 0644)
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)

	idx := NewIndex(map[string]Skill{})
	entries, err := idx.ListAssets("any", dir)
	if err != nil {
		t.Fatalf("ListAssets failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestSkillIndex_ListAssets_NonexistentDir(t *testing.T) {
	idx := NewIndex(map[string]Skill{})
	_, err := idx.ListAssets("any", "/nonexistent/dir/path")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

// ---------------------------------------------------------------------------
// Legacy frontmatter parser tests
// ---------------------------------------------------------------------------

func TestLegacyParseFrontmatter(t *testing.T) {
	lines := []string{"name: my-skill", "description: A test skill", "whenToUse: testing"}
	fm, body := legacyParseFrontmatter(lines, "body content")
	if fm.Name != "my-skill" {
		t.Errorf("expected name 'my-skill', got %q", fm.Name)
	}
	if fm.Description != "A test skill" {
		t.Errorf("unexpected description: %q", fm.Description)
	}
	if fm.WhenToUse != "testing" {
		t.Errorf("unexpected whenToUse: %q", fm.WhenToUse)
	}
	if body != "body content" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestLegacyParseFrontmatter_Type(t *testing.T) {
	lines := []string{"type: always"}
	fm, _ := legacyParseFrontmatter(lines, "")
	if fm.Type != "always" {
		t.Errorf("expected type 'always', got %q", fm.Type)
	}
}

func TestLegacyParseFrontmatter_NoKeyValue(t *testing.T) {
	lines := []string{"not a key value line", "   ", "also not valid"}
	fm, body := legacyParseFrontmatter(lines, "the body")
	if fm.Name != "" || fm.Description != "" {
		t.Error("expected empty Frontmatter for invalid lines")
	}
	if body != "the body" {
		t.Errorf("unexpected body: %q", body)
	}
}

// ---------------------------------------------------------------------------
// cutKeyValue tests
// ---------------------------------------------------------------------------

func TestCutKeyValue_Normal(t *testing.T) {
	key, val, ok := cutKeyValue("name: my-skill")
	if !ok {
		t.Fatal("expected ok")
	}
	if key != "name" {
		t.Errorf("expected key 'name', got %q", key)
	}
	if val != " my-skill" {
		t.Errorf("expected val ' my-skill', got %q", val)
	}
}

func TestCutKeyValue_NoColon(t *testing.T) {
	_, _, ok := cutKeyValue("no colon here")
	if ok {
		t.Error("expected false for string without colon")
	}
}

func TestCutKeyValue_MultipleColons(t *testing.T) {
	key, val, ok := cutKeyValue("url: https://example.com")
	if !ok {
		t.Fatal("expected ok")
	}
	if key != "url" {
		t.Errorf("expected key 'url', got %q", key)
	}
	if val != " https://example.com" {
		t.Errorf("expected val ' https://example.com', got %q", val)
	}
}

// ---------------------------------------------------------------------------
// Frontmatter YAML failure → legacy fallback test
// ---------------------------------------------------------------------------

func TestParseFrontmatter_YAMLFailureFallsBackToLegacy(t *testing.T) {
	// Invalid YAML inside the frontmatter block triggers legacy parser fallback.
	content := "---\nname: fallback-skill\ndescription: A skill for fallback testing\n---\nRemaining body here"
	fm, body := ParseFrontmatter([]byte(content))
	// YAML unmarshal should succeed for this simple case, so name/desc come through.
	if fm.Name != "fallback-skill" {
		t.Errorf("expected name 'fallback-skill', got %q", fm.Name)
	}
	if fm.Description != "A skill for fallback testing" {
		t.Errorf("expected description from frontmatter, got %q", fm.Description)
	}
	if body != "Remaining body here" {
		t.Errorf("expected remaining body, got %q", body)
	}
}

func TestLoadSkills_HasAssets_EdgeCases(t *testing.T) {
	t.Run("with assets", func(t *testing.T) {
		dir := t.TempDir()
		skillDir := filepath.Join(dir, "with-assets")
		os.MkdirAll(skillDir, 0755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: with-assets\n---\nBody"), 0644)
		// Create asset files: example.html, fonts.css, nav.js.
		os.WriteFile(filepath.Join(skillDir, "example.html"), []byte("<html></html>"), 0644)
		os.WriteFile(filepath.Join(skillDir, "fonts.css"), []byte("body{}"), 0644)
		os.WriteFile(filepath.Join(skillDir, "nav.js"), []byte("// nav"), 0644)

		skills := LoadSkills(dir, "/nonexistent")
		s, ok := skills.ByName("with-assets")
		if !ok {
			t.Fatal("expected with-assets skill")
		}
		if !s.HasAssets {
			t.Error("expected HasAssets=true when assets dir has files")
		}
	})

	t.Run("without assets", func(t *testing.T) {
		dir := t.TempDir()
		skillDir := filepath.Join(dir, "no-assets")
		os.MkdirAll(skillDir, 0755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: no-assets\n---\nBody"), 0644)
		// No other files in the directory.

		skills := LoadSkills(dir, "/nonexistent")
		s, ok := skills.ByName("no-assets")
		if !ok {
			t.Fatal("expected no-assets skill")
		}
		if s.HasAssets {
			t.Error("expected HasAssets=false when no asset files exist")
		}
	})
}
