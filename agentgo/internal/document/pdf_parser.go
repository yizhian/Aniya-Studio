package document

import (
	"bytes"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// PDFParser extracts text from .pdf files.
type PDFParser struct{}

func (p *PDFParser) Parse(file File, sessionDir string) (*ParsedDocument, error) {
	reader, err := pdf.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)))
	if err != nil {
		return nil, fmt.Errorf("invalid pdf: %w", err)
	}

	var buf strings.Builder
	numPages := reader.NumPage()

	for pageIdx := 1; pageIdx <= numPages; pageIdx++ {
		page := reader.Page(pageIdx)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			log.Printf(`{"ts":"%s","type":"document:parse_warn","file":"%s","parser":"pdf","page":%d,"error":"%s"}`,
				time.Now().Format(time.RFC3339), file.Name, pageIdx, err.Error())
			continue
		}
		// Add page separator between pages.
		if pageIdx > 1 {
			buf.WriteString("\n\n")
		}
		buf.WriteString(text)
	}

	text := buf.String()
	cleaned := cleanText(text)

	outputName := file.Name + ".txt"
	savePath, err := writeUniqueFile(filepath.Join(sessionDir, "docs"), outputName, []byte(cleaned))
	if err != nil {
		return nil, fmt.Errorf("write pdf text: %w", err)
	}

	return &ParsedDocument{
		OriginalName: file.Name,
		Type:         "pdf",
		SavedPath:    savePath,
		Summary:      fmt.Sprintf("PDF 文档（%d 页），已提取文本内容（%d 字符）", numPages, len(cleaned)),
		CharCount:    len(cleaned),
		Pages:        numPages,
	}, nil
}

func (p *PDFParser) SupportedExtensions() []string {
	return []string{".pdf"}
}
