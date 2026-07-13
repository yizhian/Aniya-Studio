package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentgo/internal/model"
	"agentgo/internal/toolkit/contracts"
	"agentgo/internal/toolkit/engine"
	"agentgo/internal/toolkit/registry"

	"agentgo/internal/retry"
)

func TestReadFileTool(t *testing.T) {
	ws := t.TempDir()
	file := filepath.Join(ws, "a.txt")
	if err := os.WriteFile(file, []byte("line1\nline2\nline3"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := registry.NewToolRegistry()
	if err := r.Register(NewReadFileTool()); err != nil {
		t.Fatal(err)
	}
	exec := &engine.StreamingToolExecutor{Registry: r}
	out := exec.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:   `{"path":"a.txt","start_line":2,"end_line":3}`,
		CanUseTool: true,
		Context: contracts.ToolCallContext{
			WorkspacePath: ws,
		},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("unexpected error: %+v", out)
	}
	if !strings.Contains(out.Content, "2|line2") || !strings.Contains(out.Content, "3|line3") {
		t.Fatalf("unexpected content: %s", out.Content)
	}
	ms, ok := out.Metadata["read_mtime_unix_ns"].(string)
	if !ok || ms == "" {
		t.Fatalf("expected read_mtime_unix_ns string in metadata, got %#v", out.Metadata)
	}
}

func TestListFilesTool(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(ws, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "sub", "b.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := registry.NewToolRegistry()
	if err := r.Register(NewListFilesTool()); err != nil {
		t.Fatal(err)
	}
	exec := &engine.StreamingToolExecutor{Registry: r}
	out := exec.Execute(context.Background(), "list_files", contracts.ToolCallArgs{
		ArgsJSON:   `{"path":".","recursive":true}`,
		CanUseTool: true,
		Context: contracts.ToolCallContext{
			WorkspacePath: ws,
		},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("unexpected error: %+v", out)
	}
	if !strings.Contains(out.Content, "file\ta.txt") || !strings.Contains(out.Content, "file\tsub/b.txt") {
		t.Fatalf("unexpected list output: %s", out.Content)
	}
}

func TestGrepSearchTool(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("func alpha() {}\nfunc beta() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := registry.NewToolRegistry()
	if err := r.Register(NewGrepSearchTool()); err != nil {
		t.Fatal(err)
	}
	ex := &engine.StreamingToolExecutor{Registry: r}
	out := ex.Execute(context.Background(), "grep_search", contracts.ToolCallArgs{
		ArgsJSON:   `{"pattern":"func alpha","path":"."}`,
		CanUseTool: true,
		Context: contracts.ToolCallContext{
			WorkspacePath: ws,
		},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("grep_search: %+v", out)
	}
	if !strings.Contains(out.Content, "a.go:1:") {
		t.Fatalf("expected match line, got %q", out.Content)
	}
}

func TestWebFetchTool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><p>Hello 世界</p></body></html>"))
	}))
	t.Cleanup(srv.Close)

	reg := registry.NewToolRegistry()
	if err := reg.Register(NewWebFetchTool()); err != nil {
		t.Fatal(err)
	}
	tool, err := reg.Resolve("web_fetch")
	if err != nil {
		t.Fatal(err)
	}
	wf, ok := tool.(*WebFetchTool)
	if !ok {
		t.Fatalf("unexpected tool type %T", tool)
	}
	wf.client = srv.Client()
	wf.client.Timeout = 0

	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "web_fetch", contracts.ToolCallArgs{
		ArgsJSON:      `{"url":"` + srv.URL + `"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("web_fetch: %+v", out)
	}
	if !strings.Contains(out.Content, "Hello") || !strings.Contains(out.Content, "世界") {
		t.Fatalf("unexpected stripped body: %q", out.Content)
	}
	if strings.Contains(out.Content, "<html") {
		t.Fatalf("expected tags stripped: %q", out.Content)
	}
}

func TestEditFileRequiresReadMtime(t *testing.T) {
	ws := t.TempDir()
	p := filepath.Join(ws, "f.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	_ = reg.Register(NewEditFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "edit_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"f.txt","old_string":"x","new_string":"y"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "read_proof_missing" {
		t.Fatalf("expected read_proof_missing, got %+v", out)
	}
}

func TestEditFileSuccessAndMtime(t *testing.T) {
	ws := t.TempDir()
	p := filepath.Join(ws, "f.txt")
	if err := os.WriteFile(p, []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	_ = reg.Register(NewEditFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	readOut := ex.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"f.txt"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if readOut.IsError {
		t.Fatal(readOut)
	}
	mtime, _ := readOut.Metadata["read_mtime_unix_ns"].(string)
	args, _ := json.Marshal(map[string]string{
		"path": "f.txt", "old_string": "alpha", "new_string": "gamma",
		"read_mtime_unix_ns": mtime,
	})
	editOut := ex.Execute(context.Background(), "edit_file", contracts.ToolCallArgs{
		ArgsJSON:      string(args),
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if editOut.IsError {
		t.Fatal(editOut)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "gamma\nbeta\n" {
		t.Fatalf("file content: %q", b)
	}
}

func TestEditFileAmbiguous(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "f.txt"), []byte("aaa"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	_ = reg.Register(NewEditFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	readOut := ex.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"f.txt"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	mtime, _ := readOut.Metadata["read_mtime_unix_ns"].(string)
	args, _ := json.Marshal(map[string]string{
		"path": "f.txt", "old_string": "a", "new_string": "b", "read_mtime_unix_ns": mtime,
	})
	editOut := ex.Execute(context.Background(), "edit_file", contracts.ToolCallArgs{
		ArgsJSON:      string(args),
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !editOut.IsError || editOut.ErrorCode != "old_string_ambiguous" {
		t.Fatalf("expected ambiguous, got %+v", editOut)
	}
}

func TestEditFileQuoteTolerance(t *testing.T) {
	ws := t.TempDir()
	// 文件里是弯引号，模型侧用直引号 old_string
	line := "say " + "\u201chello\u201d" + " now\n"
	if err := os.WriteFile(filepath.Join(ws, "q.txt"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	_ = reg.Register(NewEditFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	readOut := ex.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"q.txt"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	mtime, _ := readOut.Metadata["read_mtime_unix_ns"].(string)
	old := `say "hello"`
	args, _ := json.Marshal(map[string]string{
		"path": "q.txt", "old_string": old, "new_string": "say 'hi'", "read_mtime_unix_ns": mtime,
	})
	editOut := ex.Execute(context.Background(), "edit_file", contracts.ToolCallArgs{
		ArgsJSON:      string(args),
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if editOut.IsError {
		t.Fatal(editOut)
	}
	b, _ := os.ReadFile(filepath.Join(ws, "q.txt"))
	if !strings.Contains(string(b), "hi") {
		t.Fatalf("content %q", b)
	}
}

func TestEditFileSettingsJSONInvalid(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, ".claude")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(p, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	_ = reg.Register(NewEditFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	readOut := ex.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":".claude/settings.json"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	mtime, _ := readOut.Metadata["read_mtime_unix_ns"].(string)
	args, _ := json.Marshal(map[string]string{
		"path":               ".claude/settings.json",
		"old_string":         `{"ok":true}`,
		"new_string":         `{"ok":true`, // 故意坏 JSON
		"read_mtime_unix_ns": mtime,
	})
	editOut := ex.Execute(context.Background(), "edit_file", contracts.ToolCallArgs{
		ArgsJSON:      string(args),
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !editOut.IsError || editOut.ErrorCode != "settings_json_invalid" {
		t.Fatalf("expected settings_json_invalid, got %+v", editOut)
	}
}

func TestWriteFileRejectsExisting(t *testing.T) {
	ws := t.TempDir()
	p := filepath.Join(ws, "e.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewWriteFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "write_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"e.txt","content":"y"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "file_exists" {
		t.Fatalf("expected file_exists, got %+v", out)
	}
}

func TestValidateNotSystemPath_RejectsDotSlidecraft(t *testing.T) {
	tests := []string{
		"/some/workspace/.slidecraft/versions/v1/test.html",
		"/some/workspace/.slidecraft/test.html",
		".slidecraft/versions/v1/test.html",
		"/workspace/project/.slidecraft/versions/v2/context.json",
	}
	for _, p := range tests {
		err := validateNotSystemPath(p)
		if err == nil {
			t.Errorf("expected error for path %q", p)
		}
	}
}

func TestValidateNotSystemPath_AllowsNormalPaths(t *testing.T) {
	tests := []string{
		"/some/workspace/deck.html",
		"/some/workspace/subdir/file.html",
		"deck.html",
		".agentgo/memory/design/test.md",
		"/workspace/project/index.html",
	}
	for _, p := range tests {
		err := validateNotSystemPath(p)
		if err != nil {
			t.Errorf("unexpected error for path %q: %v", p, err)
		}
	}
}

func TestValidateNotSystemPath_EdgeCases(t *testing.T) {
	// .slidecraft as part of a filename (not a directory) — should be allowed.
	t.Run("slidecraft in filename", func(t *testing.T) {
		err := validateNotSystemPath("/workspace/my.slidecraft.backup.html")
		if err != nil {
			t.Errorf("should allow .slidecraft as filename component, got: %v", err)
		}
	})
	// .slidecraft with trailing characters in the same segment.
	t.Run("slidecraft suffix in dir", func(t *testing.T) {
		err := validateNotSystemPath("/workspace/.slidecraft_backup/test.html")
		if err != nil {
			t.Errorf("should allow .slidecraft_backup directory, got: %v", err)
		}
	})
	// .slidecraft as exact directory with trailing slash.
	t.Run("exact match with trailing slash", func(t *testing.T) {
		err := validateNotSystemPath("/workspace/.slidecraft/")
		if err == nil {
			t.Error("should reject .slidecraft/ directory")
		}
	})
	// Deep nesting with .slidecraft buried.
	t.Run("deep nesting", func(t *testing.T) {
		err := validateNotSystemPath("/a/b/c/d/.slidecraft/e/f/g/test.json")
		if err == nil {
			t.Error("should reject deeply nested .slidecraft")
		}
	})
	// Relative path with .slidecraft.
	t.Run("relative path", func(t *testing.T) {
		err := validateNotSystemPath(".slidecraft/versions/v1/test.html")
		if err == nil {
			t.Error("should reject relative .slidecraft path")
		}
	})
	// .slidecraft at the very beginning of absolute path.
	t.Run("absolute slidecraft root", func(t *testing.T) {
		err := validateNotSystemPath("/.slidecraft/test.html")
		if err == nil {
			t.Error("should reject /.slidecraft path")
		}
	})
}

func TestWriteFileRejectsSystemPath(t *testing.T) {
	ws := t.TempDir()
	// Create .slidecraft directory so the path resolves.
	os.MkdirAll(filepath.Join(ws, ".slidecraft", "versions", "v1"), 0o755)
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewWriteFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "write_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":".slidecraft/versions/v1/test.html","content":"x"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "system_path" {
		t.Fatalf("expected system_path error, got %+v", out)
	}
}

func TestEditFileRejectsSystemPath(t *testing.T) {
	ws := t.TempDir()
	os.MkdirAll(filepath.Join(ws, ".slidecraft", "versions", "v1"), 0o755)
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	_ = reg.Register(NewEditFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "edit_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":".slidecraft/versions/v1/context.json","old_string":"x","new_string":"y","read_mtime_unix_ns":"1"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "system_path" {
		t.Fatalf("expected system_path error, got %+v", out)
	}
}

func TestHumanMessageForEditFail(t *testing.T) {
	tests := []struct {
		code   string
		n      int
		expect string
	}{
		{"old_string_not_found", 0, "old_string not found in file"},
		{"old_string_ambiguous", 3, "matches 3 times"},
		{"old_string_empty", 0, "old_string must not be empty"},
		{"unknown_code", 0, "unknown_code"},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := humanMessageForEditFail(tt.code, tt.n)
			if !strings.Contains(got, tt.expect) {
				t.Errorf("expected %q in message, got %q", tt.expect, got)
			}
		})
	}
}

func TestParseMtimeProof(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		wantOK  bool
		wantVal int64
	}{
		{"nil", nil, false, 0},
		{"string_valid", "123456789", true, 123456789},
		{"string_whitespace", "  999  ", true, 999},
		{"string_empty", "", false, 0},
		{"float64", float64(42), true, 42},
		{"int64", int64(777), true, 777},
		{"bool", true, false, 0},
		{"map", map[string]any{}, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := parseMtimeProof(tt.input)
			if tt.wantOK && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantOK && err == nil {
				t.Error("expected error")
			}
			if val != tt.wantVal {
				t.Errorf("expected %d, got %d", tt.wantVal, val)
			}
		})
	}
}

func TestReadFile_InvalidJSON(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:      `{not json}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "invalid_json" {
		t.Fatalf("expected invalid_json, got %+v", out)
	}
}

func TestReadFile_StatFailed(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"nonexistent.txt"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError {
		t.Fatalf("expected error for nonexistent file, got %+v", out)
	}
}

func TestReadFile_InvalidRange(t *testing.T) {
	ws := t.TempDir()
	p := filepath.Join(ws, "f.txt")
	if err := os.WriteFile(p, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"f.txt","start_line":5,"end_line":3}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "invalid_range" {
		t.Fatalf("expected invalid_range, got %+v", out)
	}
}

func TestReadFile_EndLineBeyondBounds(t *testing.T) {
	ws := t.TempDir()
	p := filepath.Join(ws, "f.txt")
	if err := os.WriteFile(p, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"f.txt","start_line":1,"end_line":999}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("unexpected error: %+v", out)
	}
	if !strings.Contains(out.Content, "line3") {
		t.Errorf("expected line3 in output, got %q", out.Content)
	}
}

func TestReadFile_DefaultLineRange(t *testing.T) {
	ws := t.TempDir()
	p := filepath.Join(ws, "f.txt")
	if err := os.WriteFile(p, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"f.txt"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("unexpected error: %+v", out)
	}
	if !strings.Contains(out.Content, "1|a") || !strings.Contains(out.Content, "3|c") {
		t.Errorf("expected all lines, got %q", out.Content)
	}
}

func TestReadFile_EmptyPath(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewReadFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "read_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":""}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "invalid_path" {
		t.Fatalf("expected invalid_path, got %+v", out)
	}
}

func TestGrepSearch_InvalidJSON(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewGrepSearchTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "grep_search", contracts.ToolCallArgs{
		ArgsJSON:      `{not json}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "invalid_json" {
		t.Fatalf("expected invalid_json, got %+v", out)
	}
}

func TestGrepSearch_EmptyPattern(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewGrepSearchTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "grep_search", contracts.ToolCallArgs{
		ArgsJSON:      `{"pattern":"  "}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "invalid_pattern" {
		t.Fatalf("expected invalid_pattern, got %+v", out)
	}
}

func TestGrepSearch_InvalidRegexp(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewGrepSearchTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "grep_search", contracts.ToolCallArgs{
		ArgsJSON:      `{"pattern":"[unclosed"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "invalid_regexp" {
		t.Fatalf("expected invalid_regexp, got %+v", out)
	}
}

func TestGrepSearch_StatFailed(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewGrepSearchTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "grep_search", contracts.ToolCallArgs{
		ArgsJSON:      `{"pattern":"x","path":"nonexistent_dir"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "stat_failed" {
		t.Fatalf("expected stat_failed, got %+v", out)
	}
}

func TestGrepSearch_NotDirectory(t *testing.T) {
	ws := t.TempDir()
	p := filepath.Join(ws, "file.txt")
	if err := os.WriteFile(p, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewGrepSearchTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "grep_search", contracts.ToolCallArgs{
		ArgsJSON:      `{"pattern":"content","path":"file.txt"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "not_directory" {
		t.Fatalf("expected not_directory, got %+v", out)
	}
}

func TestGrepSearch_NoMatches(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewGrepSearchTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "grep_search", contracts.ToolCallArgs{
		ArgsJSON:      `{"pattern":"NOTFOUND","path":"."}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("unexpected error: %+v", out)
	}
	if !strings.Contains(out.Content, "(no matches)") {
		t.Errorf("expected '(no matches)', got %q", out.Content)
	}
}

func TestGrepSearch_DefaultPath(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("func main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewGrepSearchTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	// Empty or missing path defaults to "."
	out := ex.Execute(context.Background(), "grep_search", contracts.ToolCallArgs{
		ArgsJSON:      `{"pattern":"func main"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("unexpected error: %+v", out)
	}
	if !strings.Contains(out.Content, "a.go:1:") {
		t.Errorf("expected match in a.go, got %q", out.Content)
	}
}

func TestListFiles_InvalidJSON(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewListFilesTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "list_files", contracts.ToolCallArgs{
		ArgsJSON:      `{broken`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "invalid_json" {
		t.Fatalf("expected invalid_json, got %+v", out)
	}
}

func TestListFiles_StatFailed(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewListFilesTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "list_files", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"nonexistent"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "stat_failed" {
		t.Fatalf("expected stat_failed, got %+v", out)
	}
}

func TestListFiles_NotDirectory(t *testing.T) {
	ws := t.TempDir()
	p := filepath.Join(ws, "readme.md")
	if err := os.WriteFile(p, []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewListFilesTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "list_files", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"readme.md"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "not_directory" {
		t.Fatalf("expected not_directory, got %+v", out)
	}
}

func TestListFiles_NonRecursive(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(ws, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewListFilesTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "list_files", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":".","recursive":false}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("unexpected error: %+v", out)
	}
	// Non-recursive should show children but not grandchildren.
	if !strings.Contains(out.Content, "a.txt") {
		t.Errorf("expected a.txt in listing, got %q", out.Content)
	}
	if !strings.Contains(out.Content, "subdir") {
		t.Errorf("expected subdir in listing, got %q", out.Content)
	}
}

func TestListFiles_MaxEntries(t *testing.T) {
	ws := t.TempDir()
	for i := range 5 {
		name := fmt.Sprintf("file_%d.txt", i)
		if err := os.WriteFile(filepath.Join(ws, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewListFilesTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "list_files", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":".","max_entries":3}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("unexpected error: %+v", out)
	}
	count := 0
	for _, line := range strings.Split(out.Content, "\n") {
		if line != "" {
			count++
		}
	}
	if count != 3 {
		t.Errorf("expected 3 entries, got %d", count)
	}
}

func TestListFiles_DefaultMaxEntries(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewListFilesTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "list_files", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"."}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatalf("unexpected error: %+v", out)
	}
}

func TestResolveWorkspacePath_Empty(t *testing.T) {
	_, err := resolveWorkspacePath("/ws", "")
	if err == nil {
		t.Error("expected error for empty path")
	}
	_, err = resolveWorkspacePath("/ws", "   ")
	if err == nil {
		t.Error("expected error for whitespace-only path")
	}
}

func TestResolveWorkspacePath_Absolute(t *testing.T) {
	ws := t.TempDir()
	p := filepath.Join(ws, "sub", "file.txt")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("x"), 0o644)
	resolved, err := resolveWorkspacePath(ws, p)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != p {
		t.Errorf("expected %q, got %q", p, resolved)
	}
}

func TestResolveWorkspacePath_OutsideWorkspace(t *testing.T) {
	ws := t.TempDir()
	_, err := resolveWorkspacePath(ws, "../outside")
	if err == nil {
		t.Error("expected error for path outside workspace")
	}
}

func TestResolveWorkspacePath_EmptyWorkspace(t *testing.T) {
	// Empty workspace falls back to absolute path resolution.
	_, err := resolveWorkspacePath("", "relative/path")
	if err != nil {
		t.Errorf("unexpected error when workspace is empty: %v", err)
	}
}

func TestIsTextCandidate(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".go", true},
		{".txt", true},
		{".md", true},
		{".html", true},
		{".json", true},
		{".png", false},
		{".jpg", false},
		{".jpeg", false},
		{".gif", false},
		{".webp", false},
		{".ico", false},
		{".pdf", false},
		{".zip", false},
		{".gz", false},
		{".tar", false},
		{".7z", false},
		{".bin", false},
		{".so", false},
		{".dylib", false},
		{".dll", false},
		{".exe", false},
		{"", true},       // no extension
		{".unknown", true}, // unknown extension
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := isTextCandidate("file" + tt.ext)
			if got != tt.expected {
				t.Errorf("isTextCandidate(%q) = %v, want %v", tt.ext, got, tt.expected)
			}
		})
	}
}

func TestWebFetch_InvalidJSON(t *testing.T) {
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewWebFetchTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "web_fetch", contracts.ToolCallArgs{
		ArgsJSON:      `{not json`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "invalid_json" {
		t.Fatalf("expected invalid_json, got %+v", out)
	}
}

func TestWebFetch_EmptyURL(t *testing.T) {
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewWebFetchTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "web_fetch", contracts.ToolCallArgs{
		ArgsJSON:      `{"url":""}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if !out.IsError || out.ErrorCode != "invalid_url" {
		t.Fatalf("expected invalid_url, got %+v", out)
	}
}

func TestNormQuoteRune(t *testing.T) {
	tests := []struct {
		input    rune
		expected rune
	}{
		{'“', '"'}, // left double
		{'”', '"'}, // right double
		{'‘', '\''}, // left single
		{'’', '\''}, // right single
		{'a', 'a'},       // normal char
		{'1', '1'},       // digit
		{'"', '"'},       // straight double quote unchanged
		{'\'', '\''},     // straight single quote unchanged
	}
	for _, tt := range tests {
		got := normQuoteRune(tt.input)
		if got != tt.expected {
			t.Errorf("normQuoteRune(%U) = %U, want %U", tt.input, got, tt.expected)
		}
	}
}

// TestToolDescriptorPathHardening verifies all path-accepting tools
// explicitly reject absolute paths in their parameter descriptions.
func TestToolDescriptorPathHardening(t *testing.T) {
	tools := []struct {
		name       string
		descriptor contracts.ToolDescriptor
	}{
		{"read_file", NewReadFileTool().Descriptor()},
		{"write_file", NewWriteFileTool().Descriptor()},
		{"edit_file", NewEditFileTool().Descriptor()},
		{"list_files", NewListFilesTool().Descriptor()},
		{"grep_search", NewGrepSearchTool().Descriptor()},
	}

	for _, tt := range tools {
		t.Run(tt.name, func(t *testing.T) {
			props, ok := tt.descriptor.InputJSONSchema["properties"].(map[string]any)
			if !ok {
				t.Fatal("missing properties in schema")
			}
			pathProp, ok := props["path"].(map[string]any)
			if !ok {
				t.Fatal("missing path property in schema")
			}
			desc, ok := pathProp["description"].(string)
			if !ok {
				t.Fatal("path description is not a string")
			}
			if desc == "" {
				t.Error("path description is empty")
			}
			// All path descriptions must mention "absolute" or "relative"
			hasAbs := strings.Contains(desc, "absolute")
			hasRel := strings.Contains(desc, "relative")
			if !hasAbs && !hasRel {
				t.Errorf("path description should mention absolute/relative paths: %q", desc)
			}
		})
	}
}

func TestWriteFileCreatesNew(t *testing.T) {
	ws := t.TempDir()
	reg := registry.NewToolRegistry()
	_ = reg.Register(NewWriteFileTool())
	ex := &engine.StreamingToolExecutor{Registry: reg}
	out := ex.Execute(context.Background(), "write_file", contracts.ToolCallArgs{
		ArgsJSON:      `{"path":"new/n.txt","content":"hello"}`,
		CanUseTool:    true,
		Context:       contracts.ToolCallContext{WorkspacePath: ws},
		ParentMessage: model.Message{Role: "assistant"},
	})
	if out.IsError {
		t.Fatal(out)
	}
	b, err := os.ReadFile(filepath.Join(ws, "new", "n.txt"))
	if err != nil || string(b) != "hello" {
		t.Fatalf("read %v %q", err, b)
	}
}

// ---------------------------------------------------------------------------
// WebFetch — retry and status-code gating
// ---------------------------------------------------------------------------

func TestWebFetch_RetryOn503(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>success</body></html>"))
	}))
	defer srv.Close()

	tool := NewWebFetchTool()
	// Override client timeout with a shorter one for tests.
	tool.client = srv.Client()

	result := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: fmt.Sprintf(`{"url":"%s"}`, srv.URL),
	})
	if result.IsError {
		t.Fatalf("expected success after retry, got error: %s", result.ErrorMessage)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", callCount)
	}
	if !strings.Contains(result.Content, "success") {
		t.Errorf("expected 'success' in content, got: %s", result.Content)
	}
}

func TestWebFetch_RetryOn429(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><body>rate-limited-then-ok</body></html>"))
	}))
	defer srv.Close()

	tool := NewWebFetchTool()
	tool.client = srv.Client()

	result := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: fmt.Sprintf(`{"url":"%s"}`, srv.URL),
	})
	if result.IsError {
		t.Fatalf("expected success after 429 retry, got error: %s", result.ErrorMessage)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestWebFetch_NoRetryOn404(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	tool := NewWebFetchTool()
	tool.client = srv.Client()

	result := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: fmt.Sprintf(`{"url":"%s"}`, srv.URL),
	})
	if !result.IsError {
		t.Fatal("expected error for 404 (non-retryable)")
	}
	if callCount != 1 {
		t.Errorf("expected exactly 1 call (no retry on 404), got %d", callCount)
	}
}

func TestWebFetch_RetryExhausted(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadGateway) // 502, retryable
	}))
	defer srv.Close()

	tool := NewWebFetchTool()
	tool.client = srv.Client()

	result := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: fmt.Sprintf(`{"url":"%s"}`, srv.URL),
	})
	if !result.IsError {
		t.Fatal("expected error after retries exhausted")
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (1 initial + 2 retries = 3 total), got %d", callCount)
	}
}

func TestWebFetch_NoRetryOn401(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	tool := NewWebFetchTool()
	tool.client = srv.Client()

	result := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: fmt.Sprintf(`{"url":"%s"}`, srv.URL),
	})
	if !result.IsError {
		t.Fatal("expected error for 401 (non-retryable)")
	}
	if callCount != 1 {
		t.Errorf("expected exactly 1 call (no retry on 401), got %d", callCount)
	}
}

func TestWebFetch_RetryableHTTPStatus(t *testing.T) {
	tests := []struct {
		code      int
		retryable bool
	}{
		{500, true},
		{502, true},
		{503, true},
		{504, true},
		{408, true},
		{429, true},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{200, false},
		{301, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			got := retry.IsRetryableHTTPStatus(tt.code)
			if got != tt.retryable {
				t.Errorf("IsRetryableHTTPStatus(%d) = %v, want %v", tt.code, got, tt.retryable)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// looksLikeHTML tests
// ---------------------------------------------------------------------------

func TestLooksLikeHTML_Positive(t *testing.T) {
	tests := []string{
		"<html><body>hello</body></html>",
		"   <HTML lang=\"en\">content</HTML>",
		"before stuff <body class=\"main\">body content</body>",
		"<BODY onload=\"init()\">page</BODY>",
	}
	for _, s := range tests {
		if !looksLikeHTML(s) {
			t.Errorf("expected looksLikeHTML=true for: %q", s)
		}
	}
}

func TestLooksLikeHTML_Negative(t *testing.T) {
	tests := []string{
		"",
		"short",
		"just a plain text response",
		"{\"json\": \"object with html key but not tag\"}",
		"<div>no html or body tag here</div>",
	}
	for _, s := range tests {
		if looksLikeHTML(s) {
			t.Errorf("expected looksLikeHTML=false for: %q", s)
		}
	}
}

func TestLooksLikeHTML_WhitespaceTrim(t *testing.T) {
	// 19 chars = short-circuited.
	short := "   \n\r\t<html>  "
	if looksLikeHTML(short) {
		t.Error("short strings should return false even with <html>")
	}
	// 20+ chars with <html>.
	long := "   \n\r\t<html>more stuff here"
	if !looksLikeHTML(long) {
		t.Error("long string with <html> after trim should return true")
	}
}
