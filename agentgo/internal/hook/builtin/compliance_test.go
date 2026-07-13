package builtin

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// isWhitelistedHost edge cases
// ---------------------------------------------------------------------------

func TestIsWhitelistedHost_GoogleFonts(t *testing.T) {
	if !isWhitelistedHost("https://fonts.googleapis.com/css2") {
		t.Error("fonts.googleapis.com should be whitelisted")
	}
}

func TestIsWhitelistedHost_Gstatic(t *testing.T) {
	if !isWhitelistedHost("https://fonts.gstatic.com/s/roboto.woff2") {
		t.Error("fonts.gstatic.com should be whitelisted")
	}
}

func TestIsWhitelistedHost_NonWhitelisted(t *testing.T) {
	if isWhitelistedHost("https://cdn.example.com/img.png") {
		t.Error("cdn.example.com should NOT be whitelisted")
	}
}

func TestIsWhitelistedHost_InvalidURL(t *testing.T) {
	if isWhitelistedHost("not-a-valid-url://") {
		t.Error("invalid URL should not be whitelisted")
	}
}

func TestIsWhitelistedHost_Empty(t *testing.T) {
	if isWhitelistedHost("") {
		t.Error("empty URL should not be whitelisted")
	}
}

// ---------------------------------------------------------------------------
// descSuffixLen edge cases
// ---------------------------------------------------------------------------

func TestDescSuffixLen_Standard(t *testing.T) {
	if n := descSuffixLen("1x"); n != 2 {
		t.Errorf("expected 2 for '1x', got %d", n)
	}
	if n := descSuffixLen("2x"); n != 2 {
		t.Errorf("expected 2 for '2x', got %d", n)
	}
	if n := descSuffixLen("100w"); n != 4 {
		t.Errorf("expected 4 for '100w', got %d", n)
	}
}

func TestDescSuffixLen_Decimal(t *testing.T) {
	if n := descSuffixLen("1.5x"); n != 4 {
		t.Errorf("expected 4 for '1.5x', got %d", n)
	}
}

func TestDescSuffixLen_NoDescriptor(t *testing.T) {
	if n := descSuffixLen("abc"); n != 0 {
		t.Errorf("expected 0 for 'abc', got %d", n)
	}
	if n := descSuffixLen(""); n != 0 {
		t.Errorf("expected 0 for empty string, got %d", n)
	}
	if n := descSuffixLen("x100"); n != 0 {
		t.Errorf("expected 0 for 'x100' (starts with letter), got %d", n)
	}
}

func TestDescSuffixLen_UpperCase(t *testing.T) {
	if n := descSuffixLen("2X"); n != 2 {
		t.Errorf("expected 2 for '2X', got %d", n)
	}
	if n := descSuffixLen("50W"); n != 3 {
		t.Errorf("expected 3 for '50W', got %d", n)
	}
}

// ---------------------------------------------------------------------------
// validateHTMLCompliance edge cases
// ---------------------------------------------------------------------------

func TestValidateHTMLCompliance_S4_MultipleInlineHandlers(t *testing.T) {
	// Only one issue even with multiple handlers (break on first match).
	html := `<div onclick="x()" onload="y()"></div>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "inline-handler" {
			found = true
			if !strings.Contains(iss.Message, "onclick") {
				t.Errorf("expected onclick in message, got: %s", iss.Message)
			}
		}
	}
	if !found {
		t.Error("expected inline-handler issue")
	}
}

func TestValidateHTMLCompliance_S6_CaseInsensitivePositionFixed(t *testing.T) {
	html := `<style>.a { POSITION: FIXED; }</style>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "position-fixed" {
			found = true
		}
	}
	if !found {
		t.Error("expected position-fixed issue for uppercase")
	}
}

func TestValidateHTMLCompliance_S8_MultipleStyleBlocks_OneUnclosed(t *testing.T) {
	html := `<style>.a { color: red; }</style><style>/* unclosed comment</style>`
	issues := validateHTMLCompliance(html)
	found := false
	for _, iss := range issues {
		if iss.Rule == "css-comment" {
			found = true
		}
	}
	if !found {
		t.Error("expected css-comment issue for unclosed CSS comment")
	}
}

func TestValidateHTMLCompliance_S9_UnclosedMultipleTags(t *testing.T) {
	html := `<div><div></div><span><span></span>`
	issues := validateHTMLCompliance(html)
	found := 0
	for _, iss := range issues {
		if iss.Rule == "unclosed-tag" {
			found++
		}
	}
	if found < 1 {
		t.Errorf("expected at least 1 unclosed tag issue, got %d", found)
	}
}

// ---------------------------------------------------------------------------
// checkExternalURLs edge cases
// ---------------------------------------------------------------------------

func TestCheckExternalURLs_HTTPSPoster(t *testing.T) {
	html := `<video poster="https://cdn.example.com/poster.jpg"></video>`
	issues := checkExternalURLs(html)
	if len(issues) == 0 {
		t.Fatal("expected external URL issue for poster")
	}
	if !strings.Contains(issues[0].Message, "poster") {
		t.Errorf("expected poster in message, got: %s", issues[0].Message)
	}
}

func TestCheckExternalURLs_FormAction(t *testing.T) {
	html := `<form action="https://external.com/submit"></form>`
	issues := checkExternalURLs(html)
	if len(issues) == 0 {
		t.Fatal("expected external URL issue for form action")
	}
}

func TestCheckExternalURLs_DataAttrs(t *testing.T) {
	html := `<img data-src="https://cdn.example.com/img.png">`
	issues := checkExternalURLs(html)
	if len(issues) == 0 {
		t.Fatal("expected external URL issue for data-src")
	}
}

func TestCheckExternalURLs_CiteAttr(t *testing.T) {
	html := `<blockquote cite="https://example.com/source"></blockquote>`
	issues := checkExternalURLs(html)
	if len(issues) == 0 {
		t.Fatal("expected external URL issue for cite")
	}
}

// ---------------------------------------------------------------------------
// checkFirstSlideActive edge cases
// ---------------------------------------------------------------------------

func TestCheckFirstSlideActive_SlideWithOtherClasses(t *testing.T) {
	html := `<div class="slide my-theme custom"></div>`
	issues := checkFirstSlideActive(html)
	if len(issues) == 0 {
		t.Fatal("expected issue for slide without active")
	}
	if issues[0].Rule != "slide-no-active" {
		t.Errorf("expected slide-no-active rule, got %q", issues[0].Rule)
	}
}

func TestCheckFirstSlideActive_SecondSlideHasActive(t *testing.T) {
	// Second slide has active but first doesn't — should flag first.
	html := `<div class="slide"></div><div class="slide active"></div>`
	issues := checkFirstSlideActive(html)
	if len(issues) == 0 {
		t.Fatal("expected issue for first slide missing active")
	}
}

// ---------------------------------------------------------------------------
// checkUnclosedTags edge cases
// ---------------------------------------------------------------------------

func TestCheckUnclosedTags_SelfClosingNotCounted(t *testing.T) {
	// Self-closing syntax not matched by openRe (which expects tag+space or tag+>).
	html := `<br/><hr/>`
	issues := checkUnclosedTags(html)
	// br and hr are not in the checked tag list.
	if len(issues) > 0 {
		t.Errorf("expected no issues for self-closing non-tracked tags, got: %v", issues)
	}
}

// ---------------------------------------------------------------------------
// splitSrcset edge cases
// ---------------------------------------------------------------------------

func TestSplitSrcset_DataURIWithDescriptor(t *testing.T) {
	urls := splitSrcset("data:image/png;base64,abc123 1x, /real.png 2x")
	if len(urls) < 1 {
		t.Fatal("expected at least 1 URL")
	}
	// The real.png should be present.
	found := false
	for _, u := range urls {
		if u == "/real.png" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected /real.png in results, got: %v", urls)
	}
}

func TestSplitSrcset_EmptyString(t *testing.T) {
	urls := splitSrcset("")
	if len(urls) != 0 {
		t.Errorf("expected 0 URLs for empty srcset, got %d", len(urls))
	}
}

// ---------------------------------------------------------------------------
// checkScriptInCustomCode edge cases
// ---------------------------------------------------------------------------

func TestCheckScriptInCustomCode_SingleQuoteAttr(t *testing.T) {
	html := `<div data-gjs-type='custom-code'><script>console.log(1)</script></div>`
	issues := checkScriptInCustomCode(html)
	if len(issues) > 0 {
		t.Errorf("expected no issues for script inside custom-code (single-quoted attr), got: %v", issues)
	}
}

func TestCheckScriptInCustomCode_FarFromCustomCode(t *testing.T) {
	// Script is more than 2000 chars away from custom-code marker.
	prefix := strings.Repeat("x", 2500)
	html := `<div data-gjs-type="custom-code">` + prefix + `<script>bad</script></div>`
	issues := checkScriptInCustomCode(html)
	if len(issues) == 0 {
		t.Fatal("expected violation when script is far from custom-code marker")
	}
}
