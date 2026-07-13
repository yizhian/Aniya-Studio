package builtin

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// ComplianceIssue represents a single finding from validateHTMLCompliance.
type ComplianceIssue struct {
	Rule     string
	Message  string
	Severity ComplianceSeverity
	Line     int
	Selector string
}
// validateHTMLCompliance runs all htmlchecker strict checks (S1-S10) against HTML content.
// Heuristic checks (H1-H4) are defined but not yet wired into any hook.
func validateHTMLCompliance(content string) []ComplianceIssue {
	var issues []ComplianceIssue

	// S1: DOCTYPE check.
	if len(content) < 15 || !strings.HasPrefix(strings.ToLower(content[:15]), "<!doctype html>") {
		issues = append(issues, ComplianceIssue{
			Severity: SeverityMustFix, Rule: "doctype",
			Message: "S1: Missing <!DOCTYPE html> declaration",
		})
	}

	// S2: html and body tags exist.
	hasHTML := strings.Contains(strings.ToLower(content), "<html")
	hasBody := strings.Contains(strings.ToLower(content), "<body")
	if !hasHTML {
		issues = append(issues, ComplianceIssue{
			Severity: SeverityMustFix, Rule: "no-html-tag",
			Message: "S2: Missing <html> tag",
		})
	}
	if !hasBody {
		issues = append(issues, ComplianceIssue{
			Severity: SeverityMustFix, Rule: "no-body-tag",
			Message: "S2: Missing <body> tag",
		})
	}

	// S3: No bare <table> tags.
	if strings.Contains(strings.ToLower(content), "<table") {
		issues = append(issues, ComplianceIssue{
			Severity: SeverityMustFix, Rule: "bare-table",
			Message: "S3: Contains bare <table> tag — use div grid for table layouts",
		})
	}

	// S4: No inline event handlers.
	for _, attr := range []string{"onclick", "onload", "onerror", "onmouseover", "onmouseout",
		"onkeydown", "onkeyup", "onsubmit", "onchange", "onfocus", "onblur", "oninput"} {
		if strings.Contains(strings.ToLower(content), " "+attr+"=") ||
			strings.Contains(strings.ToLower(content), " "+attr+" =") {
			issues = append(issues, ComplianceIssue{
				Severity: SeverityMustFix, Rule: "inline-handler",
				Message: "S4: Contains inline event handler " + attr + ", use addEventListener in custom-code scripts instead",
			})
			break // one finding covers all
		}
	}

	// S5: Script tags must be in custom-code blocks (tokenizer-based check).
	// This uses a simple tokenizer to find <script> tags and check ancestors.
	scriptIssues := checkScriptInCustomCode(content)
	issues = append(issues, scriptIssues...)

	// S6: No position: fixed.
	if posFixedRe.MatchString(content) {
		matches := posFixedRe.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			if len(m) > 0 {
				issues = append(issues, ComplianceIssue{
					Severity: SeverityMustFix, Rule: "position-fixed",
					Message: "S6: Contains position: fixed (" + strings.TrimSpace(m[0]) + ") — breaks editor layout",
				})
				break
			}
		}
	}

	// S7: No external http:// or https:// URLs.
	issues = append(issues, checkExternalURLs(content)...)

	// S8: Unclosed CSS comments.
	cssBlocks := extractStyleBlocks(content)
	for _, css := range cssBlocks {
		opens := strings.Count(css, "/*")
		closes := strings.Count(css, "*/")
		if opens != closes {
			issues = append(issues, ComplianceIssue{
				Severity: SeverityMustFix, Rule: "css-comment",
				Message: "S8: Unclosed CSS comment /* ... */ (will freeze the editor)",
			})
			break
		}
	}

	// S9: Unclosed tags (div, section, span, p, a, ul, ol, li, table).
	issues = append(issues, checkUnclosedTags(content)...)

	// S10: First .slide has class="active".
	issues = append(issues, checkFirstSlideActive(content)...)

	return issues
}

// posFixedRe matches position:fixed in CSS.
var posFixedRe = regexp.MustCompile(`(?i)position\s*:\s*fixed`)

// urlSemanticAttrs are HTML attributes that carry URLs.
var urlSemanticAttrs = map[string]bool{
	"src": true, "href": true, "srcset": true, "poster": true,
	"action": true, "formaction": true, "cite": true,
	"data-src": true, "data-href": true,
}

// checkScriptInCustomCode finds <script> tags not inside data-gjs-type="custom-code".
// Uses regex to find <script> tags and checks whether each is inside a custom-code block
// by looking for the nearest preceding data-gjs-type="custom-code" attribute.
func checkScriptInCustomCode(content string) []ComplianceIssue {
	var issues []ComplianceIssue
	scriptRe := regexp.MustCompile(`(?is)<script[\s>]`)
	matches := scriptRe.FindAllStringIndex(content, -1)
	for _, m := range matches {
		pos := m[0]
		// Look backwards for the nearest open-tag that could be custom-code.
		// Check if there's data-gjs-type="custom-code" between the last </ and this <script.
		// Simple heuristic: scan previous 2000 chars for custom-code marker.
		start := pos - 2000
		if start < 0 {
			start = 0
		}
		before := content[start:pos]
		// Check if custom-code opens and doesn't close before the script.
		lastOpen := strings.LastIndex(before, `data-gjs-type="custom-code"`)
		if lastOpen < 0 {
			lastOpen = strings.LastIndex(before, "data-gjs-type='custom-code'")
		}
		if lastOpen < 0 {
			issues = append(issues, ComplianceIssue{
				Severity: SeverityMustFix, Rule: "script-outside-custom-code",
				Message: "S5: <script> tag outside data-gjs-type=\"custom-code\" — GrapesJS will strip it",
			})
			break
		}
	}
	return issues
}

// s7WhitelistHosts are hostnames that serializePublishHtml injects into the
// published document. The Agent should NOT write these itself, but the check
// must not flag them post-serialization so the saved file can be re-imported.
var s7WhitelistHosts = map[string]bool{
	"fonts.googleapis.com": true,
	"fonts.gstatic.com":    true,
}

func isWhitelistedHost(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return s7WhitelistHosts[u.Host]
}

// checkExternalURLs checks HTML attributes and CSS for external URLs (S7).
func checkExternalURLs(content string) []ComplianceIssue {
	var issues []ComplianceIssue

	// Phase A: scan HTML attributes via tokenizer.
	z := html.NewTokenizer(strings.NewReader(content))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt == html.StartTagToken || tt == html.SelfClosingTagToken {
			for {
				key, val, more := z.TagAttr()
				attrName := strings.ToLower(string(key))
				if urlSemanticAttrs[attrName] {
					attrVal := string(val)
					// Check srcset specially (comma-separated URLs, skip data: segments).
					if attrName == "srcset" {
						for _, candidate := range splitSrcset(attrVal) {
							if (strings.HasPrefix(candidate, "http://") ||
								strings.HasPrefix(candidate, "https://")) &&
								!isWhitelistedHost(candidate) {
								issues = append(issues, ComplianceIssue{
									Severity: SeverityMustFix, Rule: "external-url",
									Message: "S7: srcset contains external URL: " + candidate,
								})
								break
							}
						}
					} else if (strings.HasPrefix(attrVal, "http://") ||
						strings.HasPrefix(attrVal, "https://")) && !isWhitelistedHost(attrVal) {
						issues = append(issues, ComplianceIssue{
							Severity: SeverityMustFix, Rule: "external-url",
							Message: "S7: " + attrName + " attribute contains external URL: " + attrVal,
						})
					}
				}
				if !more {
					break
				}
			}
		}
	}

	// Phase B: scan CSS for url() and @import with external URLs.
	cssBlocks := extractStyleBlocks(content)
	for _, css := range cssBlocks {
		// @import url("http...") or @import "http..."
		if matches := cssImportRe.FindAllStringSubmatch(css, -1); len(matches) > 0 {
			for _, m := range matches {
				issues = append(issues, ComplianceIssue{
					Severity: SeverityMustFix, Rule: "external-url",
					Message: "S7: CSS @import references external resource: " + strings.TrimSpace(m[0]),
				})
			}
			break
		}
		// url(http...) or url("http...")
		if matches := cssURLRe.FindAllStringSubmatch(css, -1); len(matches) > 0 {
			for _, m := range matches {
				url := strings.Trim(m[1], `"'`)
				if (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) && !isWhitelistedHost(url) {
					issues = append(issues, ComplianceIssue{
						Severity: SeverityMustFix, Rule: "external-url",
						Message: "S7: CSS url() references external resource: " + url,
					})
				}
			}
		}
	}

	return issues
}

var cssImportRe = regexp.MustCompile(`(?i)@import\s+(?:url\s*\(\s*)?["']?https?://[^"')};\s]+`)
var cssURLRe = regexp.MustCompile(`(?i)url\s*\(\s*["']?([^"')]+)["']?\s*\)`)

// splitSrcset splits a srcset attribute value into URL candidates,
// skipping data: URI segments that may contain commas.
func splitSrcset(srcset string) []string {
	var urls []string
	inDataURI := false
	start := 0
	for i := 0; i < len(srcset); i++ {
		if !inDataURI && i+5 <= len(srcset) && strings.HasPrefix(srcset[i:], "data:") {
			inDataURI = true
		}
		if inDataURI && (srcset[i] == ' ' || srcset[i] == '	') {
			// Look ahead: is the next non-whitespace a comma, end, or descriptor suffix?
			j := i + 1
			for j < len(srcset) && (srcset[j] == ' ' || srcset[j] == '	') {
				j++
			}
			if j >= len(srcset) || srcset[j] == ',' {
				inDataURI = false
			} else {
				// Check if followed by a descriptor suffix (1x, 2x, 100w, etc.)
				// which would end the data URI.
				rem := strings.TrimLeft(srcset[j:], " \t")
				descEnd := descSuffixLen(rem)
				if descEnd > 0 {
					after := strings.TrimLeft(rem[descEnd:], " \t")
					if after == "" || after[0] == ',' {
						inDataURI = false
					}
				}
			}
		}
		if !inDataURI && srcset[i] == ',' {
			candidate := strings.TrimSpace(srcset[start:i])
			// Strip descriptor suffix (1x, 2x, 100w, etc.)
			candidate = strings.TrimSpace(strings.TrimRight(
				strings.TrimRight(candidate, "0123456789.xwW"),
				" "))
			if candidate != "" {
				urls = append(urls, candidate)
			}
			start = i + 1
		}
	}
	// Last segment
	candidate := strings.TrimSpace(srcset[start:])
	candidate = strings.TrimSpace(strings.TrimRight(
		strings.TrimRight(candidate, "0123456789.xwW"),
		" "))
	if candidate != "" {
		urls = append(urls, candidate)
	}
	return urls
}

// descSuffixLen returns the length of a srcset descriptor suffix (e.g., "1x", "2x", "100w")
// at the start of s, or 0 if s doesn't start with a descriptor.
func descSuffixLen(s string) int {
	if len(s) == 0 {
		return 0
	}
	i := 0
	for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
		i++
	}
	if i == 0 {
		return 0
	}
	// Optional x/X/w/W suffix.
	if i < len(s) && (s[i] == 'x' || s[i] == 'X' || s[i] == 'w' || s[i] == 'W') {
		i++
	}
	return i
}

// checkUnclosedTags checks for unbalanced open/close tags (S9).
func checkUnclosedTags(content string) []ComplianceIssue {
	tags := []string{"div", "section", "span", "p", "a", "ul", "ol", "li", "table"}
	var issues []ComplianceIssue
	for _, tag := range tags {
		openRe := regexp.MustCompile(`(?i)<` + tag + `[\s>]`)
		closeRe := regexp.MustCompile(`(?i)</` + tag + `\s*>`)
		opens := len(openRe.FindAllString(content, -1))
		closes := len(closeRe.FindAllString(content, -1))
		if opens != closes {
			issues = append(issues, ComplianceIssue{
				Severity: SeverityMustFix, Rule: "unclosed-tag",
				Message: fmt.Sprintf("S9: <%s> tag unclosed (opened %d times, closed %d times)", tag, opens, closes),
			})
		}
	}
	return issues
}

// checkFirstSlideActive checks that the first .slide element has class="active" (S10).
func checkFirstSlideActive(content string) []ComplianceIssue {
	z := html.NewTokenizer(strings.NewReader(content))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return nil // no slide elements found at all
		}
		if tt == html.StartTagToken || tt == html.SelfClosingTagToken {
			for {
				key, val, more := z.TagAttr()
				if strings.ToLower(string(key)) == "class" {
					classes := strings.Fields(string(val))
					isSlide := false
					for _, c := range classes {
						if c == "slide" {
							isSlide = true
							break
						}
					}
					if isSlide {
						hasActive := false
						for _, c := range classes {
							if c == "active" {
								hasActive = true
								break
							}
						}
						if !hasActive {
							return []ComplianceIssue{{
								Severity: SeverityMustFix, Rule: "slide-no-active",
								Message: "S10: First .slide missing class=\"active\" — all slides will be invisible in the editor",
							}}
						}
						return nil // found first slide and it has active — ok
					}
				}
				if !more {
					break
				}
			}
		}
	}
}

// extractStyleBlocks returns the text content of all <style> elements in the HTML,
// including those inside data-gjs-type="custom-code" blocks.
func extractStyleBlocks(content string) []string {
	var blocks []string
	re := regexp.MustCompile(`(?is)<style[^>]*>(.*?)</style>`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 {
			blocks = append(blocks, m[1])
		}
	}
	return blocks
}
