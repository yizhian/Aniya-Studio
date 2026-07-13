package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"agentgo/internal/agent"
	agentctx "agentgo/internal/context"
	"agentgo/internal/document"
	"agentgo/internal/hook"
	"agentgo/internal/model"
	"agentgo/internal/persistence"
	"agentgo/internal/toolkit/extended/skill"
)

// ProjectManifest is the project.json schema read by AgentGo at /chat time.
type ProjectManifest struct {
	Name        string `json:"name"`
	Brief       string `json:"brief"`
	DesignSkill string `json:"design_skill"`
}

func readProjectManifest(wsPath string) (ProjectManifest, error) {
	data, err := os.ReadFile(filepath.Join(wsPath, "project.json"))
	if err != nil {
		return ProjectManifest{}, fmt.Errorf("read project.json: %w", err)
	}
	var m ProjectManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return ProjectManifest{}, fmt.Errorf("parse project.json: %w", err)
	}
	return m, nil
}

// handleChat runs the full chat pipeline: session loading → context assembly →
// streaming agent loop → cleanup.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Message       string           `json:"message"`
		SessionID     string           `json:"session_id,omitempty"`
		WorkspacePath string           `json:"workspace_path,omitempty"`
		ActiveFile    string           `json:"active_file,omitempty"`
		DomContext    map[string]any   `json:"dom_context,omitempty"`
		Attachments   []map[string]any `json:"attachments,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	sessID := strings.TrimSpace(req.SessionID)
	if sessID == "" {
		sessID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	if strings.ContainsAny(sessID, "/\\") || strings.Contains(sessID, "..") {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	if _, loaded := s.chatInProgress.LoadOrStore(sessID, struct{}{}); loaded {
		http.Error(w, "chat already in progress for this session", http.StatusConflict)
		return
	}
	defer s.chatInProgress.Delete(sessID)

	wsPath := s.workspaceDir
	if req.WorkspacePath != "" {
		wsPath = req.WorkspacePath
	}

	projectSessDir := filepath.Join(wsPath, ".agentgo", "sessions")
	projectSessionStore := persistence.NewSessionStore(projectSessDir)

	var history []model.Message
	var previousTimeline []agent.TimelineEvent
	loadData := func(store *persistence.SessionStore) (*agent.LoopState, bool) {
		data, err := store.Load(sessID)
		if err != nil {
			return nil, false
		}
		var prev agent.LoopState
		if err := json.Unmarshal(data, &prev); err == nil {
			for _, m := range prev.Messages {
				if m.Role != "system" {
					history = append(history, m)
				}
			}
			previousTimeline = prev.Timeline
			return &prev, true
		}
		var raw struct {
			Messages []map[string]any `json:"messages"`
		}
		if err := json.Unmarshal(data, &raw); err == nil {
			for _, item := range raw.Messages {
				role, _ := item["role"].(string)
				if role == "" || role == "system" {
					continue
				}
				content, _ := item["content"].(string)
				toolCallID, _ := item["tool_call_id"].(string)
				history = append(history, model.Message{
					Role:       role,
					Content:    content,
					ToolCallID: toolCallID,
				})
			}
			return nil, true
		}
		return nil, false
	}
	if _, loaded := loadData(projectSessionStore); loaded {
		log.Printf("session %s: loaded %d history messages (per-project)", sessID, len(history))
	} else if _, loaded2 := loadData(s.sessionStore); loaded2 {
		log.Printf("session %s: loaded %d history messages (global fallback)", sessID, len(history))
	}

	memoryContent, memErr := s.memoryStore.LoadUserMemory(wsPath)
	if memErr != nil {
		log.Printf("load user memory: %v", memErr)
	}

	sess, err := setupSSE(w, r, sessID, s.consoleObserver())
	if err != nil {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	tools := s.reg.GetActiveToolDefinitions()
	ctxMgr := agentctx.NewContextManager(wsPath, sessID, req.ActiveFile)
	ctxMgr.SetEmitter(sess.Emitter())
	if te, ok := s.toolExec.(*registryToolExecutor); ok {
		te.SetToolEmitter(sess.Emitter())
	}
	stage := ctxMgr.ClassifyStage()

	sysPromptCtx := agent.DefaultSystemPromptContext()
	sysPromptCtx.CWD = wsPath
	sysPromptCtx.UserMemory = memoryContent
	if s.loadedSkills != nil {
		sysPromptCtx.Skills = skill.FormatSkillsForMode(s.loadedSkills, "deck", 8000)
	}
	sysPromptCtx.ToolPrompts = s.reg.GetActiveToolPrompts()
	sysPromptCtx.WorkspaceRoot = wsPath

	if meta := readUploadMeta(filepath.Join(wsPath, "uploads")); meta != nil {
		sysPromptCtx.UploadedFiles = document.FormatUploadedFilesForSystemPrompt(meta)
	}

	manifest, manifestErr := readProjectManifest(wsPath)
	if manifestErr != nil {
		log.Printf("session %s: read project manifest: %v", sessID, manifestErr)
	}

	if manifest.DesignSkill != "" && s.loadedSkills != nil {
		if _, ok := s.loadedSkills.ByName(manifest.DesignSkill); !ok {
			log.Printf("session %s: design_skill %q not found in loaded skills", sessID, manifest.DesignSkill)
		}
	}

	if manifest.Brief != "" {
		sysPromptCtx.ProjectBrief = manifest.Brief
	} else if req.Message != "" {
		sysPromptCtx.ProjectBrief = req.Message
	}

	selectedDesignSkill := manifest.DesignSkill
	if selectedDesignSkill != "" {
		sysPromptCtx.SkillOverride = fmt.Sprintf(
			"[Selected design skill: %s]\n"+
				"The user selected this style for this request. You have loaded grapesjs-html-compliance rules. "+
				"Read skills/%s/example.html as a design reference — extract its design DNA "+
				"(colors, fonts, layout patterns, visual style rules) and apply to a GrapesJS-compliant skeleton. "+
				"Do NOT copy implementation patterns (opacity visibility, position:fixed, viewport units, @import, external scripts). "+
				"Do not ask the user to choose a style again.",
			selectedDesignSkill, selectedDesignSkill,
		)
	}

	sysPrompt := agent.BuildDefaultSystemPrompt(sysPromptCtx)

	if s.hookEngine != nil {
		s.hookEngine.SetEmitter(sess.Emitter())
		s.hookEngine.InitState(sessID, wsPath, stage)
		hctx := &hook.HookContext{
			SessionID:           sessID,
			WorkspacePath:       wsPath,
			Stage:               stage,
			Round:               0,
			SessionState:        s.hookEngine.State(),
			SelectedDesignSkill: selectedDesignSkill,
		}
		warnings, hookErr := s.hookEngine.Run(r.Context(), hook.PointUserPromptSubmit, hctx)
		if hookErr != nil {
			if blocked, ok := hookErr.(*hook.BlockedError); ok {
				sess.Close()
				sess.EmitError(blocked.Error())
				return
			}
			log.Printf("hook warning (user_prompt_submit): %v", hookErr)
		}
		if len(warnings) > 0 && s.hookEngine.State() != nil {
			s.hookEngine.State().AddPendingWarnings(warnings)
		}
	}

	llmProvider := s.newLLMProviderWithEmitter(sess.Emitter())

	loopCfg := agent.StreamingLoopConfig{
		SystemPrompt:     sysPrompt,
		UserMessage:      req.Message,
		History:          history,
		Tools:            tools,
		MaxRounds:        100,
		MaxTokens:        32768,
		Provider:         llmProvider,
		Execute:          s.toolExec,
		Emitter:          sess.Emitter(),
		SessionStore:     projectSessionStore,
		MemoryStore:      s.memoryStore,
		SessionID:        sessID,
		WorkspacePath:    wsPath,
		ContextManager:   ctxMgr,
		HookEngine:       s.hookEngine,
		SessionState:     hookSessionState(s.hookEngine),
		PreviousTimeline: previousTimeline,
		DesignSkill:      selectedDesignSkill,
		DomContext:       req.DomContext,
		Attachments:      req.Attachments,
	}
	reqCtx := context.WithValue(r.Context(), workspacePathCtxKey, wsPath)
	reqCtx = context.WithValue(reqCtx, providerCtxKey, llmProvider)
	_, err = agent.RunStreaming(reqCtx, loopCfg)
	sess.Close()
	if err != nil && r.Context().Err() == nil {
		log.Printf("session %s error: %v", sessID, err)
		// Error already emitted through agent.Emitter; avoid duplicate SSE write
	}
}

func (s *Server) handleRecommendStyles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Brief string `json:"brief"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Brief) == "" {
		http.Error(w, "brief is required", http.StatusBadRequest)
		return
	}
	if req.Limit <= 0 {
		req.Limit = 3
	}
	if s.loadedSkills == nil || s.loadedSkills.Len() == 0 {
		http.Error(w, "no skills available", http.StatusServiceUnavailable)
		return
	}

	llmProvider := s.newLLMProviderWithEmitter(nil)

	ranker := skill.NewSkillRanker(s.loadedSkills)
	recs, err := ranker.Recommend(r.Context(), llmProvider, req.Brief, "", req.Limit)
	if err != nil {
		http.Error(w, "recommend failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if recs == nil {
		recs = []skill.SkillRecommendation{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"recommendations": recs})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessions, err := s.listAllSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if sessions == nil {
		sessions = []persistence.SessionInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func (s *Server) listAllSessions() ([]persistence.SessionInfo, error) {
	seen := make(map[string]bool)
	var merged []persistence.SessionInfo

	appendFrom := func(store *persistence.SessionStore) error {
		infos, err := store.List()
		if err != nil {
			return err
		}
		for _, info := range infos {
			if seen[info.ID] {
				continue
			}
			seen[info.ID] = true
			merged = append(merged, info)
		}
		return nil
	}

	if err := appendFrom(s.sessionStore); err != nil {
		return nil, err
	}
	if s.workspaceDir != "" {
		wsStore := persistence.NewSessionStore(filepath.Join(s.workspaceDir, ".agentgo", "sessions"))
		if err := appendFrom(wsStore); err != nil {
			return nil, err
		}
	}
	projectsDir := filepath.Join(persistence.Dir(), "projects")
	if entries, err := os.ReadDir(projectsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			projStore := persistence.NewSessionStore(filepath.Join(projectsDir, e.Name(), ".agentgo", "sessions"))
			if err := appendFrom(projStore); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].UpdatedAt.After(merged[j].UpdatedAt)
	})
	return merged, nil
}

func (s *Server) sessionFilePaths(id string) []string {
	var paths []string
	projSessionPath := filepath.Join(persistence.Dir(), "projects", id, ".agentgo", "sessions", id+".json")
	paths = append(paths, projSessionPath)
	if s.workspaceDir != "" {
		paths = append(paths, filepath.Join(s.workspaceDir, ".agentgo", "sessions", id+".json"))
	}
	return paths
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/sessions/")
	id = strings.TrimSuffix(id, ".json")
	if strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		http.Error(w, "invalid session id", http.StatusBadRequest)
		return
	}

	for _, path := range s.sessionFilePaths(id) {
		if data, err := os.ReadFile(path); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return
		}
	}

	data, err := s.sessionStore.Load(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *Server) handleDirectEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	args := fmt.Sprintf(`{"path": %q, "old_string": %q, "new_string": %q}`,
		req.Path, req.OldString, req.NewString)
	result, _, err := s.toolExec.Execute(r.Context(), "edit_file", args)
	if err != nil {
		http.Error(w, fmt.Sprintf("edit failed: %v: %s", err, result), http.StatusInternalServerError)
		return
	}

	ctxMgr := agentctx.NewContextManager(s.workspaceDir, req.SessionID, "")
	toolCalls := []model.ToolCall{{
		ID:   "direct_edit",
		Type: "function",
		Function: model.ToolCallFunction{
			Name:      "edit_file",
			Arguments: args,
		},
	}}
	execSummaries := []agentctx.ToolExecSummary{{
		ToolCallID: "direct_edit",
		Success:    true,
	}}
	ctxMgr.DetectHTMLModification(toolCalls, execSummaries)
	newVersion, err := ctxMgr.FinalizeSnapshot("")
	if err != nil {
		http.Error(w, "version failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"version":  newVersion,
		"snapshot": ctxMgr.LatestSnapshot(),
		"result":   result,
	})
}
