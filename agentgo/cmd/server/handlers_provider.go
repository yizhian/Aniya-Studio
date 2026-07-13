package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"agentgo/internal/config"
	"agentgo/internal/provider"
)

type providerConfigRequest struct {
	ProviderType string `json:"provider_type"`
	APIKey       string `json:"api_key"`
	BaseURL      string `json:"base_url"`
	Model        string `json:"model"`
}

func parseProviderType(raw string) provider.ProviderType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "anthropic":
		return provider.ProviderAnthropic
	default:
		return provider.ProviderOpenAI
	}
}

func requestToConfig(req providerConfigRequest) config.Config {
	return config.Config{
		APIKey:  strings.TrimSpace(req.APIKey),
		BaseURL: strings.TrimSpace(req.BaseURL),
		Model:   strings.TrimSpace(req.Model),
		Type:    parseProviderType(req.ProviderType),
	}
}

func (s *Server) handleProviderConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := s.getConfig()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"provider_type": string(cfg.Type),
			"base_url":      cfg.BaseURL,
			"model":         cfg.Model,
			"api_key_set":   cfg.APIKey != "",
		})
	case http.MethodPost:
		var req providerConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		cfg := requestToConfig(req)
		if err := cfg.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.updateConfig(cfg)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "message": "Provider config updated"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProviderTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req providerConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	cfg := requestToConfig(req)
	result := provider.ProbeModel(r.Context(), provider.Config{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		Type:    cfg.Type,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":       result.OK,
		"message":  result.Message,
		"in_list":  result.InList,
		"verified": result.Verified,
	})
}

func (s *Server) handleProviderModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req providerConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	cfg := requestToConfig(req)
	if cfg.APIKey == "" || cfg.BaseURL == "" {
		http.Error(w, "api_key and base_url are required", http.StatusBadRequest)
		return
	}

	models, err := provider.ListModels(r.Context(), provider.Config{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Type:    cfg.Type,
	})
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]any{
			"models":  []string{},
			"error":   err.Error(),
			"source":  "none",
		})
		return
	}

	source := "provider"
	if len(models) == 0 {
		source = "none"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"models": models,
		"source": source,
	})
}
