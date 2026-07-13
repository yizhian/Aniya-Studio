package agent

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_AllPlaceholders(t *testing.T) {
	template := `Working directory: {{cwd}}
Date: {{date}}
Platform: {{platform}}
Memory: {{memory}}
User Memory: {{user_memory}}
Skills: {{skills}}
Tool Prompts: {{tool_prompts}}`

	ctx := SystemPromptContext{
		CWD:         "/home/user/project",
		Date:        "2026-05-02",
		Platform:    "darwin",
		Memory:      "legacy memory",
		UserMemory:  "user memory content",
		Skills:      "skill1: description\nskill2: description",
		ToolPrompts: "tool1: guide\ntool2: guide",
	}

	result := BuildSystemPrompt(template, ctx)

	if !strings.Contains(result, "/home/user/project") {
		t.Error("cwd not substituted")
	}
	if !strings.Contains(result, "2026-05-02") {
		t.Error("date not substituted")
	}
	if !strings.Contains(result, "darwin") {
		t.Error("platform not substituted")
	}
	if !strings.Contains(result, "user memory content") {
		t.Error("user_memory not substituted")
	}
	if !strings.Contains(result, "skill1: description") {
		t.Error("skills not substituted")
	}
	if !strings.Contains(result, "tool1: guide") {
		t.Error("tool_prompts not substituted")
	}
	// {{memory}} should fall back to UserMemory.
	if strings.Contains(result, "legacy memory") {
		t.Error("legacy {{memory}} should not appear; should use UserMemory fallback")
	}
}

func TestBuildSystemPrompt_ToolPromptsInjected(t *testing.T) {
	// Simulates the actual system.md template pattern.
	template := `# Tools
{{tool_prompts}}`

	ctx := SystemPromptContext{
		ToolPrompts: "- todo_write: Use this tool to manage tasks.\n- read_file: Read files.",
	}

	result := BuildSystemPrompt(template, ctx)

	if !strings.Contains(result, "todo_write: Use this tool to manage tasks") {
		t.Error("tool_prompts content should appear in output")
	}
	if !strings.Contains(result, "read_file: Read files") {
		t.Error("second tool prompt should appear")
	}
	// No leftover placeholder.
	if strings.Contains(result, "{{tool_prompts}}") {
		t.Error("placeholder should be fully replaced")
	}
}

func TestBuildSystemPrompt_EmptyToolPrompts(t *testing.T) {
	template := `pre{{tool_prompts}}post`

	ctx := SystemPromptContext{
		ToolPrompts: "",
	}

	result := BuildSystemPrompt(template, ctx)

	if !strings.Contains(result, "prepost") {
		t.Error("empty tool prompts should leave adjacent text intact")
	}
}

func TestBuildSystemPrompt_EmptySkills(t *testing.T) {
	template := `pre{{skills}}post`

	ctx := SystemPromptContext{
		Skills: "",
	}

	result := BuildSystemPrompt(template, ctx)
	if !strings.Contains(result, "prepost") {
		t.Error("empty skills should leave adjacent text intact")
	}
}

func TestBuildSystemPrompt_EmptyUserMemory(t *testing.T) {
	template := `pre{{user_memory}}post`

	ctx := SystemPromptContext{
		UserMemory: "",
	}

	result := BuildSystemPrompt(template, ctx)
	if !strings.Contains(result, "prepost") {
		t.Error("empty user memory should leave adjacent text intact")
	}
}

func TestBuildSystemPrompt_UnknownPlaceholderPreserved(t *testing.T) {
	template := `{{unknown_placeholder}}`

	ctx := DefaultSystemPromptContext()
	result := BuildSystemPrompt(template, ctx)

	// Unknown placeholders should remain for inspectability.
	if !strings.Contains(result, "{{unknown_placeholder}}") {
		t.Error("unknown placeholders should be preserved, not silently dropped")
	}
}

func TestBuildSystemPrompt_MemoryFallback(t *testing.T) {
	// When UserMemory is empty but Memory (legacy) is set, use Memory.
	template := `{{user_memory}}`

	ctx := SystemPromptContext{
		Memory:     "legacy memory value",
		UserMemory: "",
	}

	result := BuildSystemPrompt(template, ctx)
	if !strings.Contains(result, "legacy memory value") {
		t.Error("should fall back to legacy Memory when UserMemory is empty")
	}
}

func TestBuildSystemPrompt_UserMemoryPrecedence(t *testing.T) {
	// When both are set, UserMemory takes precedence.
	template := `{{user_memory}}`

	ctx := SystemPromptContext{
		Memory:     "legacy value",
		UserMemory: "current value",
	}

	result := BuildSystemPrompt(template, ctx)
	if !strings.Contains(result, "current value") {
		t.Error("UserMemory should take precedence over legacy Memory")
	}
	if strings.Contains(result, "legacy value") {
		t.Error("legacy Memory should not appear when UserMemory is set")
	}
}

func TestDefaultSystemPromptContext(t *testing.T) {
	ctx := DefaultSystemPromptContext()

	if ctx.CWD == "" {
		t.Error("CWD should not be empty")
	}
	if ctx.Date == "" {
		t.Error("Date should not be empty")
	}
	if ctx.Platform == "" {
		t.Error("Platform should not be empty")
	}
	// Memory, Skills, ToolPrompts should be left empty (populated by callers).
	if ctx.UserMemory != "" {
		t.Error("UserMemory should be empty by default")
	}
	if ctx.Skills != "" {
		t.Error("Skills should be empty by default")
	}
	if ctx.ToolPrompts != "" {
		t.Error("ToolPrompts should be empty by default")
	}
}

func TestBuildDefaultSystemPrompt_LoadsTemplate(t *testing.T) {
	ctx := DefaultSystemPromptContext()
	ctx.UserMemory = "test memory"
	ctx.Skills = "test skills"
	ctx.ToolPrompts = "test tool prompts"

	result := BuildDefaultSystemPrompt(ctx)

	// The template should be loaded and placeholders substituted.
	if result == "" {
		t.Fatal("BuildDefaultSystemPrompt returned empty string")
	}
	if !strings.Contains(result, "SlideCraft") {
		t.Error("system prompt should contain SlideCraft identity")
	}
	if !strings.Contains(result, "test memory") {
		t.Error("user memory should be injected")
	}
	if !strings.Contains(result, "test skills") {
		t.Error("skills should be injected")
	}
	if !strings.Contains(result, "test tool prompts") {
		t.Error("tool prompts should be injected")
	}
	// Verify that no unreplaced placeholders remain.
	for _, ph := range []string{"{{workspace_root}}", "{{date}}", "{{platform}}", "{{user_memory}}", "{{skills}}", "{{tool_prompts}}"} {
		if strings.Contains(result, ph) {
			t.Errorf("unreplaced placeholder found: %s", ph)
		}
	}
}

func TestSystemPromptTemplate_ContainsToolPromptsPlaceholder(t *testing.T) {
	tmpl := DefaultSystemPromptTemplate()

	if !strings.Contains(tmpl, "{{tool_prompts}}") {
		t.Error("system.md template must contain {{tool_prompts}} placeholder")
	}
	if !strings.Contains(tmpl, "{{skills}}") {
		t.Error("system.md template must contain {{skills}} placeholder")
	}
	if !strings.Contains(tmpl, "{{user_memory}}") {
		t.Error("system.md template must contain {{user_memory}} placeholder")
	}
	if !strings.Contains(tmpl, "{{workspace_root}}") {
		t.Error("system.md template must contain {{workspace_root}} placeholder")
	}
}

func TestSystemPromptTemplate_NoManualVersioning(t *testing.T) {
	tmpl := DefaultSystemPromptTemplate()

	// The manual versioning instructions should have been removed.
	if strings.Contains(tmpl, "copy the old file and version it") {
		t.Error("system prompt should NOT contain manual versioning instructions")
	}
	if strings.Contains(tmpl, "Product-Launch-v2.html") {
		t.Error("system prompt should NOT contain v2 naming example")
	}
}

func TestSystemPromptTemplate_ContainsRuntimeEnvironment(t *testing.T) {
	tmpl := DefaultSystemPromptTemplate()

	if !strings.Contains(tmpl, "Runtime Environment") {
		t.Error("system prompt should contain the Runtime Environment chapter")
	}
	if !strings.Contains(tmpl, "Context trimming") {
		t.Error("should explain context trimming")
	}
	if !strings.Contains(tmpl, "Version snapshots") {
		t.Error("should explain version snapshots")
	}
	if !strings.Contains(tmpl, "Session persistence") {
		t.Error("should explain session persistence")
	}
	if !strings.Contains(tmpl, "Memory recall") {
		t.Error("should explain memory recall")
	}
	if !strings.Contains(tmpl, "Tool execution") {
		t.Error("should explain tool execution model")
	}
}

func TestSystemPromptTemplate_ReferencesComplianceSkill(t *testing.T) {
	tmpl := DefaultSystemPromptTemplate()

	if !strings.Contains(tmpl, "grapesjs-html-compliance") {
		t.Error("system prompt should reference grapesjs-html-compliance skill")
	}
}

func TestSystemPromptTemplate_NoGrapesJSSpecificRules(t *testing.T) {
	tmpl := DefaultSystemPromptTemplate()

	// GrapesJS-specific constraints moved to SKILL.md — system prompt should
	// not duplicate them.
	noGoRules := []string{
		"data-gjs-* attributes",       // old Output Format rule
		"except data-gjs-type",        // old exception clause
		"Do NOT use data-gjs-*",       // old explicit prohibition
		"echarts.init()",              // old ECharts detailed rule — only high-level mention remains
		"echarts.init(dom)",           // variant
		"overflow-y: auto",            // old scrolling rule
	}
	for _, rule := range noGoRules {
		if strings.Contains(tmpl, rule) {
			t.Errorf("system prompt should NOT contain GrapesJS rule %q — it belongs in SKILL.md", rule)
		}
	}
}

func TestSystemPromptTemplate_NoHtmlChartReferences(t *testing.T) {
	tmpl := DefaultSystemPromptTemplate()

	// The system prompt must NOT reference the removed html-chart system.
	for _, forbidden := range []string{
		`data-gjs-type="html-chart"`,
		"data-gjs-type=\\\"html-chart\\\"",
		"html-chart",
		"data-chart-data",
		"data-chart-kind",
		"chart runtime is pre-loaded",
	} {
		if strings.Contains(tmpl, forbidden) {
			t.Errorf("system prompt must NOT contain %q — charts now use div+CSS+SVG", forbidden)
		}
	}
}

func TestSystemPromptTemplate_HasFirstSlideActiveRequirement(t *testing.T) {
	tmpl := DefaultSystemPromptTemplate()

	// The system prompt must require hardcoded class="active" on the first .slide.
	if !strings.Contains(tmpl, "FIRST") && !strings.Contains(tmpl, "first") {
		t.Error("system prompt should mention the first .slide active requirement")
	}
	if !strings.Contains(tmpl, "active") {
		t.Error("system prompt should reference .slide.active")
	}
	if !strings.Contains(tmpl, "class=\"active\" hardcoded in HTML") && !strings.Contains(tmpl, "First slide MUST") {
		t.Error("system prompt should explain WHY the first slide must hardcode active")
	}
}

func TestSystemPromptTemplate_HasDivCSSChartGuidance(t *testing.T) {
	tmpl := DefaultSystemPromptTemplate()

	// L74: Must contain div+CSS+SVG chart guidance.
	if !strings.Contains(tmpl, "div + CSS") && !strings.Contains(tmpl, "div+CSS") {
		t.Error("system prompt should explain div+CSS for layout charts")
	}
	if !strings.Contains(tmpl, "inline SVG") || !strings.Contains(tmpl, "viewBox") {
		t.Error("system prompt should explain inline SVG for complex geometry")
	}
	// Must warn about bare <table>.
	if !strings.Contains(tmpl, "<table>") && !strings.Contains(tmpl, "bare <table>") && !strings.Contains(tmpl, "<td>") {
		t.Error("system prompt should warn about bare <table> tags")
	}
}

func TestSystemPromptTemplate_HasPresentationWorkflow(t *testing.T) {
	tmpl := DefaultSystemPromptTemplate()

	if !strings.Contains(tmpl, "Presentation Workflow") {
		t.Error("system prompt should contain Presentation Workflow section")
	}
	if !strings.Contains(tmpl, "todo_write") {
		t.Error("workflow should reference todo_write")
	}
	if !strings.Contains(tmpl, "**Verify.**") || !strings.Contains(tmpl, "self-check") {
		t.Error("workflow should reference compliance self-review in Verify step")
	}
}
