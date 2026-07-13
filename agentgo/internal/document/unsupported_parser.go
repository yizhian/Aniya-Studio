package document

import "fmt"

// UnsupportedParser handles unknown file types gracefully without blocking the pipeline.
type UnsupportedParser struct{}

func (p *UnsupportedParser) Parse(file File, _ string) (*ParsedDocument, error) {
	return &ParsedDocument{
		OriginalName: file.Name,
		Type:         "unsupported",
		Summary:      fmt.Sprintf("文件格式暂不支持（本轮支持 Markdown、Word、TXT、图片）"),
	}, nil
}

func (p *UnsupportedParser) SupportedExtensions() []string {
	return nil
}
