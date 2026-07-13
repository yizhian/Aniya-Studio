package core

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"agentgo/internal/toolkit/contracts"
)

type ReadFileTool struct{}

func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{}
}

func (t *ReadFileTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:               "read_file",
		Description:        "Read a file from workspace and optionally slice by line range.",
		Prompt:             "Use read_file with a path relative to the workspace; prefer line ranges for large files. Before edit_file, read this file and pass read_mtime_unix_ns from metadata exactly as returned.",
		MaxResultSizeChars: 12000,
		InputJSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":       map[string]any{"type": "string", "description": "File path relative to workspace (e.g. \"index.html\"). Absolute paths are rejected."},
				"start_line": map[string]any{"type": "integer", "minimum": 1},
				"end_line":   map[string]any{"type": "integer", "minimum": 1},
			},
			"required": []any{"path"},
		},
		Flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: true,
			ReadOnly:        true,
		},
	}
}

func (t *ReadFileTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	_ = ctx
	if args.OnProgress != nil {
		args.OnProgress(contracts.ProgressEvent{Stage: "start", Message: "reading file"})
	}
	input, err := parseToolArgs[struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		EndLine   int    `json:"end_line"`
	}](args.ArgsJSON)
	if err != nil {
		return newToolError("invalid_json", err.Error())
	}
	resolved, err := resolveWorkspacePath(args.Context.WorkspacePath, input.Path)
	if err != nil {
		return newToolError("invalid_path", err.Error())
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return newToolError("read_failed", err.Error())
	}
	fi, err := os.Stat(resolved)
	if err != nil {
		return newToolError("stat_failed", err.Error())
	}
	mtimeNs := fi.ModTime().UnixNano()
	content := string(raw)
	lines := strings.Split(content, "\n")
	start := input.StartLine
	end := input.EndLine
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return newToolError("invalid_range", "start_line cannot be greater than end_line")
	}
	var b strings.Builder
	for i := start; i <= end; i++ {
		if i-1 >= len(lines) {
			break
		}
		b.WriteString(fmt.Sprintf("%d|%s\n", i, lines[i-1]))
	}
	if args.OnProgress != nil {
		args.OnProgress(contracts.ProgressEvent{Stage: "done", Message: "file read completed"})
	}
	mtimeNsStr := strconv.FormatInt(mtimeNs, 10)
	contentStr := strings.TrimRight(b.String(), "\n")
	contentStr += fmt.Sprintf("\n\n[file: %s | mtime_ns: %s]", resolved, mtimeNsStr)
	return contracts.ToolResult{
		Content: contentStr,
		Metadata: map[string]any{
			"path":               resolved,
			"line_start":         start,
			"line_end":           end,
			"total_lines":        len(lines),
			"size_bytes":         fi.Size(),
			"read_mtime_unix_ns": mtimeNsStr,
			"edit_requires_read": true,
		},
	}
}
