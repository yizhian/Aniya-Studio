package document

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// TextParser handles .md, .txt, and content-detected text files.
type TextParser struct{}

func (p *TextParser) Parse(file File, sessionDir string) (*ParsedDocument, error) {
	text, err := decodeToUTF8(file.Data)
	if err != nil {
		// If all charset detection fails, fall back to raw bytes as UTF-8.
		text = string(file.Data)
	}
	cleaned := cleanText(text)
	if cleaned == "" {
		return &ParsedDocument{
			OriginalName: file.Name,
			Type:         "text",
			CharCount:    0,
			Summary:      "空文件，无内容",
		}, nil
	}

	savePath, err := writeUniqueFile(filepath.Join(sessionDir, "docs"), file.Name, []byte(cleaned))
	if err != nil {
		return nil, fmt.Errorf("write text: %w", err)
	}

	docType := "text"
	if strings.HasSuffix(strings.ToLower(file.Name), ".md") {
		docType = "markdown"
	}

	return &ParsedDocument{
		OriginalName: file.Name,
		Type:         docType,
		SavedPath:    savePath,
		Summary:      summarizeText(file.Name, len(cleaned)),
		CharCount:    len(cleaned),
	}, nil
}

func (p *TextParser) SupportedExtensions() []string {
	return []string{".md", ".txt"}
}

// decodeToUTF8 detects the encoding of raw bytes and converts to UTF-8.
//
// Strategy:
//  1. Valid UTF-8 → return as-is.
//  2. HTML5 detection → if the result contains non-Latin1 characters, trust it.
//     (HTML5 detection is reliable for BOM-marked files and long samples.)
//  3. Chinese encodings (GB18030/GBK) → for short CJK samples where HTML5
//     detection misidentifies the encoding as a Latin-1 variant.
//  4. If HTML5 produced a valid Latin-1-only result, use that.
//  5. Otherwise return raw bytes.
func decodeToUTF8(data []byte) (string, error) {
	if utf8.Valid(data) {
		return string(data), nil
	}

	// Try HTML5 encoding detection first.
	enc, name, _ := charset.DetermineEncoding(data, "")

	// html5Result holds the HTML5-decoded text if it's valid UTF-8 but
	// contains only Latin-1 characters (possibly a misdetection).
	var html5Result string
	var html5OK bool

	if name != "" && !strings.EqualFold(name, "utf-8") {
		result, err := decodeWith(data, enc)
		if err == nil && utf8.ValidString(result) {
			if looksLikeDecodedText(result) {
				return result, nil
			}
			html5Result = result
			html5OK = true
		}
	}

	// Fallback: try Chinese encodings for short samples.
	// GB18030 is a superset of GBK and GB2312.
	for _, chEnc := range []encoding.Encoding{
		simplifiedchinese.GB18030,
		simplifiedchinese.GBK,
	} {
		result, err := decodeWith(data, chEnc)
		if err == nil && utf8.ValidString(result) && looksLikeDecodedText(result) {
			return result, nil
		}
	}

	// If HTML5 gave a valid Latin-1 result, return it.
	if html5OK {
		return html5Result, nil
	}

	return string(data), fmt.Errorf("charset detection failed, using raw bytes")
}

// looksLikeDecodedText returns true if the string appears to be successfully
// decoded (contains non-ASCII characters not from the Latin-1 block).
func looksLikeDecodedText(s string) bool {
	for _, r := range s {
		if r > 0xFF {
			return true
		}
	}
	return false
}

func decodeWith(data []byte, enc encoding.Encoding) (string, error) {
	reader := enc.NewDecoder().Reader(bytes.NewReader(data))
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
