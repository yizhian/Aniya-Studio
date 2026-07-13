package hook

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds persistent hook configuration loaded from .agentgo/hooks.yaml.
type Config struct {
	Version  string          `yaml:"version"`
	Settings Settings        `yaml:"settings"`
	Hooks    []HookOverrides `yaml:"hooks"`
}

// Settings holds tunable parameters for built-in hooks.
// Every threshold and strategy value is externalized here — nothing is hardcoded.
type Settings struct {
	// --- File type control ---
	AllowedExtensions []string `yaml:"allowed_extensions"`
	ProtectedPaths    []string `yaml:"protected_paths"`

	// --- Thresholds ---
	MaxConsecutiveFailures int `yaml:"max_consecutive_failures"`
	DuplicateCallWarnCount int `yaml:"duplicate_call_warn_count"`

	// --- Skill requirements ---
	RequiredSkills []string `yaml:"required_skills"`

	// --- Compliance ---
	Compliance ComplianceSettings `yaml:"compliance"`

	// --- User-facing messages ---
	Messages HookMessages `yaml:"messages"`
}

// ComplianceSettings holds tuning parameters for ComplianceReviewTrigger.
type ComplianceSettings struct {
	// PublishFullChecklistMax is the max Publish count that still gets the full
	// 15-item checklist (aligned to SKILL.md Part 3). Default 1.
	PublishFullChecklistMax int `yaml:"publish_full_checklist_max"`
	// PublishWarnMax is the max Publish count that still gets a short reminder.
	// Above this, Publish is silent. Default 3.
	PublishWarnMax int `yaml:"publish_warn_max"`
	// PatchWarnMax is the max Patch count that still gets a short reminder.
	// Above this, Patch is silent. Default 1.
	PatchWarnMax int `yaml:"patch_warn_max"`
	// EnableCSSOnPatch enables htmlchecker S1-S10 validation on Patch paths
	// (reads post-edit file from disk and runs full validateHTMLCompliance).
	// Default true.
	EnableCSSOnPatch bool `yaml:"enable_css_on_patch"`
}

// HookMessages holds all user-facing warning and block messages for built-in hooks.
type HookMessages struct {
	// pre_tool_use: file-type-whitelist
	FileTypeBlock   string `yaml:"file_type_block"`
	NoExtensionWarn string `yaml:"no_extension_warn"`

	// pre_tool_use: read-proof-pre-check
	ReadProofBlock string `yaml:"read_proof_block"`
	StaleReadWarn  string `yaml:"stale_read_warn"`

	// post_tool_use: consecutive-failure-detector
	ConsecutiveFailWarn string `yaml:"consecutive_fail_warn"`

	// post_tool_use: duplicate-call-detector
	DuplicateCallWarn string `yaml:"duplicate_call_warn"`

	// user_prompt_submit: init-check
	WorkspaceBlock string `yaml:"workspace_block"`
}

// HookOverrides allows per-project configuration of built-in hooks.
type HookOverrides struct {
	Name    string         `yaml:"name"`
	Enabled *bool          `yaml:"enabled"`
	Config  map[string]any `yaml:"config"`
}

// DefaultConfig returns a sensible default configuration with all strategy values.
func DefaultConfig() *Config {
	return &Config{
		Version: "1",
		Settings: Settings{
			MaxConsecutiveFailures: 3,
			DuplicateCallWarnCount: 3,
			AllowedExtensions: []string{
				".html", ".css", ".js", ".mjs", ".json", ".md", ".svg",
				".png", ".jpg", ".jpeg", ".webp", ".gif",
				".woff2", ".ttf", ".yaml", ".yml", ".toml", ".txt",
			},
			ProtectedPaths: []string{
				".agentgo/sessions/**",
				".agentgo/logs/**",
				".slidecraft/**",
			},
			RequiredSkills: []string{"grapesjs-html-compliance"},
			Compliance: ComplianceSettings{
				PublishFullChecklistMax: 2,
				PublishWarnMax:          5,
				PatchWarnMax:            3,
				EnableCSSOnPatch:        true,
			},
			Messages: HookMessages{
				FileTypeBlock:       "Cannot create %s files. Allowed types: %s",
				NoExtensionWarn:     "File has no extension. Verify the file type.",
				ReadProofBlock:      `You must call read_file on %q before editing it. This ensures you have the latest content and provides mtime proof.`,
				StaleReadWarn:       `File %q may have been modified since your last read. Re-read with read_file to get a fresh mtime.`,
				ConsecutiveFailWarn: `Tool %q has failed %d times in a row. Try a different approach or check input parameters.`,
				DuplicateCallWarn:   "You appear to be repeating the same failing tool call with identical arguments. Try a different approach.",
				WorkspaceBlock:      "Workspace directory does not exist: %s",
			},
		},
	}
}

// LoadConfig reads and parses a hooks.yaml configuration file.
// If the file does not exist, it returns DefaultConfig() with no error.
// Enforces the invariant PublishFullChecklistMax <= PublishWarnMax via clamping.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read hooks config %q: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse hooks config %q: %w", path, err)
	}

	// Enforce invariant: PublishFullChecklistMax <= PublishWarnMax.
	if cfg.Settings.Compliance.PublishFullChecklistMax > cfg.Settings.Compliance.PublishWarnMax {
		cfg.Settings.Compliance.PublishFullChecklistMax = cfg.Settings.Compliance.PublishWarnMax
	}

	return cfg, nil
}