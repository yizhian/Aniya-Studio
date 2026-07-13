package skill

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// LoadSkills loads skills from two directories: userDir is loaded first,
// then projectDir. Skills with the same Name in projectDir override userDir.
// Missing or unreadable directories are silently skipped.
//
// Each subdirectory under userDir/projectDir is treated as one skill.
// The skill name comes from frontmatter name: field in SKILL.md, falling back
// to the directory name. Flat .md files at the root level are also loaded
// as single-file skills (backward compatibility).
func LoadSkills(userDir, projectDir string) *SkillIndex {
	result := make(map[string]Skill)

	loadDirInto(dirSkills(userDir, "user"), result)
	loadDirInto(dirSkills(projectDir, "project"), result)

	return NewIndex(result)
}

// dirSkills scans a single directory for skills — both subdirectories and flat .md files.
func dirSkills(dir, source string) map[string]Skill {
	out := make(map[string]Skill)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("skill: read dir %q: %v", dir, err)
		}
		return out
	}
	for _, e := range entries {
		if e.IsDir() {
			skillDir := filepath.Join(dir, e.Name())
			sk, ok := loadSkillDir(skillDir, e.Name(), source)
			if ok {
				if _, exists := out[sk.Name]; exists {
					log.Printf("skill: duplicate name %q from dir %q, keeping first", sk.Name, skillDir)
				}
				out[sk.Name] = sk
			}
		} else if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				log.Printf("skill: read %q: %v", path, err)
				continue
			}
			fm, body := ParseFrontmatter(data)
			name := fm.Name
			if name == "" {
				name = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			}
			if name == "" {
				continue
			}
			if _, exists := out[name]; exists {
				log.Printf("skill: duplicate name %q from file %q, keeping first", name, path)
			}
			out[name] = skillFromFrontmatter(fm, name, body, source, path, "")
		}
	}
	return out
}

// loadSkillDir loads a skill from a subdirectory by reading its SKILL.md file.
// Returns false if no valid SKILL.md is found or parsing fails.
func loadSkillDir(skillDir, dirName, source string) (Skill, bool) {
	var path string
	var hasSKILL bool

	entries, err := os.ReadDir(skillDir)
	if err != nil {
		log.Printf("skill: read skill dir %q: %v", skillDir, err)
		return Skill{}, false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), "SKILL.md") {
			path = filepath.Join(skillDir, e.Name())
			hasSKILL = true
			break
		}
	}
	if !hasSKILL {
		log.Printf("skill: no SKILL.md found in %q, skipping", skillDir)
		return Skill{}, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("skill: read %q: %v", path, err)
		return Skill{}, false
	}
	fm, body := ParseFrontmatter(data)
	name := fm.Name
	if name == "" {
		name = dirName
	}
	if name == "" {
		return Skill{}, false
	}

	// Detect assets: any non-SKILL.md files (example.html, templates/, assets/, etc.)
	hasAssets := false
	if !hasAssets {
		for _, e := range entries {
			if !e.IsDir() && !strings.EqualFold(e.Name(), "SKILL.md") {
				hasAssets = true
				break
			}
			if e.IsDir() {
				hasAssets = true
				break
			}
		}
	}

	return skillFromFrontmatter(fm, name, body, source, path, skillDir, hasAssets), true
}

// skillFromFrontmatter builds a Skill from parsed frontmatter and runtime context.
func skillFromFrontmatter(fm Frontmatter, name, body, source, filePath, dirPath string, hasAssets ...bool) Skill {
	previewEntry := ""
	if fm.OD.Preview != nil {
		previewEntry = fm.OD.Preview.Entry
	}

	sk := Skill{
		Name:         name,
		Description:  fm.Description,
		WhenToUse:    fm.WhenToUse,
		Type:         fm.Type,
		Body:         body,
		Source:       source,
		FilePath:     filePath,
		DirPath:      dirPath,
		Triggers:     fm.Triggers,
		Mode:         fm.OD.Mode,
		Scenario:     fm.OD.Scenario,
		PreviewEntry: previewEntry,
	}
	if len(hasAssets) > 0 {
		sk.HasAssets = hasAssets[0]
	}
	if dirPath != "" {
		sk.HasPreview = HasPreview(sk)
	}
	return sk
}

// loadDirInto merges skills from src into dst, overwriting on name collision.
func loadDirInto(src map[string]Skill, dst map[string]Skill) {
	for name, sk := range src {
		dst[name] = sk
	}
}
