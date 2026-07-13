package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"agentgo/internal/toolkit/contracts"
)

// WriteFileTool 仅用于创建新文件；已存在则拒绝（引导使用 edit_file）。
type WriteFileTool struct{}

func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{}
}

func (t *WriteFileTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:               "write_file",
		Description:        "Create a new file with the given content. Fails if the path already exists — use edit_file for existing files.",
		Prompt:             "CRITICAL: write_file is ONLY for CREATING new files. It WILL FAIL with file_exists if the path already exists. For ANY existing file — including files you just created — use read_file + edit_file instead. write_file destroys GrapesJS editor state. If you see compliance violations after writing, fix them with edit_file, NOT another write_file.",
		MaxResultSizeChars: 8000,
		InputJSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path relative to workspace (e.g. \"output.html\"). Absolute paths are rejected — use relative paths only.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full file content to write",
				},
			},
			"required": []any{"path", "content"},
		},
		Flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: false,
			ReadOnly:        false,
			Destructive:     true,
		},
	}
}

func (t *WriteFileTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	_ = ctx
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(args.ArgsJSON), &input); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_json", ErrorMessage: err.Error()}
	}
	if strings.TrimSpace(input.Path) == "" {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_path", ErrorMessage: "path is required"}
	}

	resolved, err := resolveWorkspacePath(args.Context.WorkspacePath, input.Path)
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_path", ErrorMessage: err.Error()}
	}

	if err := validateNotSystemPath(resolved); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "system_path", ErrorMessage: err.Error()}
	}

	if fi, err := os.Stat(resolved); err == nil {
		if fi.IsDir() {
			return contracts.ToolResult{IsError: true, ErrorCode: "path_is_directory", ErrorMessage: "path exists as a directory"}
		}
		return contracts.ToolResult{
			IsError:      true,
			ErrorCode:    "file_exists",
			ErrorMessage: "file already exists; use read_file + edit_file instead of write_file",
			Content:      `{"is_error":true,"error_code":"file_exists","hint":"use edit_file with read proof"}`,
		}
	} else if !os.IsNotExist(err) {
		return contracts.ToolResult{IsError: true, ErrorCode: "stat_failed", ErrorMessage: err.Error()}
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "mkdir_failed", ErrorMessage: err.Error()}
	}

	mode := os.FileMode(0o644)
	if err := os.WriteFile(resolved, []byte(input.Content), mode); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "write_failed", ErrorMessage: err.Error()}
	}

	fi, _ := os.Stat(resolved)
	return contracts.ToolResult{
		Content: fmt.Sprintf("write_file: created %q", input.Path),
		Metadata: map[string]any{
			"path":               resolved,
			"read_mtime_unix_ns": strconv.FormatInt(fi.ModTime().UnixNano(), 10),
		},
	}
}
