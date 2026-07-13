package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agentgo/internal/document"
	"agentgo/internal/persistence"
	"agentgo/internal/toolkit/registry"
)

// newProviderMux creates a test mux with provider management routes.
func newProviderMux(srv *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/provider/config", srv.handleProviderConfig)
	mux.HandleFunc("/provider/test", srv.handleProviderTest)
	mux.HandleFunc("/provider/models", srv.handleProviderModels)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	return withCORS(mux)
}

// newProviderServer creates a Server for provider management tests.
func newProviderServer(t *testing.T) *Server {
	t.Helper()

	dir := t.TempDir()
	writeTestProjectManifest(t, dir)

	sessionStore := persistence.NewSessionStore(dir)
	memoryStore := persistence.NewFileMemoryStore()
	reg := registry.NewToolRegistry()

	return &Server{
		sessionStore:     sessionStore,
		memoryStore:      memoryStore,
		reg:              reg,
		toolExec:         newRegistryToolExecutor(reg, ""),
		workspaceDir:     dir,
		documentPipeline: document.NewPipeline(),
	}
}

func TestSmoke_Provider_GetConfig_Defaults(t *testing.T) {
	srv := newProviderServer(t)
	ts := httptest.NewServer(newProviderMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/provider/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	// Default provider type — empty before any config is set.
	if _, ok := body["provider_type"]; !ok {
		t.Error("expected provider_type key in response")
	}
	// api_key_set should be false by default.
	if aks, _ := body["api_key_set"].(bool); aks {
		t.Error("expected api_key_set=false by default")
	}
}

func TestSmoke_Provider_PostConfig_Success(t *testing.T) {
	srv := newProviderServer(t)
	ts := httptest.NewServer(newProviderMux(srv))
	defer ts.Close()

	reqBody, _ := json.Marshal(map[string]string{
		"provider_type": "openai",
		"api_key":       "sk-test-key",
		"base_url":      "https://api.openai.com/v1",
		"model":         "gpt-4",
	})
	resp, err := http.Post(ts.URL+"/provider/config", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if ok, _ := body["ok"].(bool); !ok {
		t.Error("expected ok=true")
	}

	// Verify config was updated via GET.
	resp2, err := http.Get(ts.URL + "/provider/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	var getBody map[string]any
	json.NewDecoder(resp2.Body).Decode(&getBody)
	if aks, _ := getBody["api_key_set"].(bool); !aks {
		t.Error("expected api_key_set=true after setting key")
	}
	if model, _ := getBody["model"].(string); model != "gpt-4" {
		t.Errorf("expected model=gpt-4, got %q", model)
	}
}

func TestSmoke_Provider_PostConfig_InvalidJSON(t *testing.T) {
	srv := newProviderServer(t)
	ts := httptest.NewServer(newProviderMux(srv))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/provider/config", "application/json", bytes.NewReader([]byte(`{bad}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSmoke_Provider_PostConfig_InvalidType(t *testing.T) {
	srv := newProviderServer(t)
	ts := httptest.NewServer(newProviderMux(srv))
	defer ts.Close()

	reqBody, _ := json.Marshal(map[string]string{
		"provider_type": "anthropic",
		"api_key":       "sk-ant-key",
		"base_url":      "https://api.anthropic.com",
		"model":         "claude-3",
	})
	resp, err := http.Post(ts.URL+"/provider/config", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify provider_type was parsed correctly.
	resp2, _ := http.Get(ts.URL + "/provider/config")
	defer resp2.Body.Close()
	var getBody map[string]any
	json.NewDecoder(resp2.Body).Decode(&getBody)
	if pt, _ := getBody["provider_type"].(string); pt != "anthropic" {
		t.Errorf("expected provider_type=anthropic, got %q", pt)
	}
}

func TestSmoke_Provider_Models_MissingFields(t *testing.T) {
	srv := newProviderServer(t)
	ts := httptest.NewServer(newProviderMux(srv))
	defer ts.Close()

	// Empty request — no api_key or base_url.
	resp, err := http.Post(ts.URL+"/provider/models", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing api_key/base_url, got %d", resp.StatusCode)
	}
}

func TestSmoke_Provider_MethodNotAllowed(t *testing.T) {
	srv := newProviderServer(t)
	ts := httptest.NewServer(newProviderMux(srv))
	defer ts.Close()

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/provider/test"},
		{"GET", "/provider/models"},
		{"PUT", "/provider/config"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, ts.URL+tt.path, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("expected 405, got %d", resp.StatusCode)
			}
		})
	}
}

func TestSmoke_Provider_Test_InvalidJSON(t *testing.T) {
	srv := newProviderServer(t)
	ts := httptest.NewServer(newProviderMux(srv))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/provider/test", "application/json", bytes.NewReader([]byte(`{bad}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSmoke_Provider_Models_InvalidJSON(t *testing.T) {
	srv := newProviderServer(t)
	ts := httptest.NewServer(newProviderMux(srv))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/provider/models", "application/json", bytes.NewReader([]byte(`{bad}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSmoke_Provider_Config_AnthropicType(t *testing.T) {
	srv := newProviderServer(t)
	ts := httptest.NewServer(newProviderMux(srv))
	defer ts.Close()

	reqBody, _ := json.Marshal(map[string]string{
		"provider_type": "Anthropic",
		"api_key":       "sk-ant-key",
		"base_url":      "https://api.anthropic.com",
		"model":         "claude-sonnet-4-6",
	})
	resp, err := http.Post(ts.URL+"/provider/config", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify via GET.
	resp2, _ := http.Get(ts.URL + "/provider/config")
	defer resp2.Body.Close()
	var getBody map[string]any
	json.NewDecoder(resp2.Body).Decode(&getBody)
	if pt, _ := getBody["provider_type"].(string); pt != "anthropic" {
		t.Errorf("expected provider_type=anthropic (case-insensitive), got %q", pt)
	}
	if model, _ := getBody["model"].(string); model != "claude-sonnet-4-6" {
		t.Errorf("expected model=claude-sonnet-4-6, got %q", model)
	}
}

func TestSmoke_Provider_Test_WithConfig(t *testing.T) {
	// Set up a mock HTTP server that the probe will connect to.
	mockAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/models") || r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `{"data":[{"id":"gpt-4"},{"id":"gpt-3.5-turbo"}]}`)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockAPI.Close()

	srv := newProviderServer(t)
	ts := httptest.NewServer(newProviderMux(srv))
	defer ts.Close()

	// First configure the provider to point at our mock.
	reqBody, _ := json.Marshal(map[string]string{
		"provider_type": "openai",
		"api_key":       "sk-test",
		"base_url":      mockAPI.URL,
		"model":         "gpt-4",
	})
	http.Post(ts.URL+"/provider/config", "application/json", bytes.NewReader(reqBody))

	// Now call /provider/test.
	testBody, _ := json.Marshal(map[string]string{
		"provider_type": "openai",
		"api_key":       "sk-test",
		"base_url":      mockAPI.URL,
		"model":         "gpt-4",
	})
	resp, err := http.Post(ts.URL+"/provider/test", "application/json", bytes.NewReader(testBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if ok, _ := result["ok"]; ok == nil {
		t.Error("expected 'ok' field in response")
	}
}
