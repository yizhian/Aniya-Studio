package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"agentgo/internal/toolkit/contracts"
)

// EditFileTool 基于 search-and-replace 的编辑：强制读证明（mtime）、唯一匹配、引号容错、配置 JSON 校验。
type EditFileTool struct{}

func NewEditFileTool() *EditFileTool {
	return &EditFileTool{}
}

func (t *EditFileTool) Descriptor() contracts.ToolDescriptor {
	return contracts.ToolDescriptor{
		Name:        "edit_file",
		Description: "Replace exactly one occurrence of old_string with new_string. Requires read_mtime_unix_ns from the most recent read_file on the same file — mtime is a freshness token that expires after ANY write_file or edit_file to the same file. Fails if old_string appears 0 or >1 times (after quote-tolerant match).",
		Prompt:      "Use edit_file for ALL modifications to existing files. You MUST call read_file on the SAME file immediately before each edit_file to get a fresh read_mtime_unix_ns. mtime expires after ANY write_file or edit_file to that file — re-read on mtime_mismatch. NEVER use write_file as a shortcut to avoid edit_file — write_file fails on existing files. Prefer unique multi-line old_string.",
		MaxResultSizeChars: 12000,
		InputJSONSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "File path relative to workspace (e.g. \"index.html\"). Absolute paths are rejected.",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "Exact snippet to replace (must occur exactly once; quote-tolerant matching)",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "Replacement text",
				},
				"read_mtime_unix_ns": map[string]any{
					"type":        "string",
					"description": "Exact read_mtime_unix_ns string from the most recent read_file on this file. Becomes stale after any write_file or edit_file to the same file — re-read to get a fresh value. (Stored as string to avoid JSON int precision loss.)",
				},
			},
			"required": []any{"path", "old_string", "new_string", "read_mtime_unix_ns"},
		},
		Flags: contracts.ToolBehaviorFlags{
			ConcurrencySafe: false,
			ReadOnly:        false,
			Destructive:     true,
		},
	}
}

func (t *EditFileTool) Call(ctx context.Context, args contracts.ToolCallArgs) contracts.ToolResult {
	_ = ctx
	var input struct {
		Path            string `json:"path"`
		OldString       string `json:"old_string"`
		NewString       string `json:"new_string"`
		ReadMtimeUnixNs int64  `json:"-"`
	}
	raw := []byte(args.ArgsJSON)
	if err := json.Unmarshal(raw, &input); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_json", ErrorMessage: err.Error()}
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_json", ErrorMessage: err.Error()}
	}
	if _, ok := m["read_mtime_unix_ns"]; !ok {
		return contracts.ToolResult{
			IsError:      true,
			ErrorCode:    "read_proof_missing",
			ErrorMessage: "read_mtime_unix_ns is required: call read_file on this file first and pass the exact value from tool metadata",
		}
	}
	mtimeParsed, err := parseMtimeProof(m["read_mtime_unix_ns"])
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_read_mtime", ErrorMessage: err.Error()}
	}
	input.ReadMtimeUnixNs = mtimeParsed
	if strings.TrimSpace(input.Path) == "" {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_path", ErrorMessage: "path is required"}
	}
	if input.OldString == "" {
		return contracts.ToolResult{IsError: true, ErrorCode: "old_string_empty", ErrorMessage: "old_string must be non-empty"}
	}

	resolved, err := resolveWorkspacePath(args.Context.WorkspacePath, input.Path)
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "invalid_path", ErrorMessage: err.Error()}
	}

	if err := validateNotSystemPath(resolved); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "system_path", ErrorMessage: err.Error()}
	}

	fi, err := os.Stat(resolved)
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "stat_failed", ErrorMessage: err.Error()}
	}
	if fi.IsDir() {
		return contracts.ToolResult{IsError: true, ErrorCode: "is_directory", ErrorMessage: "path is a directory, not a file"}
	}

	// Enforce read-before-edit: the mtime passed in must match the file on disk.
	// The LLM obtains this value from the [file: ... | mtime_ns: ...] trailer
	// appended to read_file output.
	currentMtime := fi.ModTime().UnixNano()
	if currentMtime != input.ReadMtimeUnixNs {
		return contracts.ToolResult{
			IsError:      true,
			ErrorCode:    "mtime_mismatch",
			ErrorMessage: fmt.Sprintf("file modified since read (read mtime: %d, current: %d). Call read_file again to get the latest content and mtime.", input.ReadMtimeUnixNs, currentMtime),
		}
	}

	contentBytes, err := os.ReadFile(resolved)
	if err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "read_failed", ErrorMessage: err.Error()}
	}
	content := string(contentBytes)

	startRune, endRune, nMatch, errReason := findUniqueQuoteTolerantSpan(content, input.OldString)
	if errReason != "" {
		return contracts.ToolResult{
			IsError:      true,
			ErrorCode:    errReason,
			ErrorMessage: humanMessageForEditFail(errReason, nMatch),
			Content:      fmt.Sprintf(`{"is_error":true,"error_code":%q,"match_count":%d}`, errReason, nMatch),
		}
	}

	prefix := string([]rune(content)[:startRune])
	suffix := string([]rune(content)[endRune:])
	newContent := prefix + input.NewString + suffix

	if isClaudeSettingsJSONPath(resolved) {
		if err := validateJSONDocument([]byte(newContent)); err != nil {
			return contracts.ToolResult{
				IsError:      true,
				ErrorCode:    "settings_json_invalid",
				ErrorMessage: fmt.Sprintf("edit would produce invalid JSON for .claude/settings.json: %v", err),
			}
		}
	}

	if err := os.WriteFile(resolved, []byte(newContent), fi.Mode()); err != nil {
		return contracts.ToolResult{IsError: true, ErrorCode: "write_failed", ErrorMessage: err.Error()}
	}

	fi2, _ := os.Stat(resolved)
	return contracts.ToolResult{
		Content: fmt.Sprintf("edit_file: updated %q (one replacement)", input.Path),
		Metadata: map[string]any{
			"path":                   resolved,
			"new_mtime_unix_ns":      strconv.FormatInt(fi2.ModTime().UnixNano(), 10),
			"previous_mtime_unix_ns": strconv.FormatInt(fi.ModTime().UnixNano(), 10),
		},
	}
}

func humanMessageForEditFail(code string, n int) string {
	switch code {
	case "old_string_not_found":
		return "old_string not found in file (0 matches after quote-tolerant search); possible model hallucination — re-read file"
	case "old_string_ambiguous":
		return fmt.Sprintf("old_string matches %d times (>1); provide a longer unique old_string — refusing to guess", n)
	case "old_string_empty":
		return "old_string must not be empty"
	default:
		return code
	}
}

func parseMtimeProof(v any) (int64, error) {
	if v == nil {
		return 0, fmt.Errorf("read_mtime_unix_ns is missing")
	}
	switch x := v.(type) {
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0, fmt.Errorf("read_mtime_unix_ns is empty")
		}
		return strconv.ParseInt(s, 10, 64)
	case float64:
		return int64(x), nil
	case int64:
		return x, nil
	default:
		return 0, fmt.Errorf("unsupported read_mtime_unix_ns type %T", v)
	}
}

// findUniqueQuoteTolerantSpan returns rune indices [startRune, endRune) of the match in content.
func findUniqueQuoteTolerantSpan(content, old string) (startRune, endRune, matchCount int, errCode string) {
	if old == "" {
		return 0, 0, 0, "old_string_empty"
	}
	if !utf8.ValidString(content) || !utf8.ValidString(old) {
		return 0, 0, 0, "invalid_utf8"
	}
	cr := []rune(content)
	or := []rune(old)
	if len(or) > len(cr) {
		return 0, 0, 0, "old_string_not_found"
	}
	var starts []int
	for i := 0; i <= len(cr)-len(or); i++ {
		ok := true
		for j := 0; j < len(or); j++ {
			if normQuoteRune(cr[i+j]) != normQuoteRune(or[j]) {
				ok = false
				break
			}
		}
		if ok {
			starts = append(starts, i)
		}
	}
	n := len(starts)
	if n == 0 {
		return 0, 0, 0, "old_string_not_found"
	}
	if n > 1 {
		return 0, 0, n, "old_string_ambiguous"
	}
	s := starts[0]
	return s, s + len(or), 1, ""
}

func isClaudeSettingsJSONPath(absPath string) bool {
	c := filepath.Clean(absPath)
	// 约定：工作区内 .claude/settings.json
	if strings.HasSuffix(c, filepath.Join(".claude", "settings.json")) {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.Contains(c, sep+".claude"+sep+"settings.json")
}

func validateJSONDocument(b []byte) error {
	b = trimSpaceBytes(b)
	if len(b) == 0 {
		return fmt.Errorf("empty document")
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	return nil
}

func trimSpaceBytes(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
