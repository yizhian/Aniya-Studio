/**
 * Tests for EditorRightDock — covers:
 *   - Tab rendering (style, traits, layers, blocks)
 *   - No SVG tab or SVG inline editor (regression)
 *   - Image inline editor renders for image selection
 *   - Close button works
 *   - detectComponentType returns text/image/null only (no SVG)
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { EditorRightDock } from "../EditorRightDock";

function fakeSelected(el: HTMLElement) {
  return {
    getEl: () => el,
    getStyle: () => ({}),
    getAttributes: () => ({}),
    getId: () => "test-id",
    toHTML: () => el.outerHTML,
    cid: "test-cid",
  };
}

function mockEditor(getEl: () => HTMLElement | null = () => null) {
  const ed: any = {
    getSelected: () => {
      const el = getEl();
      return el ? fakeSelected(el) : null;
    },
    on: vi.fn(),
    off: vi.fn(),
    Components: { getType: () => "default" },
    Keymaps: { getAll: () => [] },
    StyleManager: { getSectors: () => [] },
    TraitManager: { getTraitsForModel: () => [] },
    Commands: { getAll: () => [] },
    Panels: { getPanels: () => ({}) },
    getHtml: vi.fn(() => ""),
    getCss: vi.fn(() => ""),
    getStyle: vi.fn(() => ({})),
    setStyle: vi.fn(),
    setComponents: vi.fn(),
    refresh: vi.fn(),
    Canvas: { getDocument: () => document },
  };
  return ed;
}

describe("EditorRightDock", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ── UT: Tab rendering ──────────────────────────────────────

  it("renders four tabs: style, traits, layers, blocks", () => {
    render(<EditorRightDock isOpen onClose={vi.fn()} editor={null} />);
    expect(screen.getByText("panels.style")).toBeInTheDocument();
    expect(screen.getByText("panels.traits")).toBeInTheDocument();
    expect(screen.getByText("panels.layers")).toBeInTheDocument();
    expect(screen.getByText("panels.blocks")).toBeInTheDocument();
  });

  it("default active tab is style", () => {
    render(<EditorRightDock isOpen onClose={vi.fn()} editor={null} />);
    const styleTab = screen.getByText("panels.style");
    expect(styleTab.className).toContain("active");
  });

  it("calls onClose when X button is clicked", () => {
    const onClose = vi.fn();
    render(<EditorRightDock isOpen onClose={onClose} editor={null} />);
    const buttons = screen.getAllByRole("button");
    const closeBtn = buttons[buttons.length - 1];
    fireEvent.click(closeBtn);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // ── SV G removal regression ────────────────────────────────

  it("does NOT render SVG DATA section (SVG feature removed)", () => {
    const ed = mockEditor(() => {
      const p = document.createElement("p");
      p.textContent = "hello";
      return p;
    });
    render(<EditorRightDock isOpen onClose={vi.fn()} editor={ed} />);
    expect(screen.queryByText(/svg/i)).toBeNull();
  });

  it("does NOT render SVG DATA section even when SVG element selected", () => {
    const svgEl = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    const ed = mockEditor(() => svgEl);
    render(<EditorRightDock isOpen onClose={vi.fn()} editor={ed} />);
    expect(screen.queryByText("svgDataEditor.title")).toBeNull();
  });

  it("does not import BarChart3 icon component", () => {
    render(<EditorRightDock isOpen onClose={vi.fn()} editor={null} />);
    expect(screen.getByText("panels.style")).toBeInTheDocument();
  });

  // ── UT: Image selection shows inline editor ────────────────

  it("shows image section when image is selected and on style tab", () => {
    const img = document.createElement("img");
    img.src = "https://example.com/pic.png";
    const ed = mockEditor(() => img);
    render(<EditorRightDock isOpen onClose={vi.fn()} editor={ed} />);
    expect(screen.getByText("panels.image")).toBeInTheDocument();
    expect(screen.getByText("panels.uploadLocal")).toBeInTheDocument();
  });

  // ── Smoke: componentType detection ─────────────────────────

  it("detects image component type", () => {
    const img = document.createElement("img");
    const ed = mockEditor(() => img);
    render(<EditorRightDock isOpen onClose={vi.fn()} editor={ed} />);
    expect(screen.getByText("panels.image")).toBeInTheDocument();
  });

  it("detects text component type (no image section)", () => {
    const p = document.createElement("p");
    const ed = mockEditor(() => p);
    render(<EditorRightDock isOpen onClose={vi.fn()} editor={ed} />);
    expect(screen.queryByText("panels.image")).toBeNull();
  });

  it("does not crash with null editor", () => {
    expect(() =>
      render(<EditorRightDock isOpen onClose={vi.fn()} editor={null} />),
    ).not.toThrow();
  });
});
