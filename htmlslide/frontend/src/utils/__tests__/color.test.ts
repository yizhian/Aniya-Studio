import { describe, it, expect } from "vitest";
import { normalizeColorToHex, colorFromCssPaint, parseNumberish } from "../color";

describe("normalizeColorToHex", () => {
  it("returns default white for undefined", () => {
    expect(normalizeColorToHex()).toBe("#ffffff");
  });

  it("returns default white for empty string", () => {
    expect(normalizeColorToHex("")).toBe("#ffffff");
  });

  it("passes through valid 6-digit hex", () => {
    expect(normalizeColorToHex("#ff0000")).toBe("#ff0000");
  });

  it("passes through valid 6-digit hex (black)", () => {
    expect(normalizeColorToHex("#000000")).toBe("#000000");
  });

  it("expands 3-digit hex to 6-digit", () => {
    expect(normalizeColorToHex("#f00")).toBe("#ff0000");
  });

  it("expands 3-digit hex #abc", () => {
    expect(normalizeColorToHex("#abc")).toBe("#aabbcc");
  });

  it("expands 3-digit hex #fff to #ffffff", () => {
    expect(normalizeColorToHex("#fff")).toBe("#ffffff");
  });

  it("handles hex with leading/trailing spaces", () => {
    expect(normalizeColorToHex("  #ff0000  ")).toBe("#ff0000");
  });

  it("handles uppercase hex", () => {
    expect(normalizeColorToHex("#FF0000")).toBe("#ff0000");
  });

  it("converts rgb(r, g, b) to hex", () => {
    expect(normalizeColorToHex("rgb(255, 0, 0)")).toBe("#ff0000");
  });

  it("converts rgb with spaces", () => {
    expect(normalizeColorToHex("rgb(0, 128, 255)")).toBe("#0080ff");
  });

  it("converts rgba(r, g, b, a) to hex (ignoring alpha)", () => {
    expect(normalizeColorToHex("rgba(255, 0, 0, 0.5)")).toBe("#ff0000");
  });

  it("converts rgba with / syntax", () => {
    expect(normalizeColorToHex("rgb(255 0 0 / 0.5)")).toBe("#ff0000");
  });

  it("clamps out-of-range values", () => {
    expect(normalizeColorToHex("rgb(300, -10, 128)")).toBe("#ff0080");
  });

  it("returns default white for unparseable string", () => {
    expect(normalizeColorToHex("not-a-color")).toBe("#ffffff");
  });

  it("returns default white for empty rgb", () => {
    expect(normalizeColorToHex("rgb()")).toBe("#ffffff");
  });

  it("returns default white for partial rgb", () => {
    expect(normalizeColorToHex("rgb(255)")).toBe("#ffffff");
  });
});

describe("colorFromCssPaint", () => {
  it("returns undefined for undefined input", () => {
    expect(colorFromCssPaint()).toBeUndefined();
  });

  it("returns undefined for 'none'", () => {
    expect(colorFromCssPaint("none")).toBeUndefined();
  });

  it("returns undefined for empty string", () => {
    expect(colorFromCssPaint("")).toBeUndefined();
  });

  it("extracts hex color", () => {
    expect(colorFromCssPaint("#ff0000")).toBe("#ff0000");
  });

  it("extracts hex from multi-value string", () => {
    expect(colorFromCssPaint("linear-gradient(#fff, #000)")).toBe("#000");
  });

  it("extracts last color from multi-value string", () => {
    expect(colorFromCssPaint("#aaa #bbb #ccc")).toBe("#ccc");
  });

  it("extracts rgb from string", () => {
    expect(colorFromCssPaint("rgb(255, 0, 0)")).toBe("rgb(255, 0, 0)");
  });

  it("extracts rgba from string", () => {
    expect(colorFromCssPaint("rgba(0, 0, 0, 0.5)")).toBe("rgba(0, 0, 0, 0.5)");
  });
});

describe("parseNumberish", () => {
  it("parses a number directly", () => {
    expect(parseNumberish(42)).toBe(42);
  });

  it("parses a numeric string", () => {
    expect(parseNumberish("42")).toBe(42);
  });

  it("parses a float string", () => {
    expect(parseNumberish("3.14")).toBe(3.14);
  });

  it("handles zero", () => {
    expect(parseNumberish(0)).toBe(0);
    expect(parseNumberish("0")).toBe(0);
  });

  it("handles negative numbers", () => {
    expect(parseNumberish(-5)).toBe(-5);
  });

  it("returns null for non-numeric string", () => {
    expect(parseNumberish("hello")).toBeNull();
  });

  it("returns null for empty string", () => {
    expect(parseNumberish("")).toBeNull();
  });

  it("returns null for NaN", () => {
    expect(parseNumberish(NaN)).toBeNull();
  });

  it("returns null for Infinity", () => {
    expect(parseNumberish(Infinity)).toBeNull();
  });

  it("parses trimmed string", () => {
    expect(parseNumberish("  10px  ")).toBe(10);
  });

  it("parses negative numeric string", () => {
    expect(parseNumberish("-10")).toBe(-10);
  });

  it("handles px suffix by using parseFloat", () => {
    // parseFloat("10px") = 10
    expect(parseNumberish("10px")).toBe(10);
  });
});
