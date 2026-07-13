package agent

import (
	"os"
	"runtime"
	"strings"
	"time"
)

// SystemPromptContext holds all dynamic values injected into the system prompt template.
// Extend this struct when new extension points are added.
type SystemPromptContext struct {
	CWD         string // working directory
	Date        string // current date (YYYY-MM-DD)
	Platform    string // OS platform (darwin, linux, windows)
	Memory      string // legacy; mapped to UserMemory
	UserMemory  string // user-scoped memory injected at startup (replaces {{user_memory}})
	Skills      string // available skills listing
	ToolPrompts string // per-tool usage hints collected from registry

	// Phase 4: skill system extension.
	SkillOverride    string // injected when user selected a specific design skill
	SkillSuggestions string // injected when server-side trigger matching found candidates
	WorkspaceRoot    string // workspace path for skills/ access (replaces ambiguous {{cwd}})

	// Phase 4: uploaded files context for read_file instructions.
	UploadedFiles string // file manifest built from upload_meta.json, injected as {{uploaded_files}}

	// ProjectBrief is the user's content brief from project.json.
	ProjectBrief string
}

// DefaultSystemPromptContext builds a context from the current runtime environment.
// Memory, Skills, and ToolPrompts are left empty — callers should populate them from
// their respective sources (memory store, skill registry, tool registry).
func DefaultSystemPromptContext() SystemPromptContext {
	cwd, _ := os.Getwd()
	return SystemPromptContext{
		CWD:      cwd,
		Date:     time.Now().Format("2006-01-02"),
		Platform: runtime.GOOS,
	}
}

// BuildSystemPrompt substitutes {{key}} placeholders in template with values from ctx.
// Unknown placeholders are left as-is so the template remains inspectable.
func BuildSystemPrompt(template string, ctx SystemPromptContext) string {
	userMemory := ctx.UserMemory
	if userMemory == "" {
		userMemory = ctx.Memory // fallback to legacy field
	}
	replacer := strings.NewReplacer(
		"{{cwd}}", ctx.CWD,
		"{{date}}", ctx.Date,
		"{{platform}}", ctx.Platform,
		"{{user_memory}}", userMemory,
		"{{memory}}", userMemory,
		"{{skills}}", ctx.Skills,
		"{{tool_prompts}}", ctx.ToolPrompts,
		"{{workspace_root}}", ctx.WorkspaceRoot,
		"{{uploaded_files}}", ctx.UploadedFiles,
		"{{skill_override}}", ctx.SkillOverride,
		"{{skill_suggestions}}", ctx.SkillSuggestions,
		"{{project_brief}}", ctx.ProjectBrief,
	)
	return replacer.Replace(template)
}

// BuildDefaultSystemPrompt loads the system prompt template from disk (with fallback) and substitutes context.
func BuildDefaultSystemPrompt(ctx SystemPromptContext) string {
	return BuildSystemPrompt(DefaultSystemPromptTemplate(), ctx)
}

