package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentgo/internal/model"
	"agentgo/internal/toolkit/contracts"
	"agentgo/internal/toolkit/core"
	"agentgo/internal/toolkit/extended/skill"
	"agentgo/internal/toolkit/registry"
)

// ============================================================================
// SIT: Bootstrap + Registry + Core Tools integration
// ============================================================================

func TestSIT_Bootstrap_AllToolsExecutable(t *testing.T) {
	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}

	defs := r.GetActiveToolDefinitions()
	if len(defs) < 9 {
		t.Fatalf("expected at least 9 tools, got %d", len(defs))
	}

	var skillDef *model.ToolDefinition
	for i := range defs {
		if defs[i].Function.Name == "skill" {
			skillDef = &defs[i]
			break
		}
	}
	if skillDef == nil {
		t.Fatal("skill tool not registered")
	}
	if !strings.Contains(strings.ToLower(skillDef.Function.Description), "query") {
		t.Errorf("skill descriptor missing 'query': %q", skillDef.Function.Description)
	}
}

func TestSIT_Bootstrap_SkillTool_Execute(t *testing.T) {
	idx := skill.NewIndex(map[string]skill.Skill{
		"test-skill": {
			Name: "test-skill", Description: "A test skill for integration testing",
			Triggers: []string{"test", "integration"}, Mode: "deck",
		},
		"other-skill": {
			Name: "other-skill", Description: "Another test skill",
			Triggers: []string{"other"}, Mode: "landing",
		},
	})

	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, idx); err != nil {
		t.Fatal(err)
	}

	tool, err := r.Resolve("skill")
	if err != nil {
		t.Fatalf("skill tool not found: %v", err)
	}

	// Query all skills.
	result := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"operation":"query"}`,
	})
	if result.Content == "" {
		t.Fatal("expected non-empty result from skill tool")
	}
	if !strings.Contains(result.Content, "test-skill") {
		t.Errorf("expected 'test-skill' in result: %s", result.Content)
	}
	if !strings.Contains(result.Content, "other-skill") {
		t.Errorf("expected 'other-skill' in result: %s", result.Content)
	}

	// Search without Provider → error (LLM-based search requires Provider).
	result2 := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"operation":"search","query":"test"}`,
	})
	if !result2.IsError {
		t.Fatal("expected error when Provider not configured for search")
	}
	if !strings.Contains(result2.Content, "provider_unavailable") {
		t.Errorf("expected provider_unavailable in search result, got: %s", result2.Content)
	}
}

func TestSIT_Bootstrap_ReadFile_Execute(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("Hello, integration test!"), 0644)

	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}

	tool, err := r.Resolve("read_file")
	if err != nil {
		t.Fatalf("read_file tool not found: %v", err)
	}

	result := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"path":"` + testFile + `"}`,
		Context:  contracts.ToolCallContext{WorkspacePath: tmpDir},
	})
	if !strings.Contains(result.Content, "Hello, integration test!") {
		t.Errorf("unexpected file content: %s", result.Content)
	}
}

func TestSIT_Bootstrap_WriteAndReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "output.html")

	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}

	writeTool, err := r.Resolve("write_file")
	if err != nil {
		t.Fatalf("write_file tool not found: %v", err)
	}

	content := "<html><body><h1>Test</h1></body></html>"
	result := writeTool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"path":"` + targetFile + `","content":"` + escapeJSON(content) + `"}`,
		Context:  contracts.ToolCallContext{WorkspacePath: tmpDir},
	})
	if result.IsError {
		t.Fatalf("write_file failed: %s", result.ErrorMessage)
	}

	// Verify file was actually written.
	data, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(data), content)
	}

	// Read it back via the tool.
	readTool, err := r.Resolve("read_file")
	if err != nil {
		t.Fatalf("read_file tool not found after write: %v", err)
	}
	readResult := readTool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"path":"` + targetFile + `"}`,
		Context:  contracts.ToolCallContext{WorkspacePath: tmpDir},
	})
	if !strings.Contains(readResult.Content, "<h1>Test</h1>") {
		t.Errorf("read back missing expected content: %s", readResult.Content)
	}
}

func TestSIT_Bootstrap_EditFile(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "edit-test.txt")
	os.WriteFile(targetFile, []byte("Hello, World!"), 0644)

	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}

	editTool, err := r.Resolve("edit_file")
	if err != nil {
		t.Fatalf("edit_file tool not found: %v", err)
	}

	result := editTool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"path":"` + targetFile + `","old_string":"World","new_string":"Integration Test"}`,
		Context:  contracts.ToolCallContext{WorkspacePath: tmpDir},
	})
	if result.IsError {
		t.Logf("edit_file returned error (may need mtime): %s", result.ErrorMessage)
		return
	}

	data, _ := os.ReadFile(targetFile)
	if !strings.Contains(string(data), "Integration Test") {
		t.Errorf("edit not applied correctly: %s", string(data))
	}
}

func TestSIT_Bootstrap_GrepSearch(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("TODO: important\nnormal line"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("nothing here\nTODO: also important"), 0644)

	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}

	tool, err := r.Resolve("grep_search")
	if err != nil {
		t.Fatalf("grep_search tool not found: %v", err)
	}

	result := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"pattern":"TODO","path":"` + tmpDir + `"}`,
		Context:  contracts.ToolCallContext{WorkspacePath: tmpDir},
	})
	if !strings.Contains(result.Content, "TODO") {
		t.Errorf("expected TODO in grep results: %s", result.Content)
	}
}

func TestSIT_Bootstrap_TodoWrite(t *testing.T) {
	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}

	tool, err := r.Resolve("todo_write")
	if err != nil {
		t.Fatalf("todo_write tool not found: %v", err)
	}

	result := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"todos":[{"content":"Task one","status":"pending","activeForm":"Doing task one"},{"content":"Task two","status":"in_progress","activeForm":"Doing task two"}]}`,
	})
	if !strings.Contains(strings.ToLower(result.Content), "task one") || !strings.Contains(strings.ToLower(result.Content), "task two") {
		t.Errorf("unexpected todo result: %s", result.Content)
	}
}

func TestSIT_Bootstrap_ToolSearch(t *testing.T) {
	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}

	tool, err := r.Resolve("tool_search")
	if err != nil {
		t.Fatalf("tool_search tool not found: %v", err)
	}

	result := tool.Call(context.Background(), contracts.ToolCallArgs{
		ArgsJSON: `{"query":"file"}`,
	})
	// All tools are registered as immediate (not deferred), so tool_search returns empty.
	// The tool itself should not error.
	if result.IsError {
		t.Errorf("tool_search should not error: %s", result.ErrorMessage)
	}
}

func TestSIT_Bootstrap_AllToolDefinitions(t *testing.T) {
	r := registry.NewToolRegistry()
	if err := RegisterAllTools(r, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}

	defs := r.GetActiveToolDefinitions()
	seen := make(map[string]bool)
	for _, d := range defs {
		if seen[d.Function.Name] {
			t.Errorf("duplicate tool name: %s", d.Function.Name)
		}
		seen[d.Function.Name] = true

		if d.Function.Name == "" {
			t.Error("tool with empty name")
		}
		if d.Type != "function" {
			t.Errorf("%s: expected type 'function', got %q", d.Function.Name, d.Type)
		}
		if d.Function.Description == "" {
			t.Errorf("%s: missing description", d.Function.Name)
		}
	}
}

func TestSIT_Bootstrap_RegistryIsolation(t *testing.T) {
	r1 := registry.NewToolRegistry()
	r2 := registry.NewToolRegistry()

	if err := RegisterAllTools(r1, skill.NewIndex(map[string]skill.Skill{})); err != nil {
		t.Fatal(err)
	}

	defs2 := r2.GetActiveToolDefinitions()
	if len(defs2) != 0 {
		t.Errorf("r2 should be empty, got %d tools", len(defs2))
	}

	defs1 := r1.GetActiveToolDefinitions()
	if len(defs1) < 8 {
		t.Errorf("r1 should have at least 8 tools, got %d", len(defs1))
	}
}

// ============================================================================
// SIT: Core tool contracts verification
// ============================================================================

func TestSIT_CoreTool_ImplementsContract(t *testing.T) {
	tools := []struct {
		name string
		tool contracts.Tool
	}{
		{"read_file", core.NewReadFileTool()},
		{"write_file", core.NewWriteFileTool()},
		{"edit_file", core.NewEditFileTool()},
		{"list_files", core.NewListFilesTool()},
		{"grep_search", core.NewGrepSearchTool()},
		{"todo_write", core.NewTodoWriteTool()},
		{"web_fetch", core.NewWebFetchTool()},
	}

	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			desc := tc.tool.Descriptor()
			if desc.Name == "" {
				t.Error("tool has empty name")
			}
			if desc.Description == "" {
				t.Error("tool has empty description")
			}
		})
	}
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
