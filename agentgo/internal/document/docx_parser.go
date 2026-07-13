package document

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const (
	maxDocxDecompressedSize = 100 << 20 // 100 MB
	maxXMLNestDepth         = 256
)

// DocxParser extracts text from .docx files (ZIP containing word/document.xml).
type DocxParser struct{}

func (p *DocxParser) Parse(file File, sessionDir string) (*ParsedDocument, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)))
	if err != nil {
		return nil, fmt.Errorf("invalid docx: %w", err)
	}

	var docXML *zip.File
	for _, f := range zipReader.File {
		if f.Name == "word/document.xml" {
			docXML = f
			break
		}
	}
	if docXML == nil {
		return nil, fmt.Errorf("invalid docx: word/document.xml not found")
	}

	rc, err := docXML.Open()
	if err != nil {
		return nil, fmt.Errorf("open docx xml: %w", err)
	}
	defer rc.Close()

	xmlBytes, err := io.ReadAll(io.LimitReader(rc, maxDocxDecompressedSize))
	if err != nil {
		return nil, fmt.Errorf("read docx xml: %w", err)
	}

	var probe [1]byte
	n, probeErr := rc.Read(probe[:])
	if n > 0 || probeErr == nil {
		return nil, fmt.Errorf("docx too large: word/document.xml exceeds %d bytes", maxDocxDecompressedSize)
	}
	_ = probeErr

	text, err := extractTextFromDocxXML(xmlBytes)
	if err != nil {
		return nil, fmt.Errorf("parse docx xml: %w", err)
	}
	cleaned := cleanText(text)

	outputName := file.Name + ".txt"
	savePath, err := writeUniqueFile(filepath.Join(sessionDir, "docs"), outputName, []byte(cleaned))
	if err != nil {
		return nil, fmt.Errorf("write docx text: %w", err)
	}

	return &ParsedDocument{
		OriginalName: file.Name,
		Type:         "docx",
		SavedPath:    savePath,
		Summary:      fmt.Sprintf("Word 文档，已提取文本内容（%d 字符）", len(cleaned)),
		CharCount:    len(cleaned),
	}, nil
}

func (p *DocxParser) SupportedExtensions() []string {
	return []string{".docx"}
}

// extractTextFromDocxXML extracts text from <w:t> elements in the document XML.
// Returns an error if the XML nesting depth exceeds maxXMLNestDepth.
func extractTextFromDocxXML(data []byte) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var buf strings.Builder
	inText := false
	inParagraph := false
	depth := 0

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > maxXMLNestDepth {
				return "", fmt.Errorf("xml nesting depth exceeds limit (%d)", maxXMLNestDepth)
			}
			switch se.Name.Local {
			case "p":
				inParagraph = true
			case "t":
				inText = true
			}
		case xml.EndElement:
			depth--
			switch se.Name.Local {
			case "p":
				if inParagraph {
					buf.WriteByte('\n')
				}
				inParagraph = false
			case "t":
				inText = false
			}
		case xml.CharData:
			if inText {
				buf.Write(se)
			}
		}
	}

	return buf.String(), nil
}
