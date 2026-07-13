package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"agentgo/internal/persistence"
)

// handleServeOriginal serves an uploaded original file for viewing in the browser.
// GET /files/{project_id}/originals/{filename}
func (s *Server) handleServeOriginal(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	filename := r.PathValue("filename")

	if !isValidProjectID(projectID) {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	safeName := filepath.Base(filename)
	if safeName != filename || safeName == "." || safeName == ".." {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(persistence.Dir(), "projects", projectID, "uploads", "originals", safeName)
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
		} else {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	ext := strings.ToLower(filepath.Ext(safeName))
	contentType := "application/octet-stream"
	switch ext {
	case ".pdf":
		contentType = "application/pdf"
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	case ".svg":
		contentType = "image/svg+xml"
	case ".txt", ".md":
		contentType = "text/plain; charset=utf-8"
	case ".docx":
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeContent(w, r, safeName, stat.ModTime(), f)
}

// handleServeDoc serves a parsed document file (text) for viewing when the
// original binary is not available (backward compatibility with old uploads).
// GET /files/{project_id}/docs/{filename}
func (s *Server) handleServeDoc(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	filename := r.PathValue("filename")

	if !isValidProjectID(projectID) {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	safeName := filepath.Base(filename)
	if safeName != filename || safeName == "." || safeName == ".." {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(persistence.Dir(), "projects", projectID, "uploads", "docs", safeName)
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "file not found", http.StatusNotFound)
		} else {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeContent(w, r, safeName, stat.ModTime(), f)
}
