package hook

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ResolveWorkspacePath resolves a (possibly relative) tool path to an absolute
// path within workspacePath. Returns an error if the path is empty, the
// workspace cannot be resolved, or the resolved path escapes the workspace.
//
// This is the single shared implementation used by both the toolkit/core tools
// (write_file, read_file, edit_file, etc.) and the hook/builtin compliance
// hooks (ComplianceReviewTrigger, DesignSkillRequired).
func ResolveWorkspacePath(workspacePath, targetPath string) (string, error) {
	if strings.TrimSpace(targetPath) == "" {
		return "", fmt.Errorf("path is required")
	}
	cleanWorkspace := strings.TrimSpace(workspacePath)
	if cleanWorkspace == "" {
		return filepath.Abs(targetPath)
	}
	workspaceAbs, err := filepath.Abs(cleanWorkspace)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	var candidate string
	if filepath.IsAbs(targetPath) {
		candidate = targetPath
	} else {
		candidate = filepath.Join(workspaceAbs, targetPath)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve target path: %w", err)
	}
	workspaceWithSep := workspaceAbs + string(filepath.Separator)
	if candidateAbs != workspaceAbs && !strings.HasPrefix(candidateAbs, workspaceWithSep) {
		return "", fmt.Errorf("path %q is outside the workspace (root: %s)", targetPath, workspaceAbs)
	}
	return candidateAbs, nil
}

// IsPathSafe verifies that absPath is within workspaceRoot.
// Uses filepath.Rel for cross-platform path containment.
//
// Symlink policy: ALL symlinks on the path are rejected.
// os.Open follows symlinks by default — filepath.Rel alone is not
// sufficient to prevent escape via symlink.
//
// KNOWN LIMITATION: workspace paths must not contain symlink segments.
// Internal symlinks like dist → ./build/out will cause ReadFileWithCap to
// return "" (skipping htmlchecker validation for the affected file). This is acceptable for the
// SlideCraft use case.
func IsPathSafe(workspaceRoot, absPath string) bool {
	if workspaceRoot == "" {
		return false
	}

	clean := filepath.Clean(absPath)
	root := filepath.Clean(workspaceRoot)
	rel, err := filepath.Rel(root, clean)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}

	return hasNoSymlinks(root, clean)
}

// hasNoSymlinks walks each path segment between root and clean and verifies
// none are symlinks. Both arguments must be filepath.Clean'd before calling.
// Returns true if no symlinks are found on the path.
func hasNoSymlinks(root, clean string) bool {
	if clean == root {
		return true
	}
	// Walk each intermediate segment from root towards clean.
	// filepath.Join builds the path segment by segment, which is
	// cross-platform (handles \ vs / correctly).
	current := root
	rel, err := filepath.Rel(root, clean)
	if err != nil {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		fi, err := os.Lstat(current)
		if err != nil {
			return false
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}
	return true
}

// ReadFileWithCap reads at most maxBytes from the file at absPath.
// Uses io.LimitReader + io.ReadAll — avoids the io.ErrUnexpectedEOF trap
// that io.CopyN produces when the file is shorter than the limit.
// Returns empty string on any error, if the file exceeds maxBytes,
// if workspaceRoot is empty, or if the path fails IsPathSafe.
func ReadFileWithCap(absPath string, maxBytes int64, workspaceRoot string) string {
	if workspaceRoot == "" {
		return ""
	}
	if !IsPathSafe(workspaceRoot, absPath) {
		return ""
	}
	f, err := os.Open(absPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return ""
	}
	// +1 overflow detection: if we read more than maxBytes, the file is too large.
	if int64(len(data)) > maxBytes {
		return ""
	}
	return string(data)
}