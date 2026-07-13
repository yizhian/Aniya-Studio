package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"agentgo/internal/provider"
)

// Guard constants — single source of truth for magic numbers used across the agent.
const (
	DefaultMaxRounds       = 100
	DefaultMaxTokens       = 8192
	SSEScanBufferSize      = 524288  // bytes
	MaxParseErrorsPerStream = 10
	DefaultRetryCount      = 2
)

// Config holds all model connection settings sourced from environment variables.
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Type    provider.ProviderType
}

// Load reads required settings from .env/environment in strict mode (no hardcoded defaults).
func Load() (Config, error) {
	// Best-effort: load .env in current working directory.
	// Keeps existing exported env vars as priority.
	if err := loadDotEnvFromFile(".env"); err != nil {
		log.Printf("warning: failed to load .env: %v", err)
	}
	var providerType provider.ProviderType
	switch strings.ToLower(strings.TrimSpace(os.Getenv("PROVIDER_TYPE"))) {
	case "anthropic":
		providerType = provider.ProviderAnthropic
	default:
		providerType = provider.ProviderOpenAI
	}

	cfg := Config{
		APIKey:  strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")),
		BaseURL: strings.TrimSpace(os.Getenv("DEEPSEEK_BASE_URL")),
		Model:   strings.TrimSpace(os.Getenv("DEEPSEEK_MODEL")),
		Type:    providerType,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// NewProvider creates a StreamingProvider from this config.
func (c Config) NewProvider() provider.StreamingProvider {
	p, err := provider.New(provider.Config{
		APIKey:  c.APIKey,
		BaseURL: c.BaseURL,
		Model:   c.Model,
		Type:    c.Type,
	})
	if err != nil {
		return nil
	}
	return p
}

// Validate enforces strict required config (no hardcoded fallback).
func (c Config) Validate() error {
	missing := make([]string, 0, 3)
	if c.APIKey == "" {
		missing = append(missing, "DEEPSEEK_API_KEY")
	}
	if c.BaseURL == "" {
		missing = append(missing, "DEEPSEEK_BASE_URL")
	}
	if c.Model == "" {
		missing = append(missing, "DEEPSEEK_MODEL")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s (set in .env or exported env vars)", strings.Join(missing, ", "))
	}
	return nil
}
