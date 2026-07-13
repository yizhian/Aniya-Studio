package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentctx "agentgo/internal/context"
	"agentgo/internal/document"
	"agentgo/internal/persistence"
	p "agentgo/internal/provider"
	"agentgo/internal/toolkit/bootstrap"
	"agentgo/internal/toolkit/registry"
)

// ---------------------------------------------------------------------------
// E2E Test Harness
// ---------------------------------------------------------------------------

// e2eHarness provides a fully-wired test server backed by a mock LLM provider.
type e2eHarness struct {
	Server       *httptest.Server
	MockProvider *p.MockProviderServer
	WorkDir      string
	t            *testing.T

	sessionStore *persistence.SessionStore
	memoryStore  persistence.MemoryStore
	reg          *registry.ToolRegistry
	srv          *Server
	origDir      string
}

// e2eSSEEvent is a parsed SSE event from the chat endpoint.
type e2eSSEEvent struct {
	Type  string         `json:"type"`
	Time  string         `json:"time"`
	Round int            `json:"round"`
	Data  map[string]any `json:"data"`
}

// uploadResponse mirrors the server's upload response.
type uploadTestResponse struct {
	UploadID    string                 `json:"upload_id"`
	SessionID   string                 `json:"session_id"`
	Files       []uploadResponseFile   `json:"files"`
	SummaryText string                 `json:"summary_text"`
	ParseStats  document.ParseStats    `json:"parse_stats"`
}


// editTestResponse mirrors the server's direct edit response.
type editTestResponse struct {
	Version  int                      `json:"version"`
	Snapshot *agentctx.DesignSnapshot `json:"snapshot"`
	Result   string                   `json:"result"`
}

func newE2EHarness(t *testing.T, script p.MockSSEScript) *e2eHarness {
	t.Helper()

	workDir := t.TempDir()

	// Write a minimal project.json for manifest-driven chat.
	writeTestProjectManifest(t, workDir)

	// Change to project root so prompts/*.md can be found.
	origDir, _ := os.Getwd()
	projectRoot := findProjectRoot(t)
	os.Chdir(projectRoot)

	// Create mock provider server.
	mockProvider := p.NewMockProviderServer(script)

	// Setup persistence stores.
	sessionStore := persistence.NewSessionStore(filepath.Join(workDir, "sessions"))
	memoryStore := persistence.NewFileMemoryStore()

	// Setup tool registry.
	reg := registry.NewToolRegistry()
	if err := bootstrap.RegisterAllTools(reg, nil); err != nil {
		t.Fatalf("register tools: %v", err)
	}

	// Create provider pointing to mock.
	provider := p.NewOpenAIProvider(p.Config{
		APIKey:  "test-key",
		BaseURL: mockProvider.URL,
		Model:   "test-model",
		Type:    p.ProviderOpenAI,
	})

	// Build the Server.
	srv := &Server{
		sessionStore:     sessionStore,
		memoryStore:      memoryStore,
		reg:              reg,
		toolExec:         newRegistryToolExecutor(reg, workDir),
		workspaceDir:     workDir,
		documentPipeline: document.NewPipeline(),
		Provider:         provider,
	}

	// Build HTTP mux matching production routes.
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", srv.handleChat)
	mux.HandleFunc("/edit", srv.handleDirectEdit)
	mux.HandleFunc("/upload", srv.handleUpload)
	mux.HandleFunc("/history", srv.handleHistory)
	mux.HandleFunc("/sessions/", srv.handleSession)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	ts := httptest.NewServer(withCORS(mux))

	return &e2eHarness{
		Server:       ts,
		MockProvider: mockProvider,
		WorkDir:      workDir,
		t:            t,
		sessionStore: sessionStore,
		memoryStore:  memoryStore,
		reg:          reg,
		srv:          srv,
		origDir:      origDir,
	}
}

func (h *e2eHarness) Close() {
	h.Server.Close()
	h.MockProvider.Close()
	if h.origDir != "" {
		os.Chdir(h.origDir)
	}
}

// ensureProjectRoot changes cwd to the project root so that prompts/
// and other relative-path files are found.
func (h *e2eHarness) ensureProjectRoot() {
	projectRoot := findProjectRoot(h.t)
	if err := os.Chdir(projectRoot); err != nil {
		h.t.Fatalf("chdir to project root %q: %v", projectRoot, err)
	}
}

// Chat sends a message and returns parsed SSE events.
func (h *e2eHarness) Chat(sessionID, message string) []e2eSSEEvent {
	h.ensureProjectRoot()

	body := fmt.Sprintf(`{"message":%q,"session_id":%q}`, message, sessionID)
	resp, err := http.Post(h.Server.URL+"/chat", "application/json", strings.NewReader(body))
	if err != nil {
		h.t.Fatalf("chat request failed: %v", err)
	}
	defer resp.Body.Close()

	return parseSSEStream(resp.Body)
}

// ChatAutoSession sends without session ID, returns events and generated session ID.
func (h *e2eHarness) ChatAutoSession(message string) ([]e2eSSEEvent, string) {
	h.ensureProjectRoot()

	body := fmt.Sprintf(`{"message":%q}`, message)
	resp, err := http.Post(h.Server.URL+"/chat", "application/json", strings.NewReader(body))
	if err != nil {
		h.t.Fatalf("chat request failed: %v", err)
	}
	defer resp.Body.Close()

	sessionID := resp.Header.Get("X-Session-Id")
	return parseSSEStream(resp.Body), sessionID
}

// CreateProject creates a minimal project directory under .agentgo/projects/.
func (h *e2eHarness) CreateProject(projectID string) {
	projDir := filepath.Join(".agentgo", "projects", projectID)
	if err := os.MkdirAll(projDir, 0755); err != nil {
		h.t.Fatalf("create project dir: %v", err)
	}
}

// Upload sends files to the upload endpoint.
func (h *e2eHarness) Upload(projectID string, files map[string]string) *uploadTestResponse {
	body := &bytesReadCloser{}
	body.data = []byte(buildMultipartBody(projectID, files))

	resp, err := http.Post(h.Server.URL+"/upload", "multipart/form-data; boundary=testboundary", body)
	if err != nil {
		h.t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()

	var ur uploadTestResponse
	json.NewDecoder(resp.Body).Decode(&ur)
	return &ur
}

// Edit sends a direct edit request.
func (h *e2eHarness) Edit(sessionID, path, oldString, newString, readMtime string) *editTestResponse {
	reqBody := fmt.Sprintf(
		`{"session_id":%q,"path":%q,"old_string":%q,"new_string":%q,"read_mtime_unix_ns":%q}`,
		sessionID, path, oldString, newString, readMtime,
	)
	resp, err := http.Post(h.Server.URL+"/edit", "application/json", strings.NewReader(reqBody))
	if err != nil {
		h.t.Fatalf("edit request failed: %v", err)
	}
	defer resp.Body.Close()

	var er editTestResponse
	json.NewDecoder(resp.Body).Decode(&er)
	return &er
}

// ListSessions returns session summaries.
func (h *e2eHarness) ListSessions() []persistence.SessionInfo {
	resp, err := http.Get(h.Server.URL + "/history")
	if err != nil {
		h.t.Fatalf("history request failed: %v", err)
	}
	defer resp.Body.Close()

	var infos []persistence.SessionInfo
	json.NewDecoder(resp.Body).Decode(&infos)
	return infos
}

// GetSession returns raw session JSON data.
func (h *e2eHarness) GetSession(id string) (map[string]any, error) {
	resp, err := http.Get(h.Server.URL + "/sessions/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

// Health checks the health endpoint.
func (h *e2eHarness) Health() bool {
	resp, err := http.Get(h.Server.URL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// parseSSEStream reads SSE events from a response body.
func parseSSEStream(r io.Reader) []e2eSSEEvent {
	var events []e2eSSEEvent
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var ev e2eSSEEvent
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events
}

// bytesReadCloser implements io.Reader from a byte slice.
type bytesReadCloser struct {
	data   []byte
	offset int
}

func (b *bytesReadCloser) Read(p []byte) (int, error) {
	if b.offset >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.offset:])
	b.offset += n
	return n, nil
}

func (b *bytesReadCloser) Close() error { return nil }

func buildMultipartBody(projectID string, files map[string]string) string {
	var b strings.Builder
	boundary := "testboundary"

	for name, content := range files {
		b.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		b.WriteString(fmt.Sprintf("Content-Disposition: form-data; name=\"files\"; filename=\"%s\"\r\n", name))
		b.WriteString("Content-Type: application/octet-stream\r\n\r\n")
		b.WriteString(content)
		b.WriteString("\r\n")
	}

	b.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	b.WriteString("Content-Disposition: form-data; name=\"project_id\"\r\n\r\n")
	b.WriteString(projectID)
	b.WriteString("\r\n")

	b.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	return b.String()
}

func findProjectRoot(t *testing.T) string {
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
