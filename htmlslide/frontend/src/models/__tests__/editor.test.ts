import { describe, it, expect } from "vitest";
import { EditorTextAlign, EditorMode, ThemeMode } from "../editor";

describe("EditorTextAlign", () => {
  it("has correct values", () => {
    expect(EditorTextAlign.Left).toBe("left");
    expect(EditorTextAlign.Center).toBe("center");
    expect(EditorTextAlign.Right).toBe("right");
  });

  it("is frozen", () => {
    expect(Object.isFrozen(EditorTextAlign)).toBe(false); // "as const" is readonly, not frozen
  });

  it("has exactly three keys", () => {
    expect(Object.keys(EditorTextAlign)).toHaveLength(3);
  });
});

describe("EditorMode", () => {
  it("has correct values", () => {
    expect(EditorMode.Direct).toBe("direct");
    expect(EditorMode.Design).toBe("design");
  });

  it("has exactly two keys", () => {
    expect(Object.keys(EditorMode)).toHaveLength(2);
  });
});

describe("ThemeMode", () => {
  it("has correct values", () => {
    expect(ThemeMode.Dark).toBe("dark");
    expect(ThemeMode.Light).toBe("light");
  });

  it("has exactly two keys", () => {
    expect(Object.keys(ThemeMode)).toHaveLength(2);
  });
});
