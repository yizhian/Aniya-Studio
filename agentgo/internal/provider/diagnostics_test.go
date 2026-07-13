package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListModels_EmptyAPIKey(t *testing.T) {
	_, err := ListModels(context.Background(), Config{APIKey: "", BaseURL: "http://localhost"})
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "api_key") {
		t.Errorf("expected api_key error, got: %v", err)
	}
}

func TestListModels_EmptyBaseURL(t *testing.T) {
	_, err := ListModels(context.Background(), Config{APIKey: "key", BaseURL: ""})
	if err == nil {
		t.Fatal("expected error for empty base URL")
	}
	if !strings.Contains(err.Error(), "base_url") {
		t.Errorf("expected base_url error, got: %v", err)
	}
}

func TestListModels_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "model-a"},
				{"id": "model-b"},
				{"id": ""},
			},
		})
	}))
	defer srv.Close()

	models, err := ListModels(context.Background(), Config{APIKey: "key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if models[0] != "model-a" || models[1] != "model-b" {
		t.Errorf("unexpected models: %v", models)
	}
}

func TestListModels_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := ListModels(context.Background(), Config{APIKey: "bad-key", BaseURL: srv.URL})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected authentication failed, got: %v", err)
	}
}

func TestListModels_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := ListModels(context.Background(), Config{APIKey: "key", BaseURL: srv.URL})
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("expected access denied, got: %v", err)
	}
}

func TestListModels_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	models, err := ListModels(context.Background(), Config{APIKey: "key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Fatalf("expected empty list for 404, got %d models", len(models))
	}
}

func TestListModels_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()

	_, err := ListModels(context.Background(), Config{APIKey: "key", BaseURL: srv.URL})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("expected HTTP 500 error, got: %v", err)
	}
}

func TestListModels_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	models, err := ListModels(context.Background(), Config{APIKey: "key", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Fatalf("expected empty list for invalid JSON, got %d models", len(models))
	}
}

func TestProbeModel_MissingAPIKey(t *testing.T) {
	result := ProbeModel(context.Background(), Config{})
	if result.OK {
		t.Fatal("expected OK=false for missing API key")
	}
	if !strings.Contains(result.Message, "API key") {
		t.Errorf("expected API key message, got: %s", result.Message)
	}
}

func TestProbeModel_MissingBaseURL(t *testing.T) {
	result := ProbeModel(context.Background(), Config{APIKey: "key"})
	if result.OK {
		t.Fatal("expected OK=false for missing base URL")
	}
	if !strings.Contains(result.Message, "Base URL") {
		t.Errorf("expected Base URL message, got: %s", result.Message)
	}
}

func TestProbeModel_MissingModel(t *testing.T) {
	result := ProbeModel(context.Background(), Config{APIKey: "key", BaseURL: "http://localhost"})
	if result.OK {
		t.Fatal("expected OK=false for missing model")
	}
	if !strings.Contains(result.Message, "Model name") {
		t.Errorf("expected Model name message, got: %s", result.Message)
	}
}

func TestProbeModel_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v1/models") {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "test-model"}},
			})
			return
		}
		// Chat completions endpoint — return 200 (success).
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	result := ProbeModel(context.Background(), Config{
		APIKey:  "key",
		BaseURL: srv.URL,
		Model:   "test-model",
		Type:    ProviderOpenAI,
	})
	if !result.OK {
		t.Fatalf("expected OK=true, got: %s", result.Message)
	}
	if !result.Verified {
		t.Error("expected Verified=true")
	}
	if !result.InList {
		t.Error("expected InList=true for model in list")
	}
}

func TestProbeModel_ProbeFailed(t *testing.T) {
	// Models list works, but the probe request fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v1/models") {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "test-model"}},
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	result := ProbeModel(context.Background(), Config{
		APIKey:  "bad-key",
		BaseURL: srv.URL,
		Model:   "test-model",
		Type:    ProviderOpenAI,
	})
	if result.OK {
		t.Fatal("expected OK=false for probe failure")
	}
	if !result.InList {
		t.Error("expected InList=true when model in list but probe fails")
	}
}

func TestProbeModel_ModelNotInList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v1/models") {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "other-model"}, {"id": "another-model"}},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	result := ProbeModel(context.Background(), Config{
		APIKey:  "key",
		BaseURL: srv.URL,
		Model:   "test-model",
		Type:    ProviderOpenAI,
	})
	if result.OK {
		t.Fatal("expected OK=false when model not in list and probe fails")
	}
	if result.InList {
		t.Error("expected InList=false for model not in list")
	}
}

func TestProbeModel_AnthropicProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v1/models") {
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]string{{"id": "claude-model"}},
			})
			return
		}
		// Anthropic messages endpoint returns 200.
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	result := ProbeModel(context.Background(), Config{
		APIKey:  "key",
		BaseURL: srv.URL,
		Model:   "claude-model",
		Type:    ProviderAnthropic,
	})
	if !result.OK {
		t.Fatalf("expected OK=true for Anthropic provider, got: %s", result.Message)
	}
	if !result.Verified {
		t.Error("expected Verified=true")
	}
}

func TestProbeModel_NoModelListAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v1/models") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	result := ProbeModel(context.Background(), Config{
		APIKey:  "key",
		BaseURL: srv.URL,
		Model:   "test-model",
		Type:    ProviderOpenAI,
	})
	if !result.OK {
		t.Fatalf("expected OK=true even without model list, got: %s", result.Message)
	}
	if result.InList {
		t.Error("expected InList=false when model list is unavailable")
	}
}

func newProbeErrorResponse(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestParseProbeError_Unauthorized(t *testing.T) {
	resp := newProbeErrorResponse(http.StatusUnauthorized, `{"error":"invalid key"}`)
	err := parseProbeError(resp)
	if err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected authentication failed error, got: %v", err)
	}
}

func TestParseProbeError_Forbidden(t *testing.T) {
	resp := newProbeErrorResponse(http.StatusForbidden, `{"error":"no access"}`)
	err := parseProbeError(resp)
	if err == nil || !strings.Contains(err.Error(), "access denied") {
		t.Errorf("expected access denied error, got: %v", err)
	}
}

func TestParseProbeError_NotFound(t *testing.T) {
	resp := newProbeErrorResponse(http.StatusNotFound, "")
	err := parseProbeError(resp)
	if err == nil || !strings.Contains(err.Error(), "endpoint not found") {
		t.Errorf("expected endpoint not found error, got: %v", err)
	}
}

func TestParseProbeError_OtherWithBody(t *testing.T) {
	resp := newProbeErrorResponse(http.StatusBadGateway, "upstream error")
	err := parseProbeError(resp)
	if err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Errorf("expected HTTP 502 error with body, got: %v", err)
	}
	if !strings.Contains(err.Error(), "upstream error") {
		t.Errorf("expected body text in error, got: %v", err)
	}
}

func TestParseProbeError_OtherNoBody(t *testing.T) {
	resp := newProbeErrorResponse(http.StatusInternalServerError, "")
	err := parseProbeError(resp)
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("expected HTTP 500 error, got: %v", err)
	}
}

func TestSetAuthHeaders_OpenAI(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://localhost", nil)
	setAuthHeaders(req, Config{APIKey: "sk-test", Type: ProviderOpenAI})
	if req.Header.Get("Authorization") != "Bearer sk-test" {
		t.Errorf("expected Bearer token, got: %s", req.Header.Get("Authorization"))
	}
	if req.Header.Get("x-api-key") != "" {
		t.Error("expected no x-api-key for OpenAI")
	}
}

func TestSetAuthHeaders_Anthropic(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://localhost", nil)
	setAuthHeaders(req, Config{APIKey: "ant-key", Type: ProviderAnthropic})
	if req.Header.Get("x-api-key") != "ant-key" {
		t.Errorf("expected x-api-key, got: %s", req.Header.Get("x-api-key"))
	}
	if req.Header.Get("anthropic-version") == "" {
		t.Error("expected anthropic-version header")
	}
	if req.Header.Get("Authorization") != "" {
		t.Error("expected no Authorization header for Anthropic")
	}
}
