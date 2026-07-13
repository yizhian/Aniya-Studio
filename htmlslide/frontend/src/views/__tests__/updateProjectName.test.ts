import { describe, it, expect } from "vitest";

// Mirror of the regex-based extraction logic from EditorView.tsx:updateProjectNameFromHtml.
// Uses /s (dotAll) flag so `.` matches newlines in multiline tags.
function extractTitleFromHtml(html: string): string | undefined {
  const titleMatch = html.match(/<title[^>]*>(.*?)<\/title>/is);
  const title = titleMatch?.[1]?.trim();
  if (title) return title;

  const h1Match = html.match(/<h1[^>]*>(.*?)<\/h1>/is);
  return h1Match?.[1]?.trim() || undefined;
}

describe("extractTitleFromHtml", () => {
  it("extracts text from <title> tag", () => {
    const html = "<html><head><title>My Project Name</title></head><body></body></html>";
    expect(extractTitleFromHtml(html)).toBe("My Project Name");
  });

  it("trims whitespace from title", () => {
    const html = "<title>  Spaced Title  </title>";
    expect(extractTitleFromHtml(html)).toBe("Spaced Title");
  });

  it("handles title with attributes", () => {
    const html = '<title lang="en">English Title</title>';
    expect(extractTitleFromHtml(html)).toBe("English Title");
  });

  it("falls back to <h1> when no <title>", () => {
    const html = "<body><h1>Main Heading</h1><p>content</p></body>";
    expect(extractTitleFromHtml(html)).toBe("Main Heading");
  });

  it("prefers <title> over <h1> when both present", () => {
    const html = "<title>Page Title</title><h1>Section Heading</h1>";
    expect(extractTitleFromHtml(html)).toBe("Page Title");
  });

  it("falls back to h1 with attributes", () => {
    const html = '<h1 class="main-heading">Styled Heading</h1>';
    expect(extractTitleFromHtml(html)).toBe("Styled Heading");
  });

  it("trims h1 whitespace", () => {
    const html = "<h1>\n  Multi-line Heading  \n</h1>";
    expect(extractTitleFromHtml(html)).toBe("Multi-line Heading");
  });

  it("returns undefined when neither title nor h1 exists", () => {
    const html = "<body><p>Just a paragraph</p></body>";
    expect(extractTitleFromHtml(html)).toBeUndefined();
  });

  it("returns undefined for empty HTML", () => {
    expect(extractTitleFromHtml("")).toBeUndefined();
  });

  it("returns undefined when title is empty", () => {
    expect(extractTitleFromHtml("<title></title>")).toBeUndefined();
  });

  it("returns undefined when h1 is empty", () => {
    const html = "<body><h1>  </h1></body>";
    expect(extractTitleFromHtml(html)).toBeUndefined();
  });

  it("handles AI-generated GrapesJS HTML without title/h1 tags", () => {
    const html = `<section class="slide"><div class="content"><p>Welcome to my presentation</p></div></section>`;
    expect(extractTitleFromHtml(html)).toBeUndefined();
  });

  it("extracts h1 from inline element content", () => {
    const html = "<h1><span>Nested</span> Heading</h1>";
    expect(extractTitleFromHtml(html)).toBe("<span>Nested</span> Heading");
  });
});
