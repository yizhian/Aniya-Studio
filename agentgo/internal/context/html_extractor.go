package context

import (
	"io"
	"os"
	"strings"

	"golang.org/x/net/html"
)

// ExtractDesignSnapshot parses an HTML file and extracts structural metadata.
func ExtractDesignSnapshot(htmlPath string) (*DesignSnapshot, error) {
	f, err := os.Open(htmlPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := &snapshotBuilder{}
	z := html.NewTokenizer(f)

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return s.finalize(htmlPath)
			}
			return nil, z.Err()

		case html.StartTagToken, html.SelfClosingTagToken:
			tagName, hasAttr := z.TagName()
			attrs := make(map[string]string)
			if hasAttr {
				for {
					key, val, more := z.TagAttr()
					attrs[string(key)] = string(val)
					if !more {
						break
					}
				}
			}
			s.handleStartTag(string(tagName), attrs)

		case html.EndTagToken:
			tagName, _ := z.TagName()
			s.handleEndTag(string(tagName))

		case html.TextToken:
			s.handleText(string(z.Text()))
		}
	}
}

type snapshotBuilder struct {
	title      string
	headings   []string
	sections   []HTMLSection
	classes    map[string]struct{}
	palette    map[string]string
	fonts      map[string]string // family -> source
	inTitle    bool
	inStyle    bool
	styleBuf   strings.Builder
	inHeading  string // tag name of current heading (h1-h6), empty if not in a heading
	headingBuf strings.Builder
}

func (s *snapshotBuilder) handleStartTag(tag string, attrs map[string]string) {
	class := attrs["class"]
	id := attrs["id"]

	if class != "" {
		for _, c := range strings.Fields(class) {
			if s.classes == nil {
				s.classes = make(map[string]struct{})
			}
			s.classes[c] = struct{}{}
		}
	}

	switch tag {
	case "title":
		s.inTitle = true

	case "style":
		s.inStyle = true
		s.styleBuf.Reset()

	case "link":
		rel := strings.ToLower(attrs["rel"])
		if rel == "stylesheet" || rel == "preload" {
			href := attrs["href"]
			if strings.Contains(href, "fonts.googleapis.com") {
				s.extractGoogleFont(href)
			}
		}

	case "h1", "h2", "h3", "h4", "h5", "h6":
		s.inHeading = tag
		s.headingBuf.Reset()

	case "section", "div", "article", "main", "header", "footer":
		if class != "" || id != "" {
			sec := HTMLSection{Tag: tag, Class: class, ID: id}
			s.sections = append(s.sections, sec)
		}
	}
}

func (s *snapshotBuilder) handleEndTag(tag string) {
	switch tag {
	case "style":
		s.inStyle = false

	case "h1", "h2", "h3", "h4", "h5", "h6":
		if s.inHeading != "" {
			text := strings.TrimSpace(s.headingBuf.String())
			if text != "" {
				s.headings = append(s.headings, text)
				// Also attach heading text to the most recent matching section.
				for i := len(s.sections) - 1; i >= 0; i-- {
					if s.sections[i].Heading == "" {
						s.sections[i].Heading = text
						break
					}
				}
			}
			s.inHeading = ""
		}

	case "title":
		s.inTitle = false
	}
}

func (s *snapshotBuilder) handleText(text string) {
	if s.inTitle {
		text = strings.TrimSpace(text)
		if text != "" {
			s.title = text
		}
		s.inTitle = false
		return
	}

	if s.inStyle {
		s.styleBuf.WriteString(text)
		return
	}

	if s.inHeading != "" {
		s.headingBuf.WriteString(text)
	}
}

func (s *snapshotBuilder) finalize(htmlPath string) (*DesignSnapshot, error) {
	if s.styleBuf.Len() > 0 {
		s.parseInlineStyles(s.styleBuf.String())
	}

	fi, err := os.Stat(htmlPath)
	if err != nil {
		return nil, err
	}

	classes := make([]string, 0, len(s.classes))
	for c := range s.classes {
		classes = append(classes, c)
	}

	fonts := make([]FontInfo, 0, len(s.fonts))
	for family, source := range s.fonts {
		fonts = append(fonts, FontInfo{Family: family, Source: source})
	}

	return &DesignSnapshot{
		Title:         s.title,
		SlideCount:    countSlides(s.sections),
		SlideHeadings: s.headings,
		CSSClasses:    classes,
		Fonts:         fonts,
		ColorPalette:  s.palette,
		Sections:      s.sections,
		FileSizeBytes: fi.Size(),
	}, nil
}

func (s *snapshotBuilder) extractGoogleFont(href string) {
	family := ""
	if idx := strings.Index(href, "family="); idx >= 0 {
		family = href[idx+7:]
		if sep := strings.IndexAny(family, "&:|"); sep >= 0 {
			family = family[:sep]
		}
		family = strings.ReplaceAll(family, "+", " ")
	}
	if family == "" {
		return
	}
	if s.fonts == nil {
		s.fonts = make(map[string]string)
	}
	s.fonts[family] = "Google Fonts"
}

func (s *snapshotBuilder) parseInlineStyles(css string) {
	for _, line := range strings.Split(css, ";") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "--") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		prop := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if s.palette == nil {
			s.palette = make(map[string]string)
		}
		s.palette[prop] = val
	}

	for _, line := range strings.Split(css, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "@import") && strings.Contains(line, "font") {
			if start := strings.Index(line, "url("); start >= 0 {
				url := line[start+4:]
				if end := strings.Index(url, ")"); end >= 0 {
					url = url[:end]
					url = strings.Trim(url, "'\"")
					if strings.Contains(url, "fonts.googleapis.com") {
						s.extractGoogleFont(url)
					}
				}
			}
		}
	}
}

func countSlides(sections []HTMLSection) int {
	n := 0
	for _, sec := range sections {
		c := strings.ToLower(sec.Class)
		if strings.Contains(c, "slide") || strings.Contains(c, "slide-") {
			n++
		}
	}
	return n
}
