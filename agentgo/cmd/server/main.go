// Command server runs an HTTP server exposing the streaming agent loop via SSE.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"agentgo/internal/agent"
	"agentgo/internal/config"
	"agentgo/internal/document"
	"agentgo/internal/hook"
	"agentgo/internal/hook/builtin"
	"agentgo/internal/observability"
	"agentgo/internal/persistence"
	"agentgo/internal/provider"
	"agentgo/internal/toolkit/bootstrap"
	"agentgo/internal/toolkit/extended/skill"
	"agentgo/internal/toolkit/registry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	sessionStore := persistence.NewSessionStore("")
	memoryStore := persistence.NewFileMemoryStore()

	reg := registry.NewToolRegistry()

	userSkillsDir := resolveRuntimeDir("AGENTGO_SKILLS_DIR", "skills")
	projectSkillsDir := resolveRuntimeDir("AGENTGO_PROJECT_SKILLS_DIR", "project-skills")
	loadedSkills := skill.LoadSkills(userSkillsDir, projectSkillsDir)
	log.Printf("loaded %d skills", loadedSkills.Len())
	log.Printf("skills directories: user=%s project=%s", userSkillsDir, projectSkillsDir)

	if err := bootstrap.RegisterAllTools(reg, loadedSkills); err != nil {
		log.Fatalf("register tools: %v", err)
	}

	workspaceDir := os.Getenv("AGENTGO_DATA_DIR")
	if workspaceDir == "" {
		workspaceDir = "."
	}

	hookConfigPath := ".agentgo/hooks.yaml"
	hookEngine := hook.NewEngine(hookConfigPath)
	builtin.RegisterBuiltins(hookEngine)
	log.Printf("hook engine initialized")

	// Global log file (all events, all sessions) — set via AGENTGO_LOG_FILE env var.
	if logPath := os.Getenv("AGENTGO_LOG_FILE"); logPath != "" {
		if err := observability.SetGlobalLogFile(logPath); err != nil {
			log.Printf("global log file: %v", err)
		}
	}

	srv := &Server{
		cfg:              cfg,
		sessionStore:     sessionStore,
		memoryStore:      memoryStore,
		reg:              reg,
		toolExec:         newRegistryToolExecutor(reg, workspaceDir),
		loadedSkills:     loadedSkills,
		userSkillsDir:    userSkillsDir,
		projectSkillsDir: projectSkillsDir,
		workspaceDir:     workspaceDir,
		documentPipeline: document.NewPipeline(),
		hookEngine:       hookEngine,
		consoleObs:       observability.NewConsoleObserver(),
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", srv.handleChat)
	mux.HandleFunc("/edit", srv.handleDirectEdit)
	mux.HandleFunc("/upload", srv.handleUpload)
	mux.HandleFunc("/recommend-styles", srv.handleRecommendStyles)
	mux.HandleFunc("/history", srv.handleHistory)
	mux.HandleFunc("/sessions/", srv.handleSession)
	mux.HandleFunc("/skills", srv.handleSkills)
	mux.HandleFunc("GET /skills/{name}/example", srv.handleSkillExample)
	mux.HandleFunc("GET /skills/{name}/content", srv.handleSkillContent)
	mux.HandleFunc("GET /skills/{name}/assets/{path...}", srv.handleSkillAsset)
	mux.HandleFunc("/skills/precipitate/stream", srv.handlePrecipitateStream)
	mux.HandleFunc("/skills/precipitate/confirm", srv.handlePrecipitateConfirm)
	mux.HandleFunc("/provider/config", srv.handleProviderConfig)
	mux.HandleFunc("/provider/test", srv.handleProviderTest)
	mux.HandleFunc("/provider/models", srv.handleProviderModels)
	mux.HandleFunc("GET /files/{project_id}/originals/{filename}", srv.handleServeOriginal)
	mux.HandleFunc("OPTIONS /files/{project_id}/originals/{filename}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /projects/{project_id}/uploads", srv.handleGetUploads)
	mux.HandleFunc("OPTIONS /projects/{project_id}/uploads", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /files/{project_id}/docs/{filename}", srv.handleServeDoc)
	mux.HandleFunc("OPTIONS /files/{project_id}/docs/{filename}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	httpServer := &http.Server{Addr: ":" + port, Handler: withCORS(mux)}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("server listening on http://localhost:%s", port)
	log.Printf("provider: %s | model: %s", cfg.Type, cfg.Model)

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Context key for per-request workspace path
// ---------------------------------------------------------------------------

type contextKey string

const (
	workspacePathCtxKey contextKey = "workspace_path"
	providerCtxKey      contextKey = "provider"
)

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

type Server struct {
	cfgMu            sync.RWMutex
	cfg              config.Config
	sessionStore     *persistence.SessionStore
	memoryStore      persistence.MemoryStore
	reg              *registry.ToolRegistry
	toolExec         agent.ToolExecutor
	loadedSkills     *skill.SkillIndex
	userSkillsDir    string
	projectSkillsDir string
	workspaceDir     string
	documentPipeline *document.Pipeline
	hookEngine       *hook.Engine

	Provider provider.StreamingProvider

	consoleObs     *observability.ConsoleObserver
	chatInProgress sync.Map // sessionID → struct{}
}

func (s *Server) consoleObserver() *observability.ConsoleObserver {
	if s.consoleObs == nil {
		s.consoleObs = observability.NewConsoleObserver()
	}
	return s.consoleObs
}

func (s *Server) getConfig() config.Config {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg
}

func (s *Server) updateConfig(cfg config.Config) {
	s.cfgMu.Lock()
	s.cfg = cfg
	s.cfgMu.Unlock()
	log.Printf("provider config updated: %s | model: %s", cfg.Type, cfg.Model)
}

func (s *Server) newLLMProvider() provider.StreamingProvider {
	if s.Provider != nil {
		return s.Provider
	}
	return s.getConfig().NewProvider()
}

func (s *Server) newLLMProviderWithEmitter(emitter *observability.Emitter) provider.StreamingProvider {
	if s.Provider != nil {
		return s.Provider
	}
	cfg := s.getConfig()
	p, err := provider.New(provider.Config{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		Type:    cfg.Type,
		Emitter: emitter,
	})
	if err != nil {
		return nil
	}
	return p
}

func resolveRuntimeDir(envName, dirName string) string {
	if v := os.Getenv(envName); v != "" {
		if abs, err := filepath.Abs(v); err == nil {
			return abs
		}
		return v
	}

	candidates := []string{dirName}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), dirName))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, dirName),
			filepath.Join(cwd, "agentgo", dirName),
		)
	}

	for _, candidate := range candidates {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			if abs, err := filepath.Abs(candidate); err == nil {
				return abs
			}
			return candidate
		}
	}

	return dirName
}

func hookSessionState(engine *hook.Engine) *hook.SessionState {
	if engine == nil {
		return nil
	}
	return engine.State()
}
