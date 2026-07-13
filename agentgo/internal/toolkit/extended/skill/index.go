package skill

import (
	"os"
	"sort"
	"sync"
)

// SkillIndex provides indexed lookup and search over loaded skills.
type SkillIndex struct {
	mu        sync.RWMutex
	byName    map[string]Skill
	byMode    map[string][]string
	deckNames []string
	allNames  []string
}

// NewIndex builds a SkillIndex from the given skill map.
func NewIndex(skills map[string]Skill) *SkillIndex {
	idx := &SkillIndex{
		byName: skills,
		byMode: make(map[string][]string),
	}
	for name, sk := range skills {
		idx.allNames = append(idx.allNames, name)
		mode := sk.Mode
		if mode == "" {
			mode = "unknown"
		}
		idx.byMode[mode] = append(idx.byMode[mode], name)
		if mode == "deck" {
			idx.deckNames = append(idx.deckNames, name)
		}
	}
	sort.Strings(idx.allNames)
	for mode := range idx.byMode {
		sort.Strings(idx.byMode[mode])
	}
	sort.Strings(idx.deckNames)
	return idx
}

// ByName returns the skill with the given name, or false if not found.
func (idx *SkillIndex) ByName(name string) (Skill, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	sk, ok := idx.byName[name]
	return sk, ok
}

// ByMode returns all skills with the given mode.
func (idx *SkillIndex) ByMode(mode string) []Skill {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	names := idx.byMode[mode]
	out := make([]Skill, 0, len(names))
	for _, n := range names {
		out = append(out, idx.byName[n])
	}
	return out
}

// DeckSkills returns all skills with mode=="deck".
func (idx *SkillIndex) DeckSkills() []Skill {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	names := idx.byMode["deck"]
	out := make([]Skill, 0, len(names))
	for _, n := range names {
		out = append(out, idx.byName[n])
	}
	return out
}

// AllNames returns sorted skill names.
func (idx *SkillIndex) AllNames() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.allNames
}

// Len returns the total number of loaded skills.
func (idx *SkillIndex) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.byName)
}

// ToMap returns the underlying map (for callers that still need it).
func (idx *SkillIndex) ToMap() map[string]Skill {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.byName
}

// Reload re-scans the skill directories and atomically swaps the index contents.
// The pointer stays the same so existing references (SkillTool, Ranker) see the
// updated data automatically.
func (idx *SkillIndex) Reload(userDir, projectDir string) {
	newIdx := LoadSkills(userDir, projectDir)
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.byName = newIdx.byName
	idx.byMode = newIdx.byMode
	idx.deckNames = newIdx.deckNames
	idx.allNames = newIdx.allNames
}

// DirEntry describes a single entry in a skill's asset directory.
type DirEntry struct {
	Name  string
	IsDir bool
}

// ListAssets returns the directory listing of a skill's asset directory.
// The name parameter is used only for path safety validation (already done by caller).
func (idx *SkillIndex) ListAssets(_, dirPath string) ([]DirEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	out := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, DirEntry{Name: e.Name(), IsDir: e.IsDir()})
	}
	return out, nil
}
