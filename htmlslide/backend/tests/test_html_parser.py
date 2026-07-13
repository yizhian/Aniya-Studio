from src.utils.html_parser import (
    count_slides,
    extract_style_content,
    extract_title_from_html,
    has_slide_elements,
    inject_base_tag,
    strip_style_tags,
)


class TestExtractStyleContent:
    def test_single_style_tag(self):
        html = "<style>body { color: red; }</style>"
        assert extract_style_content(html) == "body { color: red; }"

    def test_multiple_style_tags(self):
        html = "<style>a {}</style>\n<style>b {}</style>"
        result = extract_style_content(html)
        assert "a {}" in result
        assert "b {}" in result

    def test_no_style_tag(self):
        assert extract_style_content("<div>hello</div>") == ""

    def test_empty_input(self):
        assert extract_style_content("") == ""

    def test_style_with_attributes(self):
        html = '<style type="text/css">p { margin: 0; }</style>'
        assert extract_style_content(html) == "p { margin: 0; }"


class TestStripStyleTags:
    def test_removes_style(self):
        html = "<div>x</div><style>a {}</style>"
        assert "<style>" not in strip_style_tags(html)

    def test_no_style_tag(self):
        html = "<div>hello</div>"
        assert strip_style_tags(html) == html

    def test_empty_input(self):
        assert strip_style_tags("") == ""


class TestCountSlides:
    def test_single_slide(self):
        html = '<div class="slide">content</div>'
        assert count_slides(html) == 1

    def test_multiple_slides(self):
        html = '<div class="slide">a</div><div class="slide">b</div>'
        assert count_slides(html) == 2

    def test_no_slide(self):
        assert count_slides("<div>hello</div>") == 0

    def test_slide_in_class_list(self):
        html = '<div class="hero slide active">content</div>'
        assert count_slides(html) == 1

    def test_empty_input(self):
        assert count_slides("") == 0


class TestHasSlideElements:
    def test_has_slides(self):
        assert has_slide_elements('<div class="slide">x</div>')

    def test_no_slides(self):
        assert not has_slide_elements("<div>x</div>")

    def test_empty(self):
        assert not has_slide_elements("")


class TestExtractTitleFromHtml:
    def test_extracts_title(self):
        assert extract_title_from_html("<html><head><title>My Page</title></head></html>") == "My Page"

    def test_no_title_returns_empty(self):
        assert extract_title_from_html("<html><body></body></html>") == ""

    def test_empty_html(self):
        assert extract_title_from_html("") == ""

    def test_case_insensitive(self):
        html = "<HTML><HEAD><TITLE>Test</TITLE></HEAD></HTML>"
        assert extract_title_from_html(html) == "Test"


class TestInjectBaseTag:
    def test_injects_into_head(self):
        html = "<html><head></head><body></body></html>"
        result = inject_base_tag(html, "/api/v1/skills/foo/")
        assert '<base href="/api/v1/skills/foo/">' in result
        assert '<head><base href="/api/v1/skills/foo/">' in result

    def test_injects_into_uppercase_head(self):
        html = "<html><HEAD></HEAD><body></body></html>"
        result = inject_base_tag(html, "/api/v1/skills/bar/")
        assert '<base href="/api/v1/skills/bar/">' in result
        assert '<HEAD><base href="/api/v1/skills/bar/">' in result

    def test_no_head_prepends_base(self):
        html = "<html><body>content</body></html>"
        result = inject_base_tag(html, "/base/")
        assert result.startswith('<base href="/base/">')

    def test_preserves_existing_content(self):
        html = "<html><head><meta charset='utf-8'></head><body>x</body></html>"
        result = inject_base_tag(html, "/base/")
        assert "<meta charset='utf-8'>" in result
        assert "x" in result
