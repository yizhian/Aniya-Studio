package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agentgo/internal/toolkit/contracts"
)

type ListFilesTool struct{}

func NewListFilesTool() *ListFilesTool {
	return &ListFilesTool{}
}

func (t *ListFilesTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:               "list_files",
		Description:        "List files in a directory with optional recursive traversal.",
		Prompt:             "Use list_files to discover repository structure before read/search actions.",
		MaxResultSizeChars: 12000,
		InputJSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":        map[string]any{"type": "string", "description": "Directory relative to workspace root (e.g. \".\", \"subdir\"). NEVER use absolute paths like /go, /etc, /usr — they will fail."},
				"recursive":   map[string]any{"type": "boolean"},
				"max_entries": map[string]any{"type": "integer", "minimum": 1},
			},
		},
		Flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: true,
			ReadOnly:        true,
		},
	}
}

func (t *ListFilesTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	_ = ctx
	var input struct {
		Path       string `json:"path"`
		Recursive  bool   `json:"recursive"`
		MaxEntries int    `json:"max_entries"`
	}
	if err := json.Unmarshal([]byte(args.ArgsJSON), &input); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_json", ErrorMessage: err.Error()}
	}
	if strings.TrimSpace(input.Path) == "" {
		input.Path = "."
	}
	if input.MaxEntries <= 0 {
		input.MaxEntries = 200
	}
	basePath, err := resolveWorkspacePath(args.Context.WorkspacePath, input.Path)
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_path", ErrorMessage: err.Error()}
	}
	info, err := os.Stat(basePath)
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "stat_failed", ErrorMessage: err.Error()}
	}
	if !info.IsDir() {
		return contracts.ToolResult{IsError: true, ErrorCode: "not_directory", ErrorMessage: fmt.Sprintf("%s is not a directory", input.Path)}
	}

	entries := make([]string, 0, input.MaxEntries)
	var errStopWalk = errors.New("stop_walk")
	appendEntry := func(kind, rel string) bool {
		entries = append(entries, fmt.Sprintf("%s\t%s", kind, rel))
		return len(entries) < input.MaxEntries
	}
	if input.Recursive {
		walkErr := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(basePath, path)
			if err != nil || rel == "." {
				return nil
			}
			kind := "file"
			if d.IsDir() {
				kind = "dir"
			}
			if !appendEntry(kind, rel) {
				return errStopWalk
			}
			return nil
		})
		if walkErr != nil && !errors.Is(walkErr, errStopWalk) {
			return contracts.ToolResult{IsError: true, ErrorCode: "walk_failed", ErrorMessage: walkErr.Error()}
		}
	} else {
		items, err := os.ReadDir(basePath)
		if err != nil {
			return contracts.ToolResult{IsError: true, ErrorCode: "read_dir_failed", ErrorMessage: err.Error()}
		}
		for _, item := range items {
			kind := "file"
			if item.IsDir() {
				kind = "dir"
			}
			if !appendEntry(kind, item.Name()) {
				break
			}
		}
	}
	sort.Strings(entries)
	return contracts.ToolResult{
		Content: strings.Join(entries, "\n"),
		Metadata: map[string]any{
			"path":         basePath,
			"recursive":    input.Recursive,
			"returned":     len(entries),
			"max_entries":  input.MaxEntries,
			"tool_version": "v1",
		},
	}
}
