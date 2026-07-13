package skill

import (
	"fmt"
	"sort"
	"strings"
)

// FormatSkills formats the given skills for system prompt injection.
// budgetTokens caps the estimated token count (~4 chars per token).
// The richest format that fits within the budget is selected.
func FormatSkills(skills map[string]Skill, budgetTokens int) string {
	if len(skills) == 0 {
		return ""
	}
	sorted := sortedSkills(skills)
	full := formatFull(sorted)
	if estimateTokens(full) <= budgetTokens {
		return full
	}
	medium := formatMedium(sorted)
	if estimateTokens(medium) <= budgetTokens {
		return medium
	}
	return formatMinimal(sorted)
}

// FormatSkillsForMode formats skills grouped by mode, giving deck-mode skills more token budget.
// Deck skills get name + triggers + one-line description; non-deck skills are folded.
func FormatSkillsForMode(idx *SkillIndex, targetMode string, budgetTokens int) string {
	if idx.Len() == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Available Skills\n\n")

	deckSkills := idx.ByMode(targetMode)
	if len(deckSkills) > 0 {
		b.WriteString(fmt.Sprintf("### Design skills (%s mode)\n", targetMode))
		for _, s := range deckSkills {
			triggers := ""
			if len(s.Triggers) > 0 {
				triggers = " [" + strings.Join(s.Triggers[:min(5, len(s.Triggers))], ", ") + "]"
			}
			fmt.Fprintf(&b, "- **%s**%s", s.Name, triggers)
			if s.Description != "" {
				desc := firstLine(s.Description)
				fmt.Fprintf(&b, " — %s", desc)
			}
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	// Fold non-target-mode skills into a compact list.
	otherNames := make([]string, 0)
	for _, name := range idx.AllNames() {
		sk, _ := idx.ByName(name)
		if sk.Mode != targetMode {
			otherNames = append(otherNames, name)
		}
	}
	if len(otherNames) > 0 {
		fmt.Fprintf(&b, "### Other skills (%s mode)\n", "non-"+targetMode)
		fmt.Fprintf(&b, "%s\n", strings.Join(otherNames, ", "))
	}

	result := b.String()
	if estimateTokens(result) > budgetTokens {
		return FormatSkills(idx.ToMap(), budgetTokens)
	}
	return result
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

func sortedSkills(skills map[string]Skill) []Skill {
	out := make([]Skill, 0, len(skills))
	for _, s := range skills {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func estimateTokens(s string) int {
	return len(s) / 4
}

func formatFull(skills []Skill) string {
	var b strings.Builder
	b.WriteString("## Available Skills\n\n")
	for _, s := range skills {
		fmt.Fprintf(&b, "### %s\n", s.Name)
		if s.Description != "" {
			fmt.Fprintf(&b, "**Description:** %s\n", s.Description)
		}
		if s.WhenToUse != "" {
			fmt.Fprintf(&b, "**When to use:** %s\n", s.WhenToUse)
		}
		if s.Body != "" {
			b.WriteString(s.Body)
			if !strings.HasSuffix(s.Body, "\n") {
				b.WriteByte('\n')
			}
		}
		b.WriteString("---\n\n")
	}
	return b.String()
}

func formatMedium(skills []Skill) string {
	var b strings.Builder
	b.WriteString("## Available Skills\n\n")
	for _, s := range skills {
		if s.Type == "always" {
			fmt.Fprintf(&b, "### %s\n", s.Name)
			if s.Description != "" {
				fmt.Fprintf(&b, "**Description:** %s\n", s.Description)
			}
			if s.WhenToUse != "" {
				fmt.Fprintf(&b, "**When to use:** %s\n", s.WhenToUse)
			}
			if s.Body != "" {
				b.WriteString(s.Body)
				if !strings.HasSuffix(s.Body, "\n") {
					b.WriteByte('\n')
				}
			}
			b.WriteString("---\n\n")
		} else {
			desc := s.Description
			if s.WhenToUse != "" {
				if desc != "" {
					desc += ". "
				}
				desc += "Use when: " + s.WhenToUse
			}
			if desc != "" {
				fmt.Fprintf(&b, "- **%s**: %s\n", s.Name, desc)
			} else {
				fmt.Fprintf(&b, "- **%s**\n", s.Name)
			}
		}
	}
	return b.String()
}

func formatMinimal(skills []Skill) string {
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	return "## Available Skills\n\n" + strings.Join(names, ", ") + "\n"
}
