package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"agentgo/internal/toolkit/extended/skill"
)

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mode := r.URL.Query().Get("mode")
	if s.loadedSkills == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"skills": []skill.SkillSummary{}, "mode": mode})
		return
	}
	var summaries []skill.SkillSummary
	if mode != "" {
		for _, sk := range s.loadedSkills.ByMode(mode) {
			summaries = append(summaries, skill.SkillSummary{
				Name:        sk.Name,
				Description: sk.Description,
				Triggers:    sk.Triggers,
				Scenario:    sk.Scenario,
				HasAssets:   sk.HasAssets,
				HasPreview:  sk.HasPreview,
			})
		}
	} else {
		for _, name := range s.loadedSkills.AllNames() {
			sk, ok := s.loadedSkills.ByName(name)
			if !ok {
				continue
			}
			summaries = append(summaries, skill.SkillSummary{
				Name:        sk.Name,
				Description: sk.Description,
				Triggers:    sk.Triggers,
				Scenario:    sk.Scenario,
				HasAssets:   sk.HasAssets,
				HasPreview:  sk.HasPreview,
			})
		}
	}
	if summaries == nil {
		summaries = []skill.SkillSummary{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"skills": summaries,
		"mode":   mode,
	})
}

func (s *Server) handleSkillContent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		http.Error(w, "invalid skill name", http.StatusBadRequest)
		return
	}
	if s.loadedSkills == nil {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}
	sk, ok := s.loadedSkills.ByName(name)
	if !ok {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}
	data, err := os.ReadFile(sk.FilePath)
	if err != nil {
		http.Error(w, "skill content unavailable", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"content": string(data)})
}

func (s *Server) handleSkillExample(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		http.Error(w, "invalid skill name", http.StatusBadRequest)
		return
	}
	if s.loadedSkills == nil {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}
	sk, ok := s.loadedSkills.ByName(name)
	if !ok {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}
	examplePath := skill.ResolvePreviewPath(sk)
	if examplePath == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("no example for skill: %s", name)})
		return
	}
	data, err := os.ReadFile(examplePath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("no example for skill: %s", name)})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleSkillAsset(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	assetPath := r.PathValue("path")
	if name == "" || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		http.Error(w, "invalid skill name", http.StatusBadRequest)
		return
	}
	if assetPath == "" || strings.Contains(assetPath, "..") {
		http.Error(w, "invalid asset path", http.StatusBadRequest)
		return
	}
	if s.loadedSkills == nil {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}
	sk, ok := s.loadedSkills.ByName(name)
	if !ok {
		http.Error(w, "skill not found", http.StatusNotFound)
		return
	}
	fullPath := skill.ResolveAssetPath(sk, assetPath)
	if fullPath == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("asset not found: %s", assetPath)})
		return
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("asset not found: %s", assetPath)})
		return
	}
	ext := filepath.Ext(assetPath)
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Write(data)
}

func (s *Server) reloadSkills() {
	if s.loadedSkills == nil {
		return
	}
	s.loadedSkills.Reload(s.userSkillsDir, s.projectSkillsDir)
}
