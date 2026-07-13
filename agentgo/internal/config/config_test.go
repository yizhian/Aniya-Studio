package config

import (
	"os"
	"path/filepath"
	"testing"

	"agentgo/internal/provider"
)

func TestLoad_AllEnvVars(t *testing.T) {
	// Save old env and restore after.
	oldKey := os.Getenv("DEEPSEEK_API_KEY")
	oldURL := os.Getenv("DEEPSEEK_BASE_URL")
	oldModel := os.Getenv("DEEPSEEK_MODEL")
	oldProvider := os.Getenv("PROVIDER_TYPE")
	defer func() {
		os.Setenv("DEEPSEEK_API_KEY", oldKey)
		os.Setenv("DEEPSEEK_BASE_URL", oldURL)
		os.Setenv("DEEPSEEK_MODEL", oldModel)
		os.Setenv("PROVIDER_TYPE", oldProvider)
	}()

	os.Setenv("DEEPSEEK_API_KEY", "sk-test-key")
	os.Setenv("DEEPSEEK_BASE_URL", "https://api.test.com")
	os.Setenv("DEEPSEEK_MODEL", "test-model")
	os.Setenv("PROVIDER_TYPE", "openai")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.APIKey != "sk-test-key" {
		t.Errorf("expected APIKey='sk-test-key', got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://api.test.com" {
		t.Errorf("expected BaseURL='https://api.test.com', got %q", cfg.BaseURL)
	}
	if cfg.Model != "test-model" {
		t.Errorf("expected Model='test-model', got %q", cfg.Model)
	}
	if cfg.Type != provider.ProviderOpenAI {
		t.Errorf("expected ProviderOpenAI, got %v", cfg.Type)
	}
}

func TestLoad_AnthropicProvider(t *testing.T) {
	oldKey := os.Getenv("DEEPSEEK_API_KEY")
	oldURL := os.Getenv("DEEPSEEK_BASE_URL")
	oldModel := os.Getenv("DEEPSEEK_MODEL")
	oldProvider := os.Getenv("PROVIDER_TYPE")
	defer func() {
		os.Setenv("DEEPSEEK_API_KEY", oldKey)
		os.Setenv("DEEPSEEK_BASE_URL", oldURL)
		os.Setenv("DEEPSEEK_MODEL", oldModel)
		os.Setenv("PROVIDER_TYPE", oldProvider)
	}()

	os.Setenv("DEEPSEEK_API_KEY", "sk-test")
	os.Setenv("DEEPSEEK_BASE_URL", "https://api.test.com")
	os.Setenv("DEEPSEEK_MODEL", "claude-test")
	os.Setenv("PROVIDER_TYPE", "anthropic")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Type != provider.ProviderAnthropic {
		t.Errorf("expected ProviderAnthropic, got %v", cfg.Type)
	}
}

func TestLoad_DefaultProviderType(t *testing.T) {
	oldKey := os.Getenv("DEEPSEEK_API_KEY")
	oldURL := os.Getenv("DEEPSEEK_BASE_URL")
	oldModel := os.Getenv("DEEPSEEK_MODEL")
	oldProvider := os.Getenv("PROVIDER_TYPE")
	defer func() {
		os.Setenv("DEEPSEEK_API_KEY", oldKey)
		os.Setenv("DEEPSEEK_BASE_URL", oldURL)
		os.Setenv("DEEPSEEK_MODEL", oldModel)
		os.Setenv("PROVIDER_TYPE", oldProvider)
	}()

	os.Setenv("DEEPSEEK_API_KEY", "sk-test")
	os.Setenv("DEEPSEEK_BASE_URL", "https://api.test.com")
	os.Setenv("DEEPSEEK_MODEL", "test-model")
	os.Unsetenv("PROVIDER_TYPE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Type != provider.ProviderOpenAI {
		t.Errorf("expected default ProviderOpenAI, got %v", cfg.Type)
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	oldKey := os.Getenv("DEEPSEEK_API_KEY")
	oldURL := os.Getenv("DEEPSEEK_BASE_URL")
	oldModel := os.Getenv("DEEPSEEK_MODEL")
	defer func() {
		os.Setenv("DEEPSEEK_API_KEY", oldKey)
		os.Setenv("DEEPSEEK_BASE_URL", oldURL)
		os.Setenv("DEEPSEEK_MODEL", oldModel)
	}()

	os.Unsetenv("DEEPSEEK_API_KEY")
	os.Setenv("DEEPSEEK_BASE_URL", "https://api.test.com")
	os.Setenv("DEEPSEEK_MODEL", "test-model")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestLoad_MissingBaseURL(t *testing.T) {
	oldKey := os.Getenv("DEEPSEEK_API_KEY")
	oldURL := os.Getenv("DEEPSEEK_BASE_URL")
	oldModel := os.Getenv("DEEPSEEK_MODEL")
	defer func() {
		os.Setenv("DEEPSEEK_API_KEY", oldKey)
		os.Setenv("DEEPSEEK_BASE_URL", oldURL)
		os.Setenv("DEEPSEEK_MODEL", oldModel)
	}()

	os.Setenv("DEEPSEEK_API_KEY", "sk-test")
	os.Unsetenv("DEEPSEEK_BASE_URL")
	os.Setenv("DEEPSEEK_MODEL", "test-model")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing base URL")
	}
}

func TestLoad_MissingModel(t *testing.T) {
	oldKey := os.Getenv("DEEPSEEK_API_KEY")
	oldURL := os.Getenv("DEEPSEEK_BASE_URL")
	oldModel := os.Getenv("DEEPSEEK_MODEL")
	defer func() {
		os.Setenv("DEEPSEEK_API_KEY", oldKey)
		os.Setenv("DEEPSEEK_BASE_URL", oldURL)
		os.Setenv("DEEPSEEK_MODEL", oldModel)
	}()

	os.Setenv("DEEPSEEK_API_KEY", "sk-test")
	os.Setenv("DEEPSEEK_BASE_URL", "https://api.test.com")
	os.Unsetenv("DEEPSEEK_MODEL")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestLoad_WhitespaceTrimming(t *testing.T) {
	oldKey := os.Getenv("DEEPSEEK_API_KEY")
	oldURL := os.Getenv("DEEPSEEK_BASE_URL")
	oldModel := os.Getenv("DEEPSEEK_MODEL")
	defer func() {
		os.Setenv("DEEPSEEK_API_KEY", oldKey)
		os.Setenv("DEEPSEEK_BASE_URL", oldURL)
		os.Setenv("DEEPSEEK_MODEL", oldModel)
	}()

	os.Setenv("DEEPSEEK_API_KEY", "  sk-test-key  ")
	os.Setenv("DEEPSEEK_BASE_URL", "  https://api.test.com  ")
	os.Setenv("DEEPSEEK_MODEL", "  test-model  ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.APIKey != "sk-test-key" {
		t.Errorf("expected trimmed APIKey, got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://api.test.com" {
		t.Errorf("expected trimmed BaseURL, got %q", cfg.BaseURL)
	}
}

func TestLoad_DotEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "DEEPSEEK_API_KEY=sk-from-file\nDEEPSEEK_BASE_URL=https://file.api.com\nDEEPSEEK_MODEL=file-model\nPROVIDER_TYPE=anthropic\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Save and clear env vars so .env is the source.
	oldKey := os.Getenv("DEEPSEEK_API_KEY")
	oldURL := os.Getenv("DEEPSEEK_BASE_URL")
	oldModel := os.Getenv("DEEPSEEK_MODEL")
	oldProvider := os.Getenv("PROVIDER_TYPE")
	defer func() {
		os.Setenv("DEEPSEEK_API_KEY", oldKey)
		os.Setenv("DEEPSEEK_BASE_URL", oldURL)
		os.Setenv("DEEPSEEK_MODEL", oldModel)
		os.Setenv("PROVIDER_TYPE", oldProvider)
	}()
	os.Unsetenv("DEEPSEEK_API_KEY")
	os.Unsetenv("DEEPSEEK_BASE_URL")
	os.Unsetenv("DEEPSEEK_MODEL")
	os.Unsetenv("PROVIDER_TYPE")

	// Change to temp dir so .env is found.
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load from .env failed: %v", err)
	}
	if cfg.APIKey != "sk-from-file" {
		t.Errorf("expected APIKey='sk-from-file', got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://file.api.com" {
		t.Errorf("expected BaseURL='https://file.api.com', got %q", cfg.BaseURL)
	}
	if cfg.Type != provider.ProviderAnthropic {
		t.Errorf("expected ProviderAnthropic, got %v", cfg.Type)
	}
}

func TestNewProvider(t *testing.T) {
	cfg := Config{
		APIKey:  "sk-test",
		BaseURL: "https://api.test.com",
		Model:   "test-model",
		Type:    provider.ProviderOpenAI,
	}
	p := cfg.NewProvider()
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.Type() != provider.ProviderOpenAI {
		t.Errorf("expected ProviderOpenAI, got %v", p.Type())
	}

	// Test Anthropic.
	cfg2 := Config{
		APIKey:  "sk-test",
		BaseURL: "https://api.test.com",
		Model:   "claude-test",
		Type:    provider.ProviderAnthropic,
	}
	p2 := cfg2.NewProvider()
	if p2 == nil {
		t.Fatal("expected non-nil anthropic provider")
	}
	if p2.Type() != provider.ProviderAnthropic {
		t.Errorf("expected ProviderAnthropic, got %v", p2.Type())
	}
}

func TestValidate_AllMissing(t *testing.T) {
	cfg := Config{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := Config{
		APIKey:  "sk-test",
		BaseURL: "https://api.test.com",
		Model:   "test-model",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"missing key and url", Config{Model: "m"}},
		{"missing url and model", Config{APIKey: "k"}},
		{"missing key", Config{BaseURL: "u", Model: "m"}},
		{"missing url", Config{APIKey: "k", Model: "m"}},
		{"missing model", Config{APIKey: "k", BaseURL: "u"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Error("expected error")
			}
		})
	}
}
