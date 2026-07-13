import { describe, it, expect, vi } from "vitest";
import { detectComponentType } from "../editor/componentDetection";

function mockEditor(overrides: { selected?: any; tagName?: string } | null) {
  if (overrides === null) return null;
  const getEl = overrides.tagName
    ? vi.fn(() => ({ tagName: overrides.tagName }))
    : vi.fn(() => null);
  const getSelected = overrides.selected === undefined
    ? vi.fn(() => ({ getEl }))
    : vi.fn(() => overrides.selected);
  return { getSelected } as any;
}

describe("detectComponentType", () => {
  it("returns null when editor is null", () => {
    expect(detectComponentType(null)).toBeNull();
  });

  it("returns null when nothing is selected", () => {
    const editor = mockEditor({ selected: null });
    expect(detectComponentType(editor)).toBeNull();
  });

  it("returns null when selected component has no DOM element", () => {
    const editor = mockEditor({ tagName: undefined });
    expect(detectComponentType(editor)).toBeNull();
  });

  it("returns 'image' for an img element", () => {
    const editor = mockEditor({ tagName: "IMG" });
    expect(detectComponentType(editor)).toBe("image");
  });

  it("returns 'image' for a lowercase img tag", () => {
    const editor = mockEditor({ tagName: "img" });
    expect(detectComponentType(editor)).toBe("image");
  });

  it("returns 'text' for a div element", () => {
    const editor = mockEditor({ tagName: "DIV" });
    expect(detectComponentType(editor)).toBe("text");
  });

  it("returns 'text' for a span element", () => {
    const editor = mockEditor({ tagName: "span" });
    expect(detectComponentType(editor)).toBe("text");
  });

  it("returns 'text' for any non-img element", () => {
    const editor = mockEditor({ tagName: "section" });
    expect(detectComponentType(editor)).toBe("text");
  });
});
