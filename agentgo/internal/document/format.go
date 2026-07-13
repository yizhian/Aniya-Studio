package document

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxSummaryChars    = 4000
	maxWriteRetries    = 100
)

// FormatForUserMessage generates the summary_text field for the /upload API response
// (backward compatibility only — the frontend no longer injects it into user messages).
// Returns an empty string if docs is empty.
func FormatForUserMessage(docs []ParsedDocument) string {
	if len(docs) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "【本次上传的参考文档】")

	for _, doc := range docs {
		if doc.Error != "" {
			lines = append(lines, fmt.Sprintf("- ⚠️ `%s` 解析失败：%s", doc.OriginalName, doc.Error))
		} else {
			path := doc.SavedPath
			if path == "" {
				path = doc.OriginalName
			}
			lines = append(lines, fmt.Sprintf("- `%s` → %s", path, doc.Summary))
		}
	}

	lines = append(lines, "\n如需查看文档详细内容，请使用 read_file 工具读取对应路径。")

	result := strings.Join(lines, "\n")
	if len(result) > maxSummaryChars {
		// Truncate at a safe UTF-8 boundary.
		cut := maxSummaryChars
		for cut > 0 && result[cut]&0xC0 == 0x80 {
			cut--
		}
		result = result[:cut]
	}
	return result
}

// FormatUploadedFilesForSystemPrompt builds the {{uploaded_files}} block injected
// into the system prompt. It reads file metadata from upload_meta.json and produces
// a path-based file manifest that tells the agent to read_file each document.
// Returns an empty string when meta has no files.
func FormatUploadedFilesForSystemPrompt(meta *UploadMeta) string {
	if meta == nil || len(meta.Files) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "【项目参考文档】")
	lines = append(lines, "以下文档已上传至项目。你必须先用 read_file 工具完整阅读所有参考文档，理解用户需求后再开始工作：")
	lines = append(lines, "")

	for _, f := range meta.Files {
		path := f.SavedPathRel
		if path == "" {
			path = f.SavedName
		}
		detail := f.Type
		if f.Pages > 0 {
			detail += fmt.Sprintf("，%d 页", f.Pages)
		}
		if f.CharCount > 0 {
			detail += fmt.Sprintf("，%d 字符", f.CharCount)
		}
		if f.Width > 0 && f.Height > 0 {
			detail += fmt.Sprintf("，%d×%d", f.Width, f.Height)
		}
		if f.Error != "" {
			detail += fmt.Sprintf("（解析问题：%s）", f.Error)
		}
		lines = append(lines, fmt.Sprintf("- `%s` — %s", path, detail))
	}

	return strings.Join(lines, "\n")
}

var newlineRe = regexp.MustCompile(`\n{3,}`)

// cleanText normalizes line endings and collapses excessive blank lines.
func cleanText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	raw = newlineRe.ReplaceAllString(raw, "\n\n")
	return strings.TrimSpace(raw)
}

// summarizeText creates a one-line summary for a text-based document.
func summarizeText(filename string, charCount int) string {
	if strings.HasSuffix(strings.ToLower(filename), ".md") {
		return fmt.Sprintf("Markdown 文档，约 %d 字符", charCount)
	}
	return fmt.Sprintf("文本文档，约 %d 字符", charCount)
}

// writeUniqueFile atomically writes data to a uniquely-named file under dir.
// Uses O_EXCL to prevent silent overwrites from concurrent writers. If name
// already exists, appends _2, _3, etc. up to maxWriteRetries attempts.
func writeUniqueFile(dir, name string, data []byte) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	path, err := tryWriteExclusive(dir, name, data)
	if err == nil {
		return path, nil
	}
	if !os.IsExist(err) {
		return "", err
	}

	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for i := 2; i <= maxWriteRetries; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		path, err = tryWriteExclusive(dir, candidate, data)
		if err == nil {
			return path, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("writeUniqueFile: exceeded %d retries for %s", maxWriteRetries, name)
}

func tryWriteExclusive(dir, name string, data []byte) (string, error) {
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return path, nil
}
