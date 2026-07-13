package provider

import (
	"testing"
)

func TestNew_OpenAIDefault(t *testing.T) {
	cfg := Config{
		APIKey:  "sk-test",
		BaseURL: "https://api.deepseek.com/v1",
		Model:   "deepseek-chat",
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("New() returned nil provider")
	}
	if p.Type() != ProviderOpenAI {
		t.Errorf("expected type openai, got %s", p.Type())
	}
}

func TestNew_OpenAIExplicit(t *testing.T) {
	cfg := Config{
		APIKey:  "sk-test",
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-4",
		Type:    ProviderOpenAI,
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if p.Type() != ProviderOpenAI {
		t.Errorf("expected type openai, got %s", p.Type())
	}
}

func TestNew_Anthropic(t *testing.T) {
	cfg := Config{
		APIKey:  "sk-test",
		BaseURL: "https://api.deepseek.com/anthropic/v1",
		Model:   "deepseek-chat",
		Type:    ProviderAnthropic,
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if p.Type() != ProviderAnthropic {
		t.Errorf("expected type anthropic, got %s", p.Type())
	}
}

func TestNew_UnknownTypeDefaultsToOpenAI(t *testing.T) {
	cfg := Config{
		APIKey:  "sk-test",
		BaseURL: "https://api.deepseek.com/v1",
		Model:   "deepseek-chat",
		Type:    ProviderType("unknown"),
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if p.Type() != ProviderOpenAI {
		t.Errorf("expected unknown type to default to openai, got %s", p.Type())
	}
}

func TestNew_EmptyTypeDefaultsToOpenAI(t *testing.T) {
	cfg := Config{
		APIKey:  "sk-test",
		BaseURL: "https://api.deepseek.com/v1",
		Model:   "deepseek-chat",
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}
	if p.Type() != ProviderOpenAI {
		t.Errorf("expected empty type to default to openai, got %s", p.Type())
	}
}

func TestProviderType_Constants(t *testing.T) {
	if ProviderOpenAI != "openai" {
		t.Errorf("expected 'openai', got %q", ProviderOpenAI)
	}
	if ProviderAnthropic != "anthropic" {
		t.Errorf("expected 'anthropic', got %q", ProviderAnthropic)
	}
}
