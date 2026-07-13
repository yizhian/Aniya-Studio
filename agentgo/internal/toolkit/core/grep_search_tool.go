package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"agentgo/internal/toolkit/contracts"
)

const (
	defaultGrepMaxMatches = 200
	defaultGrepMaxFiles   = 400
	maxPreviewPerMatch    = 400
)

// GrepSearchTool 在工作区内按正则检索文件内容（只读）。
type GrepSearchTool struct{}

func NewGrepSearchTool() *GrepSearchTool {
	return &GrepSearchTool{}
}

func (t *GrepSearchTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:        "grep_search",
		Description: "Search file contents under a workspace directory using a regular expression (line-based matches). The path must be a directory — file paths will fail with an error.",
		Prompt:      "Use grep_search to search across directories for symbols or strings. Pass a directory path (e.g. \".\") — passing a file path will fail. Prefer grep_search over loading whole files when searching across the repo.",
		MaxResultSizeChars: 12000,
		InputJSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{
					"type":        "string",
					"description": "Go regexp pattern, e.g. \"func main\" or \"TODO\"",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Directory path relative to workspace (e.g. \".\"). Must be a directory — file paths will fail with \"not a directory\" error. Absolute paths like /go, /tmp are rejected.",
				},
				"max_matches": map[string]any{"type": "integer", "minimum": 1},
				"max_files":   map[string]any{"type": "integer", "minimum": 1},
			},
			"required": []any{"pattern"},
		},
		Flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: true,
			ReadOnly:        true,
		},
	}
}

func (t *GrepSearchTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	_ = ctx
	var input struct {
		Pattern    string `json:"pattern"`
		Path       string `json:"path"`
		MaxMatches int    `json:"max_matches"`
		MaxFiles   int    `json:"max_files"`
	}
	if err := json.Unmarshal([]byte(args.ArgsJSON), &input); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_json", ErrorMessage: err.Error()}
	}
	if strings.TrimSpace(input.Pattern) == "" {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_pattern", ErrorMessage: "pattern is required"}
	}
	if strings.TrimSpace(input.Path) == "" {
		input.Path = "."
	}
	if input.MaxMatches <= 0 {
		input.MaxMatches = defaultGrepMaxMatches
	}
	if input.MaxFiles <= 0 {
		input.MaxFiles = defaultGrepMaxFiles
	}

	re, err := regexp.Compile(input.Pattern)
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_regexp", ErrorMessage: err.Error()}
	}

	root, err := resolveWorkspacePath(args.Context.WorkspacePath, input.Path)
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_path", ErrorMessage: err.Error()}
	}
	info, err := os.Stat(root)
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "stat_failed", ErrorMessage: err.Error()}
	}
	if !info.IsDir() {
		return contracts.ToolResult{IsError: true, ErrorCode: "not_directory", ErrorMessage: fmt.Sprintf("%q is not a directory", input.Path)}
	}

	var (
		matches   int
		filesSeen int
		b         strings.Builder
	)
	errStopGrep := errors.New("grep_stop")

	skipDir := func(name string) bool {
		switch name {
		case ".git", "node_modules", "vendor", ".idea", ".vscode":
			return true
		default:
			return false
		}
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if d.IsDir() {
			if path != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if matches >= input.MaxMatches || filesSeen >= input.MaxFiles {
			return errStopGrep
		}
		if !isTextCandidate(path) {
			return nil
		}
		filesSeen++
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !utf8.Valid(data) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if matches >= input.MaxMatches {
				return errStopGrep
			}
			if re.MatchString(line) {
				matches++
				preview := line
				if len(preview) > maxPreviewPerMatch {
					preview = preview[:maxPreviewPerMatch] + "…"
				}
				fmt.Fprintf(&b, "%s:%d:%s\n", rel, i+1, preview)
			}
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errStopGrep) {
		return contracts.ToolResult{IsError: true, ErrorCode: "walk_failed", ErrorMessage: walkErr.Error()}
	}
	out := strings.TrimRight(b.String(), "\n")
	if out == "" {
		out = "(no matches)"
	}
	return contracts.ToolResult{
		Content: out,
		Metadata: map[string]any{
			"pattern":       input.Pattern,
			"root":          root,
			"match_count":   matches,
			"files_scanned": filesSeen,
		},
	}
}

func isTextCandidate(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".gz", ".tar", ".7z", ".bin", ".so", ".dylib", ".dll", ".exe":
		return false
	default:
		return true
	}
}
