package document

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"path/filepath"
	"strings"

	// Register image decoders for format detection.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// ImageParser handles .png, .jpg, .jpeg, .gif, .webp, .svg.
// It validates the image and saves it to the session assets directory.
// SVG and WebP files skip image.DecodeConfig (stdlib lacks decoders);
// SVG files are validated as basic well-formed XML; WebP files use the
// file extension for format identification.
type ImageParser struct{}

func (p *ImageParser) Parse(file File, sessionDir string) (*ParsedDocument, error) {
	ext := strings.ToLower(filepath.Ext(file.Name))
	if len(ext) <= 1 {
		return nil, fmt.Errorf("invalid image: no extension in filename %q", file.Name)
	}

	var width, height int
	format := ext[1:]

	switch ext {
	case ".svg":
		if !isWellFormedXML(file.Data) {
			return nil, fmt.Errorf("invalid svg: not well-formed XML")
		}
	case ".webp":
		// stdlib lacks a webp decoder; use extension as format hint.
	default:
		cfg, imgFormat, err := image.DecodeConfig(bytes.NewReader(file.Data))
		if err != nil {
			return nil, fmt.Errorf("invalid image: %w", err)
		}
		width = cfg.Width
		height = cfg.Height
		format = imgFormat
	}

	savePath, err := writeUniqueFile(filepath.Join(sessionDir, "assets"), file.Name, file.Data)
	if err != nil {
		return nil, fmt.Errorf("write image: %w", err)
	}

	return &ParsedDocument{
		OriginalName: file.Name,
		Type:         "image",
		SavedPath:    savePath,
		Summary:      fmt.Sprintf("%s（%d×%d），可在 HTML 中引用", format, width, height),
		Width:        width,
		Height:       height,
		Format:       format,
	}, nil
}

func (p *ImageParser) SupportedExtensions() []string {
	return []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg"}
}

// isWellFormedXML checks if data is well-formed XML by attempting to decode it.
// Returns true if the decoder reaches EOF without encountering a parse error.
func isWellFormedXML(data []byte) bool {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		_, err := decoder.Token()
		if err == io.EOF {
			return true
		}
		if err != nil {
			return false
		}
	}
}
