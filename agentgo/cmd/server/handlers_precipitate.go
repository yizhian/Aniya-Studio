package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentgo/internal/agent"
	agentctx "agentgo/internal/context"
	"agentgo/internal/persistence"
	"agentgo/internal/toolkit/extended/skill"
)

//go:embed prompts/precipitate.md
var precipitateTemplate string

func precipitateSystemPrompt(ctx agent.SystemPromptContext) string {
	return agent.BuildSystemPrompt(precipitateTemplate, ctx)
}

type precipitatePreviewRequest struct {
	HTMLContent string `json:"html_content"`
}

type precipitatePreviewResponse struct {
	SuggestedName string `json:"suggested_name"`
	Description   string `json:"description"`
	Scenario      string `json:"scenario"`
	SkillMD       string `json:"skill_md"`
	ExampleHTML   string `json:"example_html"`
}

type precipitateConfirmRequest struct {
	SkillName   string `json:"skill_name"`
	Scenario    string `json:"scenario"`
	SkillMD     string `json:"skill_md"`
	ExampleHTML string `json:"example_html"`
}

type precipitateConfirmResponse struct {
	SkillName string `json:"skill_name"`
	DirPath   string `json:"dir_path"`
	Reloaded  bool   `json:"reloaded"`
}

func (s *Server) handlePrecipitateStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req precipitatePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.HTMLContent) == "" {
		http.Error(w, "html_content is required", http.StatusBadRequest)
		return
	}
	if len(req.HTMLContent) > 2_000_000 {
		http.Error(w, "html_content exceeds 2MB limit", http.StatusBadRequest)
		return
	}

	sessID := fmt.Sprintf("precip_%d", time.Now().UnixNano())

	wsPath := filepath.Join(s.workspaceDir, ".precipitate", sessID)
	if err := os.MkdirAll(wsPath, 0755); err != nil {
		log.Printf("precipitate stream: mkdir workspace: %v", err)
		http.Error(w, "failed to create workspace", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(wsPath)

	inputPath := filepath.Join(wsPath, "input.html")
	if err := os.WriteFile(inputPath, []byte(req.HTMLContent), 0644); err != nil {
		log.Printf("precipitate stream: write input.html: %v", err)
		http.Error(w, "failed to write input.html", http.StatusInternalServerError)
		return
	}

	log.Printf("precipitate stream: starting agent (input_html=%d bytes, workspace=%s)", len(req.HTMLContent), wsPath)

	sess, err := setupSSE(w, r, sessID, s.consoleObserver())
	if err != nil {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	llmProvider := s.newLLMProvider()

	ctxMgr := agentctx.NewContextManager(wsPath, sessID, "input.html")

	sysPromptCtx := agent.DefaultSystemPromptContext()
	sysPromptCtx.ToolPrompts = s.reg.GetActiveToolPrompts()
	sysPromptCtx.WorkspaceRoot = wsPath

	loopCfg := agent.StreamingLoopConfig{
		SystemPrompt:  precipitateSystemPrompt(sysPromptCtx),
		UserMessage:   "Analyze the HTML below and extract its design system. Write SKILL.md.\n\n<source_html>\n" + req.HTMLContent + "\n</source_html>",
		Tools:         s.reg.GetActiveToolDefinitions(),
		MaxRounds:     15,
		MaxTokens:     32768,
		Provider:      llmProvider,
		Execute:       s.toolExec,
		Emitter:       sess.Emitter(),
		SessionStore:  persistence.NewSessionStore(""),
		MemoryStore:   s.memoryStore,
		SessionID:     sessID,
		WorkspacePath: wsPath,
		ContextManager: ctxMgr,
		HookEngine:    s.hookEngine,
		SessionState:  hookSessionState(s.hookEngine),
	}

	reqCtx := context.WithValue(r.Context(), workspacePathCtxKey, wsPath)
	reqCtx = context.WithValue(reqCtx, providerCtxKey, llmProvider)
	_, err = agent.RunStreaming(reqCtx, loopCfg)
	sess.Close()

	if err != nil && r.Context().Err() == nil {
		log.Printf("precipitate stream: agent error: %v", err)
		sess.EmitError(err.Error())
		return
	}

	if r.Context().Err() != nil {
		log.Printf("precipitate stream: client disconnected")
		return
	}

	skillMDPath := filepath.Join(wsPath, "SKILL.md")
	examplePath := filepath.Join(wsPath, "example.html")

	inputBytes, cpErr := os.ReadFile(inputPath)
	if cpErr != nil {
		log.Printf("precipitate stream: read input.html for copy: %v", cpErr)
		sess.EmitError("Failed to read input.html for example copy")
		return
	}
	if err := os.WriteFile(examplePath, inputBytes, 0644); err != nil {
		log.Printf("precipitate stream: write example.html copy: %v", err)
		sess.EmitError("Failed to create example.html")
		return
	}

	skillMDBytes, err1 := os.ReadFile(skillMDPath)
	if err1 != nil {
		log.Printf("precipitate stream: read output: SKILL.md=%v", err1)
		sess.EmitError("Agent did not produce SKILL.md")
		return
	}
	exampleBytes, _ := os.ReadFile(examplePath)

	skillMD := string(skillMDBytes)
	exampleHTML := string(exampleBytes)

	fm, _ := skill.ParseFrontmatter(skillMDBytes)
	description := fm.Description
	scenario := skill.NormalizeScenario(fm.OD.Scenario)

	suggestedName := skill.SanitizeSkillName(fm.Name)
	if suggestedName == "" {
		suggestedName = skill.SanitizeSkillName("html-ppt-custom")
	}
	suggestedName = dedupSkillName(s.loadedSkills, suggestedName)

	skillMD = skill.MergeFrontmatter(skillMD, suggestedName, description, scenario)

	log.Printf("precipitate stream: result suggested_name=%s description=%d bytes scenario=%s skill_md=%d bytes example_html=%d bytes",
		suggestedName, len(description), scenario, len(skillMD), len(exampleHTML))

	resultEv := sseEvent{
		Type: "precipitate_result",
		Time: time.Now(),
		Data: map[string]any{
			"suggested_name": suggestedName,
			"description":    description,
			"scenario":       scenario,
			"skill_md":       skillMD,
			"example_html":   exampleHTML,
		},
	}
	sess.SendEvent(resultEv)
}

func (s *Server) handlePrecipitateConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req precipitateConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	skillName := skill.SanitizeSkillName(req.SkillName)
	if skillName == "" || len(skillName) > 100 {
		http.Error(w, "invalid skill_name", http.StatusBadRequest)
		return
	}
	if strings.Contains(skillName, "..") || strings.Contains(skillName, "/") || strings.Contains(skillName, "\\") {
		http.Error(w, "invalid skill_name", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.SkillMD) == "" {
		http.Error(w, "skill_md is required", http.StatusBadRequest)
		return
	}
	if len(req.SkillMD) > 200_000 {
		http.Error(w, "skill_md exceeds 200KB limit", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.ExampleHTML) == "" {
		http.Error(w, "example_html is required", http.StatusBadRequest)
		return
	}
	if len(req.ExampleHTML) > 1_000_000 {
		http.Error(w, "example_html exceeds 1MB limit", http.StatusBadRequest)
		return
	}

	if s.loadedSkills != nil {
		if _, ok := s.loadedSkills.ByName(skillName); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{
				"error":      "skill already exists",
				"skill_name": skillName,
			})
			return
		}
	}

	fm, _ := skill.ParseFrontmatter([]byte(req.SkillMD))
	desc := fm.Description
	sc := skill.NormalizeScenario(fm.OD.Scenario)
	if sc == "" {
		sc = skill.NormalizeScenario(req.Scenario)
	}
	skillMD := skill.MergeFrontmatter(req.SkillMD, skillName, desc, sc)

	dirPath := filepath.Join(s.userSkillsDir, skillName)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.Printf("precipitate confirm: mkdir %q: %v", dirPath, err)
		http.Error(w, "failed to create skill directory", http.StatusInternalServerError)
		return
	}

	examplePath := filepath.Join(dirPath, "example.html")
	if err := os.WriteFile(examplePath, []byte(req.ExampleHTML), 0644); err != nil {
		os.RemoveAll(dirPath)
		log.Printf("precipitate confirm: write example.html: %v", err)
		http.Error(w, "failed to write example.html", http.StatusInternalServerError)
		return
	}

	skillMDPath := filepath.Join(dirPath, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte(skillMD), 0644); err != nil {
		os.RemoveAll(dirPath)
		log.Printf("precipitate confirm: write SKILL.md: %v", err)
		http.Error(w, "failed to write SKILL.md", http.StatusInternalServerError)
		return
	}

	s.reloadSkills()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(precipitateConfirmResponse{
		SkillName: skillName,
		DirPath:   dirPath,
		Reloaded:  true,
	})
}

func dedupSkillName(idx *skill.SkillIndex, name string) string {
	if idx == nil {
		return name
	}
	candidate := name
	for i := 2; i <= 20; i++ {
		if _, ok := idx.ByName(candidate); !ok {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", name, i)
	}
	return name
}
