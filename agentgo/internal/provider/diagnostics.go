package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProbeResult captures the outcome of a model connectivity probe.
type ProbeResult struct {
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
	InList   bool   `json:"in_list,omitempty"`
	Verified bool   `json:"verified"`
}

// ListModels fetches model IDs from the provider's /v1/models endpoint.
// Returns an empty slice when the endpoint is unavailable (not an error).
func ListModels(ctx context.Context, cfg Config) ([]string, error) {
	if cfg.APIKey == "" || cfg.BaseURL == "" {
		return nil, fmt.Errorf("api_key and base_url are required")
	}

	base := strings.TrimRight(cfg.BaseURL, "/")
	url := base + "/v1/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setAuthHeaders(req, cfg)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed — check your API key")
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("access denied — API key may lack permissions")
	}
	if resp.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("models endpoint HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return []string{}, nil
	}

	ids := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID != "" {
			ids = append(ids, item.ID)
		}
	}
	return ids, nil
}

// ProbeModel verifies API key, base URL, and model name with a minimal request.
func ProbeModel(ctx context.Context, cfg Config) ProbeResult {
	if cfg.APIKey == "" {
		return ProbeResult{OK: false, Message: "API key is required"}
	}
	if cfg.BaseURL == "" {
		return ProbeResult{OK: false, Message: "Base URL is required"}
	}
	if cfg.Model == "" {
		return ProbeResult{OK: false, Message: "Model name is required"}
	}

	inList := false
	models, err := ListModels(ctx, cfg)
	if err == nil && len(models) > 0 {
		for _, id := range models {
			if id == cfg.Model {
				inList = true
				break
			}
		}
	}

	probeErr := probeModelRequest(ctx, cfg)
	if probeErr == nil {
		msg := fmt.Sprintf("Model \"%s\" verified successfully", cfg.Model)
		if inList {
			msg += " (found in provider model list)"
		}
		return ProbeResult{OK: true, Message: msg, InList: inList, Verified: true}
	}

	if inList {
		return ProbeResult{
			OK:       false,
			Message:  fmt.Sprintf("Model \"%s\" is listed but probe failed: %v", cfg.Model, probeErr),
			InList:   true,
			Verified: false,
		}
	}

	if len(models) > 0 {
		return ProbeResult{
			OK:       false,
			Message:  fmt.Sprintf("Model \"%s\" not found in provider list (%d models). Probe error: %v", cfg.Model, len(models), probeErr),
			InList:   false,
			Verified: false,
		}
	}

	return ProbeResult{
		OK:       false,
		Message:  fmt.Sprintf("Model probe failed: %v", probeErr),
		Verified: false,
	}
}

func probeModelRequest(ctx context.Context, cfg Config) error {
	switch cfg.Type {
	case ProviderAnthropic:
		return probeAnthropic(ctx, cfg)
	default:
		return probeOpenAI(ctx, cfg)
	}
}

func probeOpenAI(ctx context.Context, cfg Config) error {
	base := strings.TrimRight(cfg.BaseURL, "/")
	body := map[string]any{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
		"max_tokens": 1,
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return parseProbeError(resp)
}

func probeAnthropic(ctx context.Context, cfg Config) error {
	base := strings.TrimRight(cfg.BaseURL, "/")
	body := map[string]any{
		"model":      cfg.Model,
		"max_tokens": 1,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
	}
	raw, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return parseProbeError(resp)
}

func parseProbeError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	text := strings.TrimSpace(string(body))

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("authentication failed — check your API key")
	case http.StatusForbidden:
		return fmt.Errorf("access denied")
	case http.StatusNotFound:
		return fmt.Errorf("endpoint not found — check your Base URL")
	default:
		if text != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, text)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
}

func setAuthHeaders(req *http.Request, cfg Config) {
	switch cfg.Type {
	case ProviderAnthropic:
		req.Header.Set("x-api-key", cfg.APIKey)
		req.Header.Set("anthropic-version", anthropicVersion)
	default:
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
}
