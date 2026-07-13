package skill

import (
	"regexp"
	"strings"

	"agentgo/internal/util"
	"gopkg.in/yaml.v3"
)

var validScenarios = map[string]bool{
	"marketing":      true,
	"pitch-deck":     true,
	"tech-sharing":   true,
	"internal":       true,
	"product-launch": true,
}

func NormalizeScenario(scenario string) string {
	if validScenarios[scenario] {
		return scenario
	}
	return "marketing"
}

// MergeFrontmatter normalizes a SKILL.md string: enforces name, mode=deck,
// scenario, preview, and syncs description from the LLM output.
func MergeFrontmatter(rawMD string, skillName string, description string, scenario string) string {
	fm, body := ParseFrontmatter([]byte(rawMD))

	fm.Name = skillName
	if fm.Description == "" {
		fm.Description = description
	}
	if fm.OD.Mode == "" {
		fm.OD.Mode = "deck"
	}
	fm.OD.Scenario = NormalizeScenario(scenario)
	if fm.OD.Preview == nil {
		fm.OD.Preview = &PreviewMeta{}
	}
	fm.OD.Preview.Type = "html"
	fm.OD.Preview.Entry = "example.html"

	yamlBytes, err := yaml.Marshal(&fm)
	if err != nil {
		// Fallback: return original with minimal fix.
		return "---\nname: " + skillName + "\n---\n" + body
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.Write(yamlBytes)
	b.WriteString("---\n")
	b.WriteString(body)
	return b.String()
}

var nonAlphaNumDash = regexp.MustCompile(`[^a-z0-9-]`)
var multiDash = regexp.MustCompile(`-+`)

// SanitizeSkillName normalizes a raw name into a safe kebab-case directory name.
func SanitizeSkillName(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ToLower(s)
	s = nonAlphaNumDash.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if s == "" {
		s = "custom-" + util.RandomHex(6)
	}

	return s
}
