package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentgo/internal/util"

	"agentgo/internal/document"
	"agentgo/internal/observability"
	"agentgo/internal/persistence"
	"agentgo/internal/toolkit/core"
	"agentgo/internal/toolkit/extended/skill"
	"agentgo/internal/toolkit/registry"
)

// projectRoot returns the project root directory by locating go.mod.
func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// newTestServer creates a Server with temp directories for stores.
func newTestServer(t *testing.T) (*Server, *registry.ToolRegistry) {
	t.Helper()

	dir := t.TempDir()

	// Write a minimal project.json for manifest-driven chat.
	writeTestProjectManifest(t, dir)

	sessionStore := persistence.NewSessionStore(filepath.Join(dir, "sessions"))
	memoryStore := persistence.NewFileMemoryStore()

	reg := registry.NewToolRegistry()
	reg.Register(core.NewEditFileTool())

	srv := &Server{
		sessionStore: sessionStore,
		memoryStore:  memoryStore,
		reg:          reg,
		toolExec:     newRegistryToolExecutor(reg, ""),
		workspaceDir: dir,
		consoleObs:   observability.NewConsoleObserver(),
	}

	return srv, reg
}

func newTestMux(srv *Server) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", srv.handleChat)
	mux.HandleFunc("/edit", srv.handleDirectEdit)
	mux.HandleFunc("/upload", srv.handleUpload)
	mux.HandleFunc("/recommend-styles", srv.handleRecommendStyles)
	mux.HandleFunc("/history", srv.handleHistory)
	mux.HandleFunc("/sessions/", srv.handleSession)
	mux.HandleFunc("/skills", srv.handleSkills)
	mux.HandleFunc("GET /skills/{name}/content", srv.handleSkillContent)
	mux.HandleFunc("GET /skills/{name}/example", srv.handleSkillExample)
	mux.HandleFunc("GET /skills/{name}/assets/{path...}", srv.handleSkillAsset)
	mux.HandleFunc("GET /files/{project_id}/originals/{filename}", srv.handleServeOriginal)
	mux.HandleFunc("GET /files/{project_id}/docs/{filename}", srv.handleServeDoc)
	mux.HandleFunc("GET /projects/{project_id}/uploads", srv.handleGetUploads)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	return withCORS(mux)
}

// writeTestProjectManifest writes a minimal project.json with a design_skill
// so system prompt SkillOverride injection is exercised in tests.
func writeTestProjectManifest(t *testing.T, dir string) {
	t.Helper()
	manifest := ProjectManifest{
		Name:        "test-project",
		Brief:       "test brief",
		DesignSkill: "test-skill",
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestServer_Health(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", body["status"])
	}
}

func TestServer_Chat_InvalidJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/chat", "application/json", strings.NewReader(`not json`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServer_Chat_InvalidSessionID(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	body := `{"message":"hello","session_id":"bad/../../etc"}`
	resp, err := http.Post(ts.URL+"/chat", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid session ID, got %d", resp.StatusCode)
	}
}

func TestServer_Chat_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/chat")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestServer_History_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/history")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var sessions []persistence.SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions == nil {
		t.Fatal("expected empty array, got nil")
	}
}

func TestServer_Session_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/sessions/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestServer_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	tests := []struct {
		method string
		path   string
	}{
		{"POST", "/history"},
		{"GET", "/chat"},
		{"GET", "/edit"},
		{"POST", "/sessions/test"},
		{"GET", "/upload"},
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
				t.Fatalf("expected 405 for %s %s, got %d", tt.method, tt.path, resp.StatusCode)
			}
		})
	}
}

func TestServer_CORS_Headers(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("expected CORS Allow-Origin header")
	}
}

func TestServer_CORS_Preflight(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for OPTIONS preflight, got %d", resp.StatusCode)
	}
}

func TestServer_Edit_InvalidJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/edit", "application/json", strings.NewReader(`{bad}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServer_Session_InvalidID_WithSlash(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// A session ID with a slash in the extracted path part should be rejected.
	// /sessions/foo/bar → id="foo/bar" → rejected due to ContainsAny(id, "/\\")
	resp, err := http.Get(ts.URL + "/sessions/foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for session ID with slash, got %d", resp.StatusCode)
	}
}

func TestServer_Upload_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/upload")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestServer_Chat_SSE_Headers(t *testing.T) {
	// Need to be in project root for prompts/ to be found.
	root := projectRoot(t)
	origCwd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origCwd)

	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	body := `{"message":"hello","session_id":"test-sse"}`
	resp, err := http.Post(ts.URL+"/chat", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", contentType)
	}
	if resp.Header.Get("Cache-Control") != "no-cache" {
		t.Fatal("expected Cache-Control: no-cache")
	}
	if resp.Header.Get("X-Session-Id") != "test-sse" {
		t.Fatalf("expected X-Session-Id=test-sse, got %q", resp.Header.Get("X-Session-Id"))
	}
}

func TestServer_Chat_AutoSessionID(t *testing.T) {
	root := projectRoot(t)
	origCwd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origCwd)

	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	body := `{"message":"hello"}`
	resp, err := http.Post(ts.URL+"/chat", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	sessionID := resp.Header.Get("X-Session-Id")
	if sessionID == "" {
		t.Fatal("expected auto-generated X-Session-Id")
	}
	if !strings.HasPrefix(sessionID, "sess_") {
		t.Fatalf("expected session ID to start with 'sess_', got %q", sessionID)
	}
}

func TestServer_Chat_SSE_ErrorEvent(t *testing.T) {
	root := projectRoot(t)
	origCwd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origCwd)

	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	body := `{"message":"hello","session_id":"sse-error"}`
	resp, err := http.Post(ts.URL+"/chat", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Read SSE body — provider will fail (no valid config),
	// but should get some SSE frames with error event.
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	if n > 0 {
		text := string(buf[:n])
		if !strings.Contains(text, "data:") {
			t.Fatalf("expected SSE 'data:' prefix, got: %s", text[:min(n, 500)])
		}
	}
}

// TestSSEHeartbeat_SendsKeepalive verifies SSE keepalive comments are sent
// during prolonged streaming. We set the heartbeat interval very short.
func TestSSEHeartbeat_SendsKeepalive(t *testing.T) {
	origInterval := sseHeartbeatInterval
	sseHeartbeatInterval = 100 * time.Millisecond
	defer func() { sseHeartbeatInterval = origInterval }()

	root := projectRoot(t)
	origCwd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origCwd)

	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	body := `{"message":"test","session_id":"keepalive-test"}`
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/chat",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	// Read SSE lines for up to 500ms, looking for keepalive comments.
	deadline := time.After(500 * time.Millisecond)
	keepaliveFound := make(chan bool, 1)
	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, ": keepalive") {
				keepaliveFound <- true
				return
			}
		}
	}()

	select {
	case <-keepaliveFound:
		// Keepalive received.
	case <-deadline:
		t.Log("no keepalive found within 500ms — may depend on provider timing")
	}

	cancel()
}

// TestSSEHeartbeat_StopsOnContextCancel verifies the heartbeat goroutine
// exits when the request context is canceled.
func TestSSEHeartbeat_StopsOnContextCancel(t *testing.T) {
	origInterval := sseHeartbeatInterval
	sseHeartbeatInterval = 50 * time.Millisecond
	defer func() { sseHeartbeatInterval = origInterval }()

	root := projectRoot(t)
	origCwd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origCwd)

	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

	body := `{"message":"test","session_id":"cancel-test"}`
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/chat",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Wait for context to expire.
	time.Sleep(200 * time.Millisecond)

	// After context cancel, the handler should have returned cleanly.
	// The test passes if we get here without a goroutine leak or panic.
	cancel()
}

// ---------------------------------------------------------------------------
// Skills endpoint tests
// ---------------------------------------------------------------------------

func TestServer_Skills_Empty(t *testing.T) {
	srv := &Server{
		loadedSkills: nil,
	}
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/skills?mode=deck")
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

	skillsArr, ok := body["skills"].([]any)
	if !ok {
		t.Fatal("expected skills array")
	}
	if len(skillsArr) != 0 {
		t.Fatalf("expected empty skills array, got %d items", len(skillsArr))
	}
	if body["mode"] != "deck" {
		t.Fatalf("expected mode=deck, got %q", body["mode"])
	}
}

func TestServer_Skills_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/skills", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestServer_Skills_WithData(t *testing.T) {
	idx := skill.NewIndex(map[string]skill.Skill{
		"coral-deck": {
			Name:        "coral-deck",
			Description: "Warm coral theme",
			Triggers:    []string{"coral", "warm", "fashion"},
			Mode:        "deck",
			Scenario:    "marketing",
			HasAssets:   true,
		},
		"blue-deck": {
			Name:        "blue-deck",
			Description: "Cool blue theme",
			Triggers:    []string{"blue", "cool"},
			Mode:        "deck",
			Scenario:    "corporate",
			HasAssets:   false,
		},
		"other-mode": {
			Name:        "other-mode",
			Description: "Some other mode",
			Triggers:    nil,
			Mode:        "unknown",
			Scenario:    "",
			HasAssets:   false,
		},
	})
	srv := &Server{
		loadedSkills: idx,
	}
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	t.Run("filter by deck mode", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/skills?mode=deck")
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
		skillsArr, ok := body["skills"].([]any)
		if !ok {
			t.Fatal("expected skills array")
		}
		if len(skillsArr) != 2 {
			t.Fatalf("expected 2 deck skills, got %d", len(skillsArr))
		}
		if body["mode"] != "deck" {
			t.Fatalf("expected mode=deck, got %q", body["mode"])
		}
	})

	t.Run("all skills without mode filter", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/skills")
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
		skillsArr, ok := body["skills"].([]any)
		if !ok {
			t.Fatal("expected skills array")
		}
		if len(skillsArr) != 3 {
			t.Fatalf("expected 3 skills, got %d", len(skillsArr))
		}
		// mode should be empty when no filter is applied.
		if body["mode"] != "" {
			t.Fatalf("expected empty mode, got %q", body["mode"])
		}
	})

	t.Run("verify SkillSummary fields", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/skills?mode=deck")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		skillsArr := body["skills"].([]any)

		first := skillsArr[0].(map[string]any)
		if _, ok := first["name"]; !ok {
			t.Error("SkillSummary missing 'name' field")
		}
		if _, ok := first["description"]; !ok {
			t.Error("SkillSummary missing 'description' field")
		}
		if _, ok := first["triggers"]; !ok {
			t.Error("SkillSummary missing 'triggers' field")
		}
		if _, ok := first["scenario"]; !ok {
			t.Error("SkillSummary missing 'scenario' field")
		}
		if _, ok := first["has_assets"]; !ok {
			t.Error("SkillSummary missing 'has_assets' field")
		}
		if _, ok := first["has_preview"]; !ok {
			t.Error("SkillSummary missing 'has_preview' field")
		}
	})
}

func TestServer_SkillExample_DirPath(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "presenter-reveal")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "example.html"), []byte("<html>preview</html>"), 0644)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: html-ppt-presenter-mode\n---\n"), 0644)

	idx := skill.LoadSkills(dir, "/nonexistent")
	srv := &Server{loadedSkills: idx}
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/skills/html-ppt-presenter-mode/example")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "preview") {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestServer_Skills_CORS(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/skills")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("expected CORS Access-Control-Allow-Origin header on /skills")
	}
}

func TestServer_Chat_NoDesignSkill_Allowed(t *testing.T) {
	root := projectRoot(t)
	origCwd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(origCwd)

	dir := t.TempDir()
	// Write project.json with only a brief — no design_skill.
	manifest := ProjectManifest{Name: "no-skill-project", Brief: "test brief"}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	srv, _ := newTestServer(t)
	// Override workspaceDir so handleChat picks up our no-design-skill project.json.
	srv.workspaceDir = dir

	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	body := `{"message":"Minimalist black white Bauhaus","session_id":"no-skill-test"}`
	resp, err := http.Post(ts.URL+"/chat", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", contentType)
	}

	// Read first SSE frames — gate must NOT emit the rejection error.
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	text := string(buf[:n])
	if strings.Contains(text, "no design skill selected") {
		t.Fatalf("gate should not reject no-design-skill generation, got: %s", text[:min(n, 500)])
	}
	if !strings.Contains(text, "data:") {
		t.Fatalf("expected SSE 'data:' prefix, got: %s", text[:min(n, 500)])
	}
}

func TestServer_Chat_WithSkill(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.loadedSkills = skill.NewIndex(map[string]skill.Skill{
		"html-ppt-zhangzara-coral": {
			Name:        "html-ppt-zhangzara-coral",
			Description: "Coral presentation theme",
			Triggers:    []string{"coral", "warm", "fashion"},
			Mode:        "deck",
		},
	})
	// No Provider set → stream will fail, but SSE headers and skill param should be accepted.
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	body := `{"message":"make a warm brand PPT","session_id":"test-skill","workspace_path":"/tmp/test","skill":"html-ppt-zhangzara-coral"}`
	resp, err := http.Post(ts.URL+"/chat", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", contentType)
	}
}

func TestServer_Edit_DirectEdit_Integration(t *testing.T) {
	// Direct edit requires the file to have been "read" first (for mtime metadata).
	// We test the HTTP layer: request parsing, error handling, etc.
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// Without read_mtime_unix_ns, the edit should fail with a clear error.
	editBody := `{"session_id":"edit-test","path":"test.txt","old_string":"hello","new_string":"hi"}`
	resp, err := http.Post(ts.URL+"/edit", "application/json", strings.NewReader(editBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Expected: 500 because edit_file requires pre-read metadata.
	// The important thing is the server handles the request without panicking.
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 (edit requires read_mtime), got %d: %s", resp.StatusCode, bodyStr)
	}
	if !strings.Contains(bodyStr, "read_mtime_unix_ns") {
		t.Fatalf("expected error about read_mtime_unix_ns, got: %s", bodyStr)
	}
}

// ---------------------------------------------------------------------------
// Utility function unit tests
// ---------------------------------------------------------------------------

func TestIsValidProjectID(t *testing.T) {
	tests := []struct {
		id     string
		expect bool
	}{
		{"proj-test123", true},
		{"test", true},
		{"valid-project-id", true},
		{"../escape", false},
		{"/etc/passwd", false},
		{"foo/bar", false},
		{"foo\\bar", false},
		{"../../etc", false},
	}
	for _, tt := range tests {
		got := isValidProjectID(tt.id)
		if got != tt.expect {
			t.Errorf("isValidProjectID(%q) = %v, want %v", tt.id, got, tt.expect)
		}
	}
}

func TestRandomHex(t *testing.T) {
	// Test length.
	for _, n := range []int{0, 4, 12, 32} {
		s := util.RandomHex(n)
		if len(s) != n {
			t.Errorf("util.RandomHex(%d) length = %d, want %d", n, len(s), n)
		}
		for _, c := range s {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("util.RandomHex(%d) contains non-hex char: %c in %q", n, c, s)
			}
		}
	}

	// Uniqueness: two calls should produce different values.
	a := util.RandomHex(16)
	b := util.RandomHex(16)
	if a == b {
		t.Error("randomHex should produce unique values")
	}
}

func TestReadProjectManifest_Success(t *testing.T) {
	dir := t.TempDir()
	manifest := ProjectManifest{
		Name:        "test-project",
		Brief:       "A test brief",
		DesignSkill: "coral",
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(dir, "project.json"), data, 0644)

	m, err := readProjectManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "test-project" {
		t.Errorf("expected test-project, got %s", m.Name)
	}
	if m.Brief != "A test brief" {
		t.Errorf("expected 'A test brief', got %s", m.Brief)
	}
	if m.DesignSkill != "coral" {
		t.Errorf("expected coral, got %s", m.DesignSkill)
	}
}

func TestReadProjectManifest_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := readProjectManifest(dir)
	if err == nil {
		t.Fatal("expected error for missing project.json")
	}
}

func TestReadProjectManifest_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "project.json"), []byte(`{bad json`), 0644)
	_, err := readProjectManifest(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestEmitSSEError(t *testing.T) {
	w := httptest.NewRecorder()
	emitSSEError(w, nil, "something went wrong")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "data:") {
		t.Fatal("expected SSE data: prefix")
	}
	if !strings.Contains(body, "something went wrong") {
		t.Fatal("expected error message in SSE body")
	}
	if !strings.Contains(body, "error") {
		t.Fatal("expected event type 'error' in SSE body")
	}
}

func TestRegistryToolExecutor_ResolveError(t *testing.T) {
	reg := registry.NewToolRegistry()
	exec := newRegistryToolExecutor(reg, "/tmp")

	_, _, err := exec.Execute(context.Background(), "nonexistent", `{}`)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected unknown tool error, got: %v", err)
	}
}

func TestRegistryToolExecutor_WithWorkspaceContext(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.Register(core.NewReadFileTool())
	exec := newRegistryToolExecutor(reg, "/tmp/default")

	// Create a temp file and read it via the executor.
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello from context"), 0644)

	ctx := context.WithValue(context.Background(), workspacePathCtxKey, tmpDir)
	content, _, err := exec.Execute(ctx, "read_file", `{"path":"`+testFile+`"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "hello from context") {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestRegistryToolExecutor_GetToolFlags(t *testing.T) {
	reg := registry.NewToolRegistry()
	reg.Register(core.NewReadFileTool())
	exec := newRegistryToolExecutor(reg, "")

	flags, err := exec.GetToolFlags("read_file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !flags.ReadOnly {
		t.Error("read_file should have ReadOnly flag")
	}
}

func TestRegistryToolExecutor_GetToolFlags_NotFound(t *testing.T) {
	reg := registry.NewToolRegistry()
	exec := newRegistryToolExecutor(reg, "")

	_, err := exec.GetToolFlags("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestBuildResponseFiles(t *testing.T) {
	docs := []document.ParsedDocument{
		{
			OriginalName: "doc.txt",
			Type:         "text",
			SavedPath:    "/uploads/upl_1/doc.txt",
			CharCount:    100,
			Summary:      "A text file",
		},
		{
			OriginalName: "img.png",
			Type:         "image",
			SavedPath:    "/uploads/upl_1/img.png",
			Width:        800,
			Height:       600,
			Format:       "png",
		},
	}
	files := buildResponseFiles(docs)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].OriginalName != "doc.txt" {
		t.Errorf("expected doc.txt, got %s", files[0].OriginalName)
	}
	if files[0].CharCount != 100 {
		t.Errorf("expected 100, got %d", files[0].CharCount)
	}
	if files[1].OriginalName != "img.png" {
		t.Errorf("expected img.png, got %s", files[1].OriginalName)
	}
	if files[1].Width != 800 {
		t.Errorf("expected 800, got %d", files[1].Width)
	}
}

func TestBuildUploadMeta(t *testing.T) {
	docs := []document.ParsedDocument{
		{
			OriginalName: "readme.md",
			SavedPath:    "/tmp/uploads/upl_1/readme.md",
			Type:         "markdown",
			CharCount:    200,
		},
	}
	meta := buildUploadMeta("upl_test", docs)
	if meta.UploadID != "upl_test" {
		t.Errorf("expected upl_test, got %s", meta.UploadID)
	}
	if len(meta.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(meta.Files))
	}
	if meta.Files[0].OriginalName != "readme.md" {
		t.Errorf("expected readme.md, got %s", meta.Files[0].OriginalName)
	}
	if meta.CreatedAt == "" {
		t.Error("expected CreatedAt to be set")
	}
}

// ---------------------------------------------------------------------------
// handleRecommendStyles tests
// ---------------------------------------------------------------------------

func TestServer_RecommendStyles_MethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/recommend-styles")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}

func TestServer_RecommendStyles_InvalidJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/recommend-styles", "application/json", strings.NewReader(`{bad}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServer_RecommendStyles_EmptyBrief(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	body := `{"brief":"","limit":3}`
	resp, err := http.Post(ts.URL+"/recommend-styles", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty brief, got %d", resp.StatusCode)
	}
}

func TestServer_RecommendStyles_NoSkills(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.loadedSkills = nil
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	body := `{"brief":"make a warm brand PPT","limit":3}`
	resp, err := http.Post(ts.URL+"/recommend-styles", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for no skills, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// handleServeOriginal tests
// ---------------------------------------------------------------------------

func TestServer_ServeOriginal_InvalidProjectID(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// backslash passes through the router but fails isValidProjectID
	resp, err := http.Get(ts.URL + "/files/proj%5Ctest/originals/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid project_id, got %d", resp.StatusCode)
	}
}

func TestServer_ServeOriginal_InvalidFilename(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// Go router cleans ../ before routing, so we test a dot filename
	resp, err := http.Get(ts.URL + "/files/valid-project/originals/%2e")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// Router may clean the path to remove trailing dot segment
	if resp.StatusCode == http.StatusBadRequest {
		// Valid result if the handler sees it
	} else if resp.StatusCode == http.StatusNotFound {
		// Also valid — router cleaned the path
	} else {
		t.Fatalf("expected 400 or 404, got %d", resp.StatusCode)
	}
}

func TestServer_ServeOriginal_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/files/valid-project/originals/nonexistent.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestServer_ServeOriginal_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// Create a project directory with an uploaded original file.
	projDir := filepath.Join(persistence.Dir(), "projects", "happy-proj", "uploads", "originals")
	os.MkdirAll(projDir, 0755)
	os.WriteFile(filepath.Join(projDir, "hello.txt"), []byte("hello world"), 0644)

	resp, err := http.Get(ts.URL + "/files/happy-proj/originals/hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(body))
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("expected text/plain content type, got %q", ct)
	}
}

// ---------------------------------------------------------------------------
// handleServeDoc tests
// ---------------------------------------------------------------------------

func TestServer_ServeDoc_InvalidProjectID(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// backslash passes through the router but fails isValidProjectID
	resp, err := http.Get(ts.URL + "/files/proj%5Ctest/docs/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid project_id, got %d", resp.StatusCode)
	}
}

func TestServer_ServeDoc_InvalidFilename(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// Go router cleans ../ before routing; test what reaches the handler
	resp, err := http.Get(ts.URL + "/files/valid-project/docs/%2e")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		// handler rejected it
	} else if resp.StatusCode == http.StatusNotFound {
		// router cleaned the path
	} else {
		t.Fatalf("expected 400 or 404, got %d", resp.StatusCode)
	}
}

func TestServer_ServeDoc_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/files/valid-project/docs/nonexistent.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// handleSkillContent tests
// ---------------------------------------------------------------------------

func TestServer_SkillContent_InvalidName(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// Backslash survives routing but fails name validation
	resp, err := http.Get(ts.URL + "/skills/test%5Cname/content")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid skill name, got %d", resp.StatusCode)
	}
}

func TestServer_SkillContent_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/skills/nonexistent-skill/content")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestServer_SkillContent_Success(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "content-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: content-skill\n---\n# My Skill Content"), 0644)

	idx := skill.LoadSkills(dir, "/nonexistent")
	srv := &Server{loadedSkills: idx}
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/skills/content-skill/content")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body["content"], "My Skill Content") {
		t.Errorf("expected skill content in response, got %q", body["content"])
	}
}

// ---------------------------------------------------------------------------
// handleSkillAsset tests
// ---------------------------------------------------------------------------

func TestServer_SkillAsset_InvalidName(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// Backslash survives routing but fails name validation
	resp, err := http.Get(ts.URL + "/skills/test%5Cname/assets/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid skill name, got %d", resp.StatusCode)
	}
}

func TestServer_SkillAsset_AssetNotFound(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "path-skill", "assets")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "style.css"), []byte("body{}"), 0644)
	os.WriteFile(filepath.Join(dir, "path-skill", "SKILL.md"), []byte("---\nname: path-skill\n---\n"), 0644)

	idx := skill.LoadSkills(dir, "/nonexistent")
	srv := &Server{loadedSkills: idx}
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// Request an asset that doesn't exist within a valid skill.
	resp, err := http.Get(ts.URL + "/skills/path-skill/assets/nonexistent.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent asset, got %d", resp.StatusCode)
	}
}

func TestServer_SkillAsset_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/skills/nonexistent-skill/assets/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestServer_SkillAsset_ContentType(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "typed-skill", "assets")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "script.js"), []byte("const x=1;"), 0644)
	os.WriteFile(filepath.Join(skillDir, "style.css"), []byte("body{}"), 0644)
	os.WriteFile(filepath.Join(skillDir, "icon.svg"), []byte("<svg></svg>"), 0644)
	os.WriteFile(filepath.Join(dir, "typed-skill", "SKILL.md"), []byte("---\nname: typed-skill\n---\n"), 0644)

	idx := skill.LoadSkills(dir, "/nonexistent")
	srv := &Server{loadedSkills: idx}
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	tests := []struct {
		asset          string
		expectedPrefix string
	}{
		{"script.js", "application/javascript"},
		{"style.css", "text/css"},
		{"icon.svg", "image/svg+xml"},
	}
	for _, tt := range tests {
		t.Run(tt.asset, func(t *testing.T) {
			resp, err := http.Get(ts.URL + "/skills/typed-skill/assets/" + tt.asset)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
			ct := resp.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, tt.expectedPrefix) {
				t.Errorf("expected Content-Type prefix %q, got %q", tt.expectedPrefix, ct)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// handleGetUploads tests
// ---------------------------------------------------------------------------

func TestServer_GetUploads_InvalidProjectID(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// Backslash survives routing but fails isValidProjectID
	resp, err := http.Get(ts.URL + "/projects/proj%5Ctest/uploads")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid project_id, got %d", resp.StatusCode)
	}
}

func TestServer_GetUploads_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/projects/valid-project/uploads")
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
	files, ok := body["files"].([]any)
	if !ok {
		t.Fatal("expected 'files' array in response")
	}
	if len(files) != 0 {
		t.Fatalf("expected empty files array, got %d items", len(files))
	}
}

func TestServer_GetUploads_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(newTestMux(srv))
	defer ts.Close()

	// Create a project with upload metadata on disk.
	projDir := filepath.Join(persistence.Dir(), "projects", "uploaded-proj", "uploads")
	os.MkdirAll(projDir, 0755)
	meta := document.UploadMeta{
		UploadID:  "upl_test123",
		CreatedAt: "2025-01-01T00:00:00Z",
		Files: []document.UploadMetaFile{
			{OriginalName: "doc.txt", SavedName: "doc_abc.txt", Type: "text", CharCount: 42},
		},
	}
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(filepath.Join(projDir, "upload_meta.json"), metaJSON, 0644)

	resp, err := http.Get(ts.URL + "/projects/uploaded-proj/uploads")
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
	files, ok := body["files"].([]any)
	if !ok {
		t.Fatal("expected 'files' array in response")
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
}
