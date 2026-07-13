package hook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsPathSafe(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	mustMkdir(t, workspace)

	// Create a regular file inside workspace.
	insideFile := filepath.Join(workspace, "index.html")
	mustWrite(t, insideFile, "<html></html>")

	// Create a file outside workspace.
	outsideFile := filepath.Join(tmp, "outside.txt")
	mustWrite(t, outsideFile, "outside")

	tests := []struct {
		name     string
		root     string
		absPath  string
		expected bool
	}{
		{
			name:     "path within workspace",
			root:     workspace,
			absPath:  insideFile,
			expected: true,
		},
		{
			name:     "path exactly equal to workspace root",
			root:     workspace,
			absPath:  workspace,
			expected: true,
		},
		{
			name:     "path outside workspace",
			root:     workspace,
			absPath:  outsideFile,
			expected: false,
		},
		{
			name:     "empty workspaceRoot",
			root:     "",
			absPath:  insideFile,
			expected: false,
		},
		{
			name:     "path with dot-dot escape",
			root:     workspace,
			absPath:  filepath.Join(workspace, "..", "outside.txt"),
			expected: false,
		},
		{
			name:     "rel returns dot (workspace itself)",
			root:     workspace,
			absPath:  workspace + string(filepath.Separator) + ".",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPathSafe(tt.root, tt.absPath)
			if result != tt.expected {
				t.Errorf("IsPathSafe(%q, %q) = %v, want %v",
					tt.root, tt.absPath, result, tt.expected)
			}
		})
	}
}

func TestIsPathSafe_Symlink(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	mustMkdir(t, workspace)

	// Create a symlink inside workspace pointing outside.
	outsideFile := filepath.Join(tmp, "outside.txt")
	mustWrite(t, outsideFile, "outside")

	symlinkPath := filepath.Join(workspace, "escape_link")
	mustSymlink(t, outsideFile, symlinkPath)

	if IsPathSafe(workspace, symlinkPath) {
		t.Error("IsPathSafe should reject symlink paths")
	}
}

func TestIsPathSafe_SymlinkInIntermediate(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	mustMkdir(t, workspace)

	// Create dir with symlink component in the middle.
	realDir := filepath.Join(tmp, "real_subdir")
	mustMkdir(t, realDir)
	symlinkDir := filepath.Join(workspace, "link_subdir")
	mustSymlink(t, realDir, symlinkDir)

	deepFile := filepath.Join(symlinkDir, "nested.html")
	mustWrite(t, deepFile, "<html></html>")

	if IsPathSafe(workspace, deepFile) {
		t.Error("IsPathSafe should reject paths with symlink intermediate segments")
	}
}

func TestReadFileWithCap(t *testing.T) {
	tmp := t.TempDir()
	workspace := filepath.Join(tmp, "workspace")
	mustMkdir(t, workspace)

	smallFile := filepath.Join(workspace, "small.html")
	mustWrite(t, smallFile, "hello")

	largeFile := filepath.Join(workspace, "large.html")
	largeContent := make([]byte, 2000)
	for i := range largeContent {
		largeContent[i] = 'x'
	}
	mustWrite(t, largeFile, string(largeContent))

	outsideFile := filepath.Join(tmp, "outside.txt")
	mustWrite(t, outsideFile, "outside")

	tests := []struct {
		name          string
		absPath       string
		maxBytes      int64
		workspaceRoot string
		wantEmpty     bool
		wantContent   string
	}{
		{
			name:          "normal file within cap",
			absPath:       smallFile,
			maxBytes:      1024,
			workspaceRoot: workspace,
			wantContent:   "hello",
		},
		{
			name:          "file exceeds cap",
			absPath:       largeFile,
			maxBytes:      1000,
			workspaceRoot: workspace,
			wantEmpty:     true,
		},
		{
			name:          "empty workspaceRoot",
			absPath:       smallFile,
			maxBytes:      1024,
			workspaceRoot: "",
			wantEmpty:     true,
		},
		{
			name:          "path outside workspace",
			absPath:       outsideFile,
			maxBytes:      1024,
			workspaceRoot: workspace,
			wantEmpty:     true,
		},
		{
			name:          "non-existent file",
			absPath:       filepath.Join(workspace, "nope.html"),
			maxBytes:      1024,
			workspaceRoot: workspace,
			wantEmpty:     true,
		},
		{
			name:          "file exactly at cap",
			absPath:       smallFile,
			maxBytes:      5,
			workspaceRoot: workspace,
			wantContent:   "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReadFileWithCap(tt.absPath, tt.maxBytes, tt.workspaceRoot)
			if tt.wantEmpty {
				if result != "" {
					t.Errorf("ReadFileWithCap() = %q, want empty", result)
				}
			} else {
				if result != tt.wantContent {
					t.Errorf("ReadFileWithCap() = %q, want %q", result, tt.wantContent)
				}
			}
		})
	}
}

func TestHasNoSymlinks(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	mustMkdir(t, root)
	subdir := filepath.Join(root, "subdir")
	mustMkdir(t, subdir)

	// clean == root
	if !hasNoSymlinks(root, root) {
		t.Error("hasNoSymlinks(root, root) should be true")
	}

	// normal path, no symlinks
	normalPath := filepath.Join(root, "subdir", "file.txt")
	mustWrite(t, normalPath, "content")
	if !hasNoSymlinks(root, normalPath) {
		t.Error("hasNoSymlinks should be true for normal path without symlinks")
	}

	// path with symlink segment
	realDir := filepath.Join(tmp, "real")
	mustMkdir(t, realDir)
	symlinkSeg := filepath.Join(root, "linkseg")
	mustSymlink(t, realDir, symlinkSeg)
	symlinkPath := filepath.Join(symlinkSeg, "nested.txt")
	mustWrite(t, symlinkPath, "nested")

	if hasNoSymlinks(root, symlinkPath) {
		t.Error("hasNoSymlinks should be false when path contains symlink segment")
	}
}

// helpers

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink %s -> %s: %v", link, target, err)
	}
}