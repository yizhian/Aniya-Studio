package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	linkAssetRE   = regexp.MustCompile(`<link\s[^>]*rel=["']stylesheet["'][^>]*href=["']([^"']+)["'][^>]*/?>`)
	linkHrefRE    = regexp.MustCompile(`href=["']([^"']+)["']`)
	scriptAssetRE = regexp.MustCompile(`<script\s[^>]*src=["']([^"']+)["'][^>]*>\s*</script>`)
	scriptSrcRE   = regexp.MustCompile(`src=["']([^"']+)["']`)
)

func inlineDesignSkillAssets(htmlPath, skillsDir string) error {
	data, err := os.ReadFile(htmlPath)
	if err != nil {
		return fmt.Errorf("read html: %w", err)
	}
	html := string(data)
	updated := false

	html = linkAssetRE.ReplaceAllStringFunc(html, func(match string) string {
		m := linkHrefRE.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		href := m[1]
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "//") {
			return match
		}
		assetData, err := os.ReadFile(filepath.Join(skillsDir, href))
		if err != nil {
			return match
		}
		updated = true
		return "<style>\n" + string(assetData) + "\n</style>"
	})

	html = scriptAssetRE.ReplaceAllStringFunc(html, func(match string) string {
		m := scriptSrcRE.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		src := m[1]
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "//") {
			return match
		}
		assetData, err := os.ReadFile(filepath.Join(skillsDir, src))
		if err != nil {
			return match
		}
		updated = true
		return "<script>\n" + string(assetData) + "\n</script>"
	})

	if !updated {
		return nil
	}
	return os.WriteFile(htmlPath, []byte(html), 0644)
}
