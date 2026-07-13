package skill

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFrontmatter extracts --- delimited YAML metadata from content.
// Returns empty Frontmatter if no valid frontmatter block is found.
func ParseFrontmatter(content []byte) (Frontmatter, string) {
	s := string(content)
	first, rest, ok := cutLine(s)
	if !ok {
		return Frontmatter{}, s
	}
	if strings.TrimSpace(first) != "---" {
		return Frontmatter{}, s
	}

	// Scan for closing ---.
	var yamlLines []string
	remaining := rest
	closed := false
	for {
		line, after, hasMore := cutLine(remaining)
		if strings.TrimSpace(line) == "---" {
			remaining = after
			closed = true
			break
		}
		yamlLines = append(yamlLines, line)
		if !hasMore {
			remaining = ""
			break
		}
		remaining = after
	}
	if !closed {
		return Frontmatter{}, s
	}

	yamlText := strings.Join(yamlLines, "\n")
	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlText), &fm); err != nil {
		// Fall back to legacy four-field parser for backward-compatible behavior.
		return legacyParseFrontmatter(yamlLines, remaining)
	}
	return fm, remaining
}

// legacyParseFrontmatter handles pre-YAML frontmatter with flat key:value lines.
func legacyParseFrontmatter(lines []string, body string) (Frontmatter, string) {
	fm := Frontmatter{}
	for _, ml := range lines {
		key, val, ok := cutKeyValue(ml)
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "name":
			fm.Name = strings.TrimSpace(val)
		case "description":
			fm.Description = strings.TrimSpace(val)
		case "whentouse":
			fm.WhenToUse = strings.TrimSpace(val)
		case "type":
			fm.Type = strings.TrimSpace(val)
		}
	}
	return fm, body
}

func cutLine(s string) (line, rest string, ok bool) {
	idx := strings.IndexByte(s, '\n')
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+1:], true
}

func cutKeyValue(s string) (key, value string, ok bool) {
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}
