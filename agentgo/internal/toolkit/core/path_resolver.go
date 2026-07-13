package core

import (
	"fmt"
	"path/filepath"
	"strings"

	"agentgo/internal/hook"
)

func resolveWorkspacePath(workspacePath, targetPath string) (string, error) {
	return hook.ResolveWorkspacePath(workspacePath, targetPath)
}

// protectedNames lists path components that tools must not write to.
// Matching is done against each component of the resolved path.
// Note: .agentgo/memory/ is deliberately NOT protected — the Agent writes
// memory files there via write_file, and the system rebuilds the memory index.
var protectedNames = map[string]bool{
	".slidecraft":  true,
	"skills":       true,
	"active.json":  true,
	"project.json": true,
}

// protectedPrefixes lists path prefixes (relative to workspace root) that
// must not be written to. Used for subdirectory-level protection under .agentgo/.
var protectedPrefixes = []string{
	".agentgo/sessions",
	".agentgo/logs",
}

// validateNotSystemPath checks that the resolved path does not target a system-managed
// file or directory. These are maintained automatically and must not be modified
// by write_file or edit_file.
func validateNotSystemPath(resolved string) error {
	clean := filepath.Clean(resolved)
	parts := strings.Split(clean, string(filepath.Separator))
	for _, part := range parts {
		if protectedNames[part] {
			return fmt.Errorf("cannot modify system path %s — this is managed automatically", part)
		}
	}
	// Also check prefix-based protections (e.g. .agentgo/sessions/).
	sep := string(filepath.Separator)
	for _, prefix := range protectedPrefixes {
		if strings.Contains(clean, sep+prefix+sep) || strings.HasSuffix(clean, sep+prefix) {
			return fmt.Errorf("cannot modify system path %s — this is managed automatically", prefix)
		}
	}
	return nil
}