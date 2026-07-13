package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentgo/internal/document"
	"agentgo/internal/persistence"
	p "agentgo/internal/provider"
	"agentgo/internal/toolkit/bootstrap"
	"agentgo/internal/toolkit/extended/skill"
	"agentgo/internal/toolkit/registry"
)

// newPrecipitateMux creates a test mux with precipitate and provider routes.
func newPrecipitateMux(srv *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", srv.handleChat)
	mux.HandleFunc("/skills/precipitate/stream", srv.handlePrecipitateStream)
	mux.HandleFunc("/skills/precipitate/confirm", srv.handlePrecipitateConfirm)
	mux.HandleFunc("/provider/config", srv.handleProviderConfig)
	mux.HandleFunc("/provider/test", srv.handleProviderTest)
	mux.HandleFunc("/provider/models", srv.handleProviderModels)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	return withCORS(mux)
}

// newPrecipitateServer creates a Server wired for precipitate testing.
func newPrecipitateServer(t *testing.T, mockURL string) (*Server, string) {
	t.Helper()

	workDir := t.TempDir()
	writeTestProjectManifest(t, workDir)

	// Change to project root for prompts/precipitate.md lookup.
	origCwd, _ := os.Getwd()
	root := findProjectRoot(t)
	os.Chdir(root)
	t.Cleanup(func() { os.Chdir(origCwd) })

	sessionStore := persistence.NewSessionStore(filepath.Join(workDir, "sessions"))
	memoryStore := persistence.NewFileMemoryStore()

	reg := registry.NewToolRegistry()
	if err := bootstrap.RegisterAllTools(reg, nil); err != nil {
		t.Fatalf("register tools: %v", err)
	}

	userSkillsDir := filepath.Join(workDir, "skills")
	os.MkdirAll(userSkillsDir, 0755)

	provider := p.NewOpenAIProvider(p.Config{
		APIKey:  "test-key",
		BaseURL: mockURL,
		Model:   "test-model",
		Type:    p.ProviderOpenAI,
	})

	srv := &Server{
		sessionStore:     sessionStore,
		memoryStore:      memoryStore,
		reg:              reg,
		toolExec:         newRegistryToolExecutor(reg, workDir),
		workspaceDir:     workDir,
		documentPipeline: document.NewPipeline(),
		userSkillsDir:    userSkillsDir,
		Provider:         provider,
	}

	return srv, workDir
}

func TestE2E_Precipitate_BasicGeneration(t *testing.T) {
	// Script: agent writes SKILL.md via write_file, then responds with text.
	script := p.MockSSEScript{
		Rounds: [][]p.MockSSEFrame{
			{
				p.TextFrame("Analyzing the design system..."),
				p.ToolStartFrame("write_file", "toolu_001", 0),
				p.ToolCompleteFrame("write_file", "toolu_001",
					`{"path":"SKILL.md","content":"---\nname: coral-deck\nscenario: marketing\n---\n# Coral Presentation Theme\n\nWarm coral colors with rounded corners."}`, 0),
				p.DoneFrame(),
			},
			{
				p.TextFrame("Design system extracted successfully."),
				p.DoneFrame(),
			},
		},
	}
	mockProvider := p.NewMockProviderServer(script)
	defer mockProvider.Close()

	srv, _ := newPrecipitateServer(t, mockProvider.URL)
	ts := httptest.NewServer(newPrecipitateMux(srv))
	defer ts.Close()

	htmlContent := `<html><head><title>Coral Theme</title></head><body>
		<section class="slide active"><h1>Welcome</h1></section>
	</body></html>`

	body, _ := json.Marshal(map[string]string{"html_content": htmlContent})
	resp, err := http.Post(ts.URL+"/skills/precipitate/stream", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}

	// Parse SSE events and look for precipitate_result.
	events := parseSSEStream(resp.Body)
	var result map[string]any
	for _, ev := range events {
		if ev.Type == "precipitate_result" {
			result = ev.Data
			break
		}
	}
	if result == nil {
		t.Fatal("expected precipitate_result SSE event")
	}

	if name, _ := result["suggested_name"].(string); name == "" {
		t.Error("expected non-empty suggested_name")
	}
	if md, _ := result["skill_md"].(string); md == "" {
		t.Error("expected non-empty skill_md")
	}
	if example, _ := result["example_html"].(string); example == "" {
		t.Error("expected non-empty example_html")
	}
}

func TestE2E_Precipitate_InvalidInput_EmptyHTML(t *testing.T) {
	mockProvider := p.NewMockProviderServer(p.MockSSEScript{})
	defer mockProvider.Close()

	srv, _ := newPrecipitateServer(t, mockProvider.URL)
	ts := httptest.NewServer(newPrecipitateMux(srv))
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"html_content": ""})
	resp, err := http.Post(ts.URL+"/skills/precipitate/stream", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for empty html_content, got %d", resp.StatusCode)
	}
}

func TestE2E_Precipitate_InvalidInput_OversizedHTML(t *testing.T) {
	mockProvider := p.NewMockProviderServer(p.MockSSEScript{})
	defer mockProvider.Close()

	srv, _ := newPrecipitateServer(t, mockProvider.URL)
	ts := httptest.NewServer(newPrecipitateMux(srv))
	defer ts.Close()

	// Build HTML content > 2MB limit.
	largeHTML := "<html><body>" + strings.Repeat("x", 2_000_100) + "</body></html>"
	body, _ := json.Marshal(map[string]string{"html_content": largeHTML})
	resp, err := http.Post(ts.URL+"/skills/precipitate/stream", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for oversized html_content, got %d", resp.StatusCode)
	}
}

func TestE2E_Precipitate_Confirm_InvalidSkillName(t *testing.T) {
	mockProvider := p.NewMockProviderServer(p.MockSSEScript{})
	defer mockProvider.Close()

	srv, _ := newPrecipitateServer(t, mockProvider.URL)
	ts := httptest.NewServer(newPrecipitateMux(srv))
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{
		"skill_name":   "this-name-is-way-too-long-" + strings.Repeat("x", 200),
		"skill_md":     "# Test",
		"example_html": "<html></html>",
	})
	resp, err := http.Post(ts.URL+"/skills/precipitate/confirm", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid skill_name, got %d", resp.StatusCode)
	}
}

func TestE2E_Precipitate_Confirm_Success(t *testing.T) {
	mockProvider := p.NewMockProviderServer(p.MockSSEScript{})
	defer mockProvider.Close()

	srv, workDir := newPrecipitateServer(t, mockProvider.URL)
	ts := httptest.NewServer(newPrecipitateMux(srv))
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{
		"skill_name":   "my-coral-deck",
		"scenario":     "marketing",
		"skill_md":     "---\nname: my-coral-deck\nscenario: marketing\n---\n# My Coral Deck",
		"example_html": "<html><body><section class=\"slide active\"><h1>Hello</h1></section></body></html>",
	})
	resp, err := http.Post(ts.URL+"/skills/precipitate/confirm", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result precipitateConfirmResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.SkillName != "my-coral-deck" {
		t.Errorf("expected my-coral-deck, got %q", result.SkillName)
	}
	if !result.Reloaded {
		t.Error("expected reloaded=true")
	}

	// Verify files exist on disk under userSkillsDir.
	skillDir := filepath.Join(workDir, "skills", "my-coral-deck")
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "example.html")); err != nil {
		t.Errorf("example.html not found: %v", err)
	}
}

func TestE2E_Precipitate_Confirm_AlreadyExists(t *testing.T) {
	mockProvider := p.NewMockProviderServer(p.MockSSEScript{})
	defer mockProvider.Close()

	srv, workDir := newPrecipitateServer(t, mockProvider.URL)
	ts := httptest.NewServer(newPrecipitateMux(srv))
	defer ts.Close()

	// Pre-create the skill and index it.
	skillDir := filepath.Join(workDir, "skills", "existing-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: existing-skill\nscenario: marketing\n---\n# Existing"), 0644)
	os.WriteFile(filepath.Join(skillDir, "example.html"), []byte("<html></html>"), 0644)

		srv.loadedSkills = skill.LoadSkills(workDir+"/skills", "")

	body, _ := json.Marshal(map[string]string{
		"skill_name":   "existing-skill",
		"skill_md":     "# Test",
		"example_html": "<html></html>",
	})
	resp, err := http.Post(ts.URL+"/skills/precipitate/confirm", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 for existing skill, got %d", resp.StatusCode)
	}
}

func TestE2E_Precipitate_MethodNotAllowed(t *testing.T) {
	mockProvider := p.NewMockProviderServer(p.MockSSEScript{})
	defer mockProvider.Close()

	srv, _ := newPrecipitateServer(t, mockProvider.URL)
	ts := httptest.NewServer(newPrecipitateMux(srv))
	defer ts.Close()

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/skills/precipitate/stream"},
		{"GET", "/skills/precipitate/confirm"},
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
				t.Errorf("expected 405, got %d", resp.StatusCode,)
			}
		})
	}
}
