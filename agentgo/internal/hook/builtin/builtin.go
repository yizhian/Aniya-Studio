// Package builtin registers all built-in Hook functions with the engine.
package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agentgo/internal/hook"
)

// RegisterBuiltins registers all built-in hooks on the engine.
func RegisterBuiltins(engine *hook.Engine) {
	// === user_prompt_submit hooks ===
	// init-check always runs first to validate workspace existence.

	engine.Register(&hook.RegisteredHook{
		Name:     "init-check",
		On:       hook.PointUserPromptSubmit,
		Stage:    "always",
		Priority: 10,
		Fn:       InitCheck,
	})

	engine.Register(&hook.RegisteredHook{
		Name:     "skill-loading-injector",
		On:       hook.PointUserPromptSubmit,
		Stage:    "always",
		Priority: 15,
		Fn:       SkillLoadingInjector,
	})

	// === pre_context_assemble hooks ===

	engine.Register(&hook.RegisteredHook{
		Name:     "quality-inject",
		On:       hook.PointPreContextAssemble,
		Stage:    "always",
		Priority: 5,
		Fn:       QualityInject,
		Builtin:  true,
	})

	// === pre_tool_use hooks ===

	engine.Register(&hook.RegisteredHook{
		Name:     "file-type-whitelist",
		On:       hook.PointPreToolUse,
		Stage:    "always",
		Priority: 10,
		Matcher:  &hook.Matcher{ToolNames: []string{"write_file"}},
		Fn:       FileTypeWhitelist,
	})

	engine.Register(&hook.RegisteredHook{
		Name:     "read-proof-pre-check",
		On:       hook.PointPreToolUse,
		Stage:    "always",
		Priority: 20,
		Matcher:  &hook.Matcher{ToolNames: []string{"edit_file"}},
		Fn:       ReadProofPreCheck,
	})


	engine.Register(&hook.RegisteredHook{
		Name:     "design-skill-required",
		On:       hook.PointPreToolUse,
		Stage:    "always",
		Priority: 30,
		Matcher: &hook.Matcher{
			ToolNames:    []string{"write_file"},
			PathPatterns: []string{"*.html"},
		},
		Fn: DesignSkillRequired,
	})

	// === post_tool_use hooks ===

	engine.Register(&hook.RegisteredHook{
		Name:     "consecutive-failure-detector",
		On:       hook.PointPostToolUse,
		Stage:    "always",
		Priority: 20,
		Fn:       ConsecutiveFailureDetector,
	})

	engine.Register(&hook.RegisteredHook{
		Name:     "duplicate-call-detector",
		On:       hook.PointPostToolUse,
		Stage:    "always",
		Priority: 30,
		Fn:       DuplicateCallDetector,
	})

	engine.Register(&hook.RegisteredHook{
		Name:     "compliance-review-trigger",
		On:       hook.PointPostToolUse,
		Stage:    "always",
		Priority: 40,
		Matcher: &hook.Matcher{
			ToolNames:    []string{"write_file", "edit_file"},
			PathPatterns: []string{"*.html"},
		},
		Fn: ComplianceReviewTrigger,
	})

	engine.Register(&hook.RegisteredHook{
		Name:     "post_round_loop_guard",
		On:       hook.PointPostRound,
		Stage:    "always",
		Priority: 10,
		Fn:       PostRoundLoopGuard,
	})
}

// ---------------------------------------------------------------------------
// config helpers — read strategy values from Settings with defaults
// ---------------------------------------------------------------------------

func cfgThreshold(hctx *hook.HookContext) *hook.Settings {
	if hctx == nil || hctx.Config == nil {
		return &hook.DefaultConfig().Settings
	}
	return &hctx.Config.Settings
}

func cfgMsg(hctx *hook.HookContext) hook.HookMessages {
	return cfgThreshold(hctx).Messages
}

func cfgCompliance(hctx *hook.HookContext) hook.ComplianceSettings {
	return cfgThreshold(hctx).Compliance
}

// ---------------------------------------------------------------------------
// user_prompt_submit hooks
// ---------------------------------------------------------------------------

// InitCheck validates that the workspace directory exists.
func InitCheck(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
	if hctx.WorkspacePath == "" {
		return hook.HookResult{Action: hook.Allow}
	}
	if _, err := os.Stat(hctx.WorkspacePath); os.IsNotExist(err) {
		return hook.HookResult{
			Action: hook.Block,
			Reason: fmt.Sprintf(cfgMsg(hctx).WorkspaceBlock, hctx.WorkspacePath),
		}
	}
	return hook.HookResult{Action: hook.Allow}
}

// SkillLoadingInjector injects stage-appropriate workflow instructions at the
// start of every conversation. Simplified: system prompt already has full workflow
// instructions via SkillOverride + ProjectBrief. Lightweight edit-mode reminder only.
func SkillLoadingInjector(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
	if hctx.Stage != hook.StageInitialGeneration {
		return hook.HookResult{
			Action: hook.Warn,
			Reason: "[edit-mode]\n" +
				"You are editing an existing HTML file. Ensure all changes comply with GrapesJS " +
				"compatibility rules. To review the rules, load the " +
				"grapesjs-html-compliance skill.",
		}
	}
	return hook.HookResult{Action: hook.Allow}
}

// ---------------------------------------------------------------------------
// pre_tool_use hooks
// ---------------------------------------------------------------------------

// FileTypeWhitelist blocks file creation for disallowed extensions.
func FileTypeWhitelist(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
	path, _ := hctx.ToolArgs["path"].(string)
	if path == "" {
		return hook.HookResult{Action: hook.Allow}
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return hook.HookResult{
			Action: hook.Warn,
			Reason: cfgMsg(hctx).NoExtensionWarn,
		}
	}
	allowed := cfgThreshold(hctx).AllowedExtensions
	if len(allowed) == 0 {
		return hook.HookResult{Action: hook.Allow}
	}
	for _, a := range allowed {
		if ext == a {
			return hook.HookResult{Action: hook.Allow}
		}
	}
	return hook.HookResult{
		Action: hook.Block,
		Reason: fmt.Sprintf(cfgMsg(hctx).FileTypeBlock,
			ext, strings.Join(allowed, ", ")),
	}
}

// ReadProofPreCheck blocks edit_file when the target hasn't been read this session.
func ReadProofPreCheck(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
	if hctx.SessionState == nil {
		return hook.HookResult{Action: hook.Allow}
	}
	path, _ := hctx.ToolArgs["path"].(string)
	if path == "" {
		return hook.HookResult{Action: hook.Allow}
	}
	absPath, err := hook.ResolveWorkspacePath(hctx.WorkspacePath, path)
	if err != nil {
		return hook.HookResult{Action: hook.Allow}
	}

	readMtime, wasRead := hctx.SessionState.WasFileRead(absPath)
	if !wasRead {
		return hook.HookResult{
			Action: hook.Block,
			Reason: fmt.Sprintf(cfgMsg(hctx).ReadProofBlock, path),
		}
	}
	currentMtime := fileMtime(absPath)
	if currentMtime > 0 && readMtime > 0 && currentMtime != readMtime {
		return hook.HookResult{
			Action: hook.Warn,
			Reason: fmt.Sprintf(cfgMsg(hctx).StaleReadWarn, path),
		}
	}

	return hook.HookResult{Action: hook.Allow}
}

// DesignSkillRequired blocks write_file targeting *.html during initial generation
// unless the grapesjs-html-compliance skill has been loaded. During iterative_edit,
// it warns if write_file targets an existing HTML file (prefer edit_file).
func DesignSkillRequired(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
	switch hctx.Stage {
	case hook.StageInitialGeneration:
		if hctx.SessionState == nil {
			return hook.HookResult{Action: hook.Allow}
		}
		if hctx.SessionState.IsSkillLoaded("grapesjs-html-compliance") {
			return hook.HookResult{Action: hook.Allow}
		}
		return hook.HookResult{
			Action: hook.Block,
			Reason: "You must load the grapesjs-html-compliance skill first. " +
				"Learn GrapesJS compatibility rules before creating any HTML file.",
		}

	case hook.StageIterativeEdit:
		path, _ := hctx.ToolArgs["path"].(string)
		if path == "" {
			return hook.HookResult{Action: hook.Allow}
		}
		absPath, err := hook.ResolveWorkspacePath(hctx.WorkspacePath, path)
		if err != nil {
			return hook.HookResult{Action: hook.Allow}
		}
		if _, err := os.Stat(absPath); err == nil {
			return hook.HookResult{
				Action: hook.Warn,
				Reason: fmt.Sprintf(
					"[edit-reminder] Target file %s already exists. Use edit_file for incremental changes, "+
						"not write_file to rewrite the entire file.", path),
			}
		}
		return hook.HookResult{Action: hook.Allow}
	}
	return hook.HookResult{Action: hook.Allow}
}

// ---------------------------------------------------------------------------
// post_tool_use hooks
// ---------------------------------------------------------------------------

// ConsecutiveFailureDetector warns when a tool repeatedly fails.
func ConsecutiveFailureDetector(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
	if hctx.SessionState == nil || hctx.ToolResult == nil || !hctx.ToolResult.IsError {
		return hook.HookResult{Action: hook.Allow}
	}
	failures := hctx.SessionState.GetToolFailureCount(hctx.ToolName)
	threshold := hctx.SessionState.GetMaxConsecutiveFailures()
	if threshold <= 0 {
		threshold = cfgThreshold(hctx).MaxConsecutiveFailures
	}
	if threshold <= 0 {
		threshold = 3
	}
	if failures >= threshold {
		return hook.HookResult{
			Action: hook.Warn,
			Reason: fmt.Sprintf(cfgMsg(hctx).ConsecutiveFailWarn, hctx.ToolName, failures),
		}
	}
	return hook.HookResult{Action: hook.Allow}
}

// DuplicateCallDetector warns when the same failing call is repeated.
func DuplicateCallDetector(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
	if hctx.SessionState == nil {
		return hook.HookResult{Action: hook.Allow}
	}
	threshold := cfgThreshold(hctx).DuplicateCallWarnCount
	if threshold <= 0 {
		threshold = 3
	}
	key := hctx.ToolName + ":" + hashArgs(hctx.ToolArgs)
	count := hctx.SessionState.GetCallHistoryCount(key)
	if count >= threshold && hctx.ToolResult != nil && hctx.ToolResult.IsError {
		return hook.HookResult{
			Action: hook.Warn,
			Reason: cfgMsg(hctx).DuplicateCallWarn,
		}
	}
	return hook.HookResult{Action: hook.Allow}
}

// ComplianceReviewTrigger provides a 2-dimensional review system:
//
// Dimension 1 — Event Kind (governs which counter and throttle curve):
//   - Publish (write_file): full checklist on first, short reminder on 2nd–3rd, silent after.
//   - Patch (edit_file): short reminder on first, silent after. Never emits full checklist.
//
// Dimension 2 — Validator Class:
//   - htmlchecker (Objective): S1–S10 via validateHTMLCompliance.
//     Publish: uses ToolArgs["content"]. Always runs; violations are never throttled.
//     Patch: runs only when EnableCSSOnPatch is true (reads post-edit file from disk
//     and runs full S1–S10). When false, Patch skips htmlchecker entirely.
//   - selfchecklist (Subjective): Part 3 checklist, throttled by Publish/Patch counters.
func ComplianceReviewTrigger(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
	if hctx.ToolResult == nil || hctx.ToolResult.IsError {
		return hook.HookResult{Action: hook.Allow}
	}

	cfg := cfgCompliance(hctx)

	// ── htmlchecker: deterministic compliance validation ──
	path, _ := hctx.ToolArgs["path"].(string)
	if path != "" && hook.IsHTMLPath(path) {
		var content string
		if hctx.ToolName == "write_file" {
			content, _ = hctx.ToolArgs["content"].(string)
		} else if cfg.EnableCSSOnPatch {
			absPath, err := hook.ResolveWorkspacePath(hctx.WorkspacePath, path)
			if err == nil {
				content = hook.ReadFileWithCap(absPath, 2*1024*1024, hctx.WorkspacePath)
			}
		}
		if content != "" {
			issues := validateHTMLCompliance(content)
			if len(issues) > 0 {
				var lines []string
				for _, iss := range issues {
					lines = append(lines, iss.Message)
				}
				return hook.HookResult{
					Action: hook.Warn,
					Reason: "[htmlchecker] The written HTML has compliance issues:\n" +
						strings.Join(lines, "\n") +
						"\n\nFix each one immediately with edit_file. Do NOT use write_file to fix an existing file.",
				}
			}
		}
	}

	// ── selfchecklist: Part 3 self-review checklist (throttled per event kind) ──
	pubCount, patchCount := 0, 0
	if hctx.SessionState != nil {
		pubCount = hctx.SessionState.GetHTMLPublishCount()
		patchCount = hctx.SessionState.GetHTMLPatchCount()
	}

	if hctx.ToolName == "edit_file" {
		// Patch: short reminder up to PatchWarnMax, then silent.
		// Never emits the full checklist.
		if patchCount <= cfg.PatchWarnMax {
			return hook.HookResult{
				Action: hook.Warn,
				Reason: "HTML updated. Verify changes comply with grapesjs-html-compliance core rules " +
					"(no position:fixed, no external resource URLs, no bare <table> tags).",
			}
		}
		return hook.HookResult{Action: hook.Allow}
	}

	// Publish (write_file): three-tier degradation.
	if pubCount > cfg.PublishWarnMax {
		return hook.HookResult{Action: hook.Allow}
	}
	if pubCount > cfg.PublishFullChecklistMax {
		return hook.HookResult{
			Action: hook.Warn,
			Reason: "HTML rewritten. Ensure it complies with grapesjs-html-compliance rules.",
		}
	}
	// pubCount <= PublishFullChecklistMax: full checklist.
	return hook.HookResult{
		Action: hook.Warn,
		Reason: "[review] You just wrote an HTML file. Check it against " +
			"the grapesjs-html-compliance skill's selfchecklist (Part 3):" +
			"- [ ] Starts with <!DOCTYPE html>\n" +
			"- [ ] <html> and <body> tags present and complete\n" +
			"- [ ] All .slide have explicit width: 1920px; height: 1080px\n" +
			"- [ ] All .slide use overflow: hidden (not overflow-y: auto/scroll)\n" +
			"- [ ] All <script> inside <div data-gjs-type=\"custom-code\">\n" +
			"- [ ] First .slide element includes class=\"slide active\" (hardcoded, not JS)\n" +
			"- [ ] No bare <table> tags (use div grid for table layouts)\n" +
			"- [ ] Charts use div+CSS (layout) or inline SVG (complex geometry), SVG uses viewBox\n" +
			"- [ ] Viewport containers (#stage/#presentation/#scale-wrap) use position: relative\n" +
			"- [ ] No position: fixed\n" +
			"- [ ] No external resource URLs (no http/https in src/href; fonts via font-family only)\n" +
			"- [ ] No inline event handlers (onclick/onload/onerror etc.)\n" +
			"- [ ] No unclosed tags (div/section/span/p/a/ul/ol/li open/close count match)\n" +
			"- [ ] No unclosed CSS comments /* ... */\n" +
			"- [ ] Slide visibility styles in <head> <style>, not in custom-code blocks\n" +
			"Fix every violation with edit_file. Do NOT use write_file on an existing file.",
	}
}

// ---------------------------------------------------------------------------
// pre_context_assemble hooks
// ---------------------------------------------------------------------------

// QualityInject drains pending quality warnings at PreContextAssemble
// so they are injected into the LLM context for the next round.
func QualityInject(ctx context.Context, hctx *hook.HookContext) hook.HookResult {
	if hctx.SessionState == nil {
		return hook.HookResult{Action: hook.Allow}
	}
	warnings := hctx.SessionState.DrainPendingWarnings()
	if len(warnings) == 0 {
		return hook.HookResult{Action: hook.Allow}
	}
	var msgs []string
	for _, w := range warnings {
		if strings.TrimSpace(w) != "" {
			msgs = append(msgs, w)
		}
	}
	if len(msgs) == 0 {
		return hook.HookResult{Action: hook.Allow}
	}
	return hook.HookResult{
		Action: hook.Warn,
		Reason: strings.Join(msgs, "\n\n---\n\n"),
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func fileMtime(absPath string) int64 {
	info, err := os.Stat(absPath)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixNano()
}

func hashArgs(args map[string]any) string {
	if len(args) == 0 {
		return "0"
	}
	return hook.HashToolArgs(args)
}

// PostRoundLoopGuard detects read-only tool call loops after multiple consecutive
// rounds without HTML writes. Part of the loop_guard system.
func PostRoundLoopGuard(_ context.Context, hctx *hook.HookContext) hook.HookResult {
	if hctx.SessionState == nil {
		return hook.HookResult{Action: hook.Allow}
	}
	if hctx.SessionState.GetConsecutiveRoundsNoWrite() >= 3 &&
		hctx.SessionState.AllLastRoundReadOnly() {
		return hook.HookResult{
			Action: hook.Warn,
			Reason: "[loop-guard] No HTML file modifications in the last 3 rounds, and the last round contained only read-only tool calls. " +
				"If you are verifying HTML compliance, fix issues directly with edit_file " +
				"instead of repeatedly calling read_file/grep_search to confirm the same findings.",
		}
	}
	return hook.HookResult{Action: hook.Allow}
}

// ComplianceSeverity classifies a compliance finding.
type ComplianceSeverity string

const (
	SeverityMustFix    ComplianceSeverity = "must_fix"
	SeverityNeedReview ComplianceSeverity = "need_review"
)

