package hook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_FileNotExists(t *testing.T) {
	cfg, err := LoadConfig("/tmp/nonexistent_path_xyz_hooks.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil default config")
	}
	if cfg.Version != "1" {
		t.Errorf("expected version '1', got %q", cfg.Version)
	}
}

func TestLoadConfig_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.yaml")
	yamlContent := `
version: "2"
settings:
  max_consecutive_failures: 5
  allowed_extensions:
    - ".html"
    - ".css"
  protected_paths:
    - "/etc/**"
hooks:
  - name: test-hook
    enabled: false
    config:
      key: value
`
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Version != "2" {
		t.Errorf("expected version '2', got %q", cfg.Version)
	}
	if cfg.Settings.MaxConsecutiveFailures != 5 {
		t.Errorf("expected MaxConsecutiveFailures 5, got %d", cfg.Settings.MaxConsecutiveFailures)
	}
	if len(cfg.Settings.AllowedExtensions) != 2 {
		t.Errorf("expected 2 allowed extensions, got %d", len(cfg.Settings.AllowedExtensions))
	}
	if len(cfg.Hooks) != 1 {
		t.Fatalf("expected 1 hook override, got %d", len(cfg.Hooks))
	}
	if cfg.Hooks[0].Name != "test-hook" {
		t.Errorf("expected hook name 'test-hook', got %q", cfg.Hooks[0].Name)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte(": invalid yaml {{{"), 0644); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadConfig_RealFile(t *testing.T) {
	// Load the actual default hooks.yaml shipped with the project.
	path := "../../.agentgo/hooks.yaml"
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("failed to load real hooks.yaml: %v", err)
	}
	if cfg.Version != "1" {
		t.Errorf("expected version '1', got %q", cfg.Version)
	}

	s := cfg.Settings

	// Thresholds
	if s.MaxConsecutiveFailures != 3 {
		t.Errorf("expected MaxConsecutiveFailures 3, got %d", s.MaxConsecutiveFailures)
	}
	if s.DuplicateCallWarnCount != 3 {
		t.Errorf("expected DuplicateCallWarnCount 3, got %d", s.DuplicateCallWarnCount)
	}

	// Skills
	if len(s.RequiredSkills) == 0 || s.RequiredSkills[0] != "grapesjs-html-compliance" {
		t.Errorf("expected RequiredSkills[0]='grapesjs-html-compliance', got %v", s.RequiredSkills)
	}

	// Messages
	if s.Messages.FileTypeBlock == "" {
		t.Error("expected FileTypeBlock message")
	}
	if s.Messages.ReadProofBlock == "" {
		t.Error("expected ReadProofBlock message")
	}
	if s.Messages.ConsecutiveFailWarn == "" {
		t.Error("expected ConsecutiveFailWarn message")
	}
	if s.Messages.WorkspaceBlock == "" {
		t.Error("expected WorkspaceBlock message")
	}

	// Allowed extensions
	if len(s.AllowedExtensions) < 10 {
		t.Errorf("expected at least 10 allowed extensions, got %d", len(s.AllowedExtensions))
	}
}

func TestDefaultConfig_HasAllMessages(t *testing.T) {
	cfg := DefaultConfig()
	msgs := cfg.Settings.Messages

	if msgs.FileTypeBlock == "" || msgs.NoExtensionWarn == "" ||
		msgs.ReadProofBlock == "" || msgs.StaleReadWarn == "" ||
		msgs.ConsecutiveFailWarn == "" || msgs.DuplicateCallWarn == "" ||
		msgs.WorkspaceBlock == "" {
		t.Error("DefaultConfig has empty message fields")
	}
}

func TestDefaultConfig_HasAllThresholds(t *testing.T) {
	cfg := DefaultConfig()
	s := cfg.Settings

	if s.MaxConsecutiveFailures <= 0 {
		t.Error("MaxConsecutiveFailures must be > 0")
	}
	if s.DuplicateCallWarnCount <= 0 {
		t.Error("DuplicateCallWarnCount must be > 0")
	}
}

func TestLoadConfig_EnabledOverrideParsed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.yaml")
	yamlContent := `
version: "1"
settings: {}
hooks:
  - name: my-hook
    enabled: false
`
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write yaml: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.Hooks[0].Enabled == nil || *cfg.Hooks[0].Enabled != false {
		t.Error("expected enabled=false override")
	}
}
