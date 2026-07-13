package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentgo/internal/document"
	"agentgo/internal/persistence"
	"agentgo/internal/util"
)

type uploadResponse struct {
	UploadID    string                `json:"upload_id"`
	SessionID   string                `json:"session_id"`
	Files       []uploadResponseFile  `json:"files"`
	SummaryText string                `json:"summary_text"`
	ParseStats  document.ParseStats   `json:"parse_stats"`
}

type uploadResponseFile struct {
	OriginalName    string `json:"original_name"`
	Type            string `json:"type"`
	SavedPathRel    string `json:"saved_path_rel,omitempty"`
	OriginalPathRel string `json:"original_path_rel,omitempty"`
	CharCount       int    `json:"char_count,omitempty"`
	Pages           int    `json:"pages,omitempty"`
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
	Format          string `json:"format,omitempty"`
	Summary         string `json:"summary,omitempty"`
	Error           string `json:"error,omitempty"`
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	const maxUploadSize = 50 << 20
	const maxFiles = 10
	const maxTextSize = 2 << 20
	const maxPDFSize = 20 << 20
	const maxDocxSize = 10 << 20
	const maxImageSize = 10 << 20

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "文件过大或格式错误（最大 50MB）", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	projectID := strings.TrimSpace(r.FormValue("project_id"))
	if projectID == "" {
		http.Error(w, "project_id is required", http.StatusBadRequest)
		return
	}
	if !isValidProjectID(projectID) {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	projDir := filepath.Join(persistence.Dir(), "projects", projectID)
	if _, err := os.Stat(projDir); os.IsNotExist(err) {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}

	uploadID := "upl_" + util.RandomHex(12)
	targetDir := filepath.Join(projDir, "uploads")

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, "no files uploaded", http.StatusBadRequest)
		return
	}
	if len(files) > maxFiles {
		http.Error(w, fmt.Sprintf("最多 %d 个文件", maxFiles), http.StatusBadRequest)
		return
	}

	docs := make([]document.File, 0, len(files))
	seenNames := make(map[string]int)
	totalBytes := int64(0)

	for _, fh := range files {
		origName := fh.Filename

		origName = filepath.Base(origName)
		if origName == "." || origName == ".." || origName == "" {
			continue
		}
		origName = resolveNameCollision(origName, seenNames)

		if !document.IsSupportedExtension(fh.Filename) {
			if !document.IsSupportedExtension(origName) {
				docs = append(docs, document.File{
					Name: origName,
					Data: nil,
					Size: fh.Size,
				})
				continue
			}
		}

		sizeLimit := int64(maxTextSize)
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		switch ext {
		case ".pdf":
			sizeLimit = maxPDFSize
		case ".docx":
			sizeLimit = maxDocxSize
		case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
			sizeLimit = maxImageSize
		}

		if fh.Size > sizeLimit {
			log.Printf("upload: %s exceeds size limit (%d > %d)", origName, fh.Size, sizeLimit)
			continue
		}

		f, err := fh.Open()
		if err != nil {
			log.Printf("upload: open %s: %v", origName, err)
			continue
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			log.Printf("upload: read %s: %v", origName, err)
			continue
		}

		totalBytes += int64(len(data))
		docs = append(docs, document.File{
			Name: origName,
			Data: data,
			Size: fh.Size,
		})
	}

	if len(docs) == 0 {
		http.Error(w, "没有有效的文件上传", http.StatusBadRequest)
		return
	}

	log.Printf(`{"ts":"%s","event":"upload_start","upload_id":"%s","project_id":"%s","file_count":%d,"total_bytes":%d}`,
		time.Now().Format(time.RFC3339), uploadID, projectID, len(docs), totalBytes)

	results, stats := s.documentPipeline.ParseAll(r.Context(), docs, targetDir, uploadID)

	dataByName := make(map[string][]byte, len(docs))
	for _, d := range docs {
		if len(d.Data) > 0 {
			dataByName[d.Name] = d.Data
		}
	}
	originalsDir := filepath.Join(projDir, "uploads", "originals")
	_ = os.MkdirAll(originalsDir, 0755)
	for i, doc := range results {
		if doc.Error != "" || doc.Type == "unsupported" || doc.Type == "error" {
			continue
		}
		origData, ok := dataByName[doc.OriginalName]
		if !ok || len(origData) == 0 {
			continue
		}
		_ = os.WriteFile(filepath.Join(originalsDir, doc.OriginalName), origData, 0644)
		results[i].OriginalPath = filepath.Join(originalsDir, doc.OriginalName)
	}

	meta := buildUploadMeta(uploadID, results)
	existing := readUploadMeta(targetDir)
	if existing != nil {
		meta = mergeUploadMeta(*existing, meta)
	}
	writeUploadMeta(targetDir, meta)

	log.Printf(`{"ts":"%s","event":"upload_complete","upload_id":"%s","succeeded":%d,"errors":%d,"total_duration_ms":%d}`,
		time.Now().Format(time.RFC3339), uploadID, stats.Succeeded, stats.Errors, stats.TotalDurationMs)

	summaryText := document.FormatForUserMessage(results)
	resp := uploadResponse{
		UploadID:    uploadID,
		SessionID:   uploadID,
		Files:       buildResponseFiles(results),
		SummaryText: summaryText,
		ParseStats:  stats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// -- upload_meta.json helpers ------------------------------------------------

func buildUploadMeta(uploadID string, docs []document.ParsedDocument) document.UploadMeta {
	now := time.Now().UTC().Format(time.RFC3339)
	meta := document.UploadMeta{
		UploadID:  uploadID,
		CreatedAt: now,
		Files:     make([]document.UploadMetaFile, 0, len(docs)),
	}
	for _, d := range docs {
		meta.Files = append(meta.Files, document.UploadMetaFile{
			OriginalName:    d.OriginalName,
			SavedName:       filepath.Base(d.SavedPath),
			SavedPathRel:    document.BuildSavedPathRel(d),
			OriginalPathRel: document.BuildOriginalPathRel(d.OriginalName),
			Type:            d.Type,
			CharCount:       d.CharCount,
			Pages:           d.Pages,
			Width:           d.Width,
			Height:          d.Height,
			Format:          d.Format,
			Error:           d.Error,
		})
	}
	return meta
}

func mergeUploadMeta(existing, incoming document.UploadMeta) document.UploadMeta {
	merged := existing
	merged.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	byName := make(map[string]int)
	for i, mf := range merged.Files {
		byName[mf.SavedName] = i
	}

	for _, mf := range incoming.Files {
		if idx, ok := byName[mf.SavedName]; ok {
			merged.Files[idx] = mf
		} else {
			merged.Files = append(merged.Files, mf)
		}
	}
	return merged
}

func readUploadMeta(targetDir string) *document.UploadMeta {
	path := filepath.Join(targetDir, "upload_meta.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var meta document.UploadMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

func writeUploadMeta(targetDir string, meta document.UploadMeta) {
	_ = os.MkdirAll(targetDir, 0755)
	path := filepath.Join(targetDir, "upload_meta.json")
	data, _ := json.MarshalIndent(meta, "", "  ")
	_ = os.WriteFile(path, data, 0644)
}

func buildResponseFiles(docs []document.ParsedDocument) []uploadResponseFile {
	files := make([]uploadResponseFile, 0, len(docs))
	for _, d := range docs {
		files = append(files, uploadResponseFile{
			OriginalName:    d.OriginalName,
			Type:            d.Type,
			SavedPathRel:    document.BuildSavedPathRel(d),
			OriginalPathRel: document.BuildOriginalPathRel(d.OriginalName),
			CharCount:       d.CharCount,
			Pages:           d.Pages,
			Width:           d.Width,
			Height:          d.Height,
			Format:          d.Format,
			Summary:         d.Summary,
			Error:           d.Error,
		})
	}
	return files
}

// handleGetUploads returns the upload metadata for a project.
// GET /projects/{project_id}/uploads
func (s *Server) handleGetUploads(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project_id")
	if !isValidProjectID(projectID) {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	metaPath := filepath.Join(persistence.Dir(), "projects", projectID, "uploads", "upload_meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write([]byte(`{"files":[]}`))
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)
}

// -- validation helpers ------------------------------------------------------

func isValidProjectID(id string) bool {
	if strings.Contains(id, "..") || strings.ContainsAny(id, "/\\") {
		return false
	}
	projectsRoot := filepath.Join(persistence.Dir(), "projects") + string(filepath.Separator)
	projDir := filepath.Join(persistence.Dir(), "projects", id)
	return strings.HasPrefix(filepath.Clean(projDir), projectsRoot)
}
// resolveNameCollision returns a unique filename within seenNames by appending
// an incrementing suffix until no collision exists.
func resolveNameCollision(name string, seenNames map[string]int) string {
	if _, exists := seenNames[name]; !exists {
		seenNames[name] = 1
		return name
	}

	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, exists := seenNames[candidate]; !exists {
			seenNames[candidate] = 1
			return candidate
		}
	}
}
