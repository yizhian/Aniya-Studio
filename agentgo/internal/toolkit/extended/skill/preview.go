package skill

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolvePreviewRel returns the relative path to a preview HTML file within the
// skill directory, or "" if none exists.
func ResolvePreviewRel(dirPath, previewEntry string) string {
	if dirPath == "" {
		return ""
	}

	candidates := make([]string, 0, 8)
	if previewEntry != "" {
		candidates = append(candidates, previewEntry)
	}
	candidates = append(candidates,
		"example.html",
		"index.html",
		"assets/example-slides.html",
		"examples/example-helix.html",
	)
	for _, rel := range candidates {
		if fileExists(filepath.Join(dirPath, rel)) {
			return rel
		}
	}

	examplesDir := filepath.Join(dirPath, "examples")
	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".html") {
			return filepath.Join("examples", e.Name())
		}
	}
	return ""
}

// ResolvePreviewPath returns the absolute path to the preview HTML file.
func ResolvePreviewPath(sk Skill) string {
	rel := ResolvePreviewRel(sk.DirPath, sk.PreviewEntry)
	if rel == "" {
		return ""
	}
	return filepath.Join(sk.DirPath, rel)
}

// HasPreview reports whether the skill has a loadable preview HTML file.
func HasPreview(sk Skill) bool {
	return ResolvePreviewRel(sk.DirPath, sk.PreviewEntry) != ""
}

// ResolveAssetPath returns the absolute path to an asset file for a skill.
// assetPath must stay within the skill directory; callers usually pass paths
// under assets/, but previews may also reference other skill-local files.
func ResolveAssetPath(sk Skill, assetPath string) string {
	if sk.DirPath == "" || assetPath == "" || strings.Contains(assetPath, "..") || filepath.IsAbs(assetPath) {
		return ""
	}

	local := filepath.Join(sk.DirPath, "assets", assetPath)
	if isSafeChild(sk.DirPath, local) && fileExists(local) {
		return local
	}
	local = filepath.Join(sk.DirPath, assetPath)
	if isSafeChild(sk.DirPath, local) && fileExists(local) {
		return local
	}
	return ""
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func isSafeChild(root, candidate string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
