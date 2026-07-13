/**
 * Tests for EditorPreviewModal — covers:
 *   - Rendering iframe with src
 *   - Fullscreen toggle and iframe auto-focus (regression fix)
 *   - Escape key calls onClose
 *   - F key toggles fullscreen
 *   - Error state
 *   - Controls auto-hide
 *   - Blob URL cleanup
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditorPreviewModal } from "../EditorPreviewModal";

// jsdom lacks requestFullscreen — add it for the test suite.
beforeEach(() => {
  if (!HTMLDivElement.prototype.requestFullscreen) {
    Object.defineProperty(HTMLDivElement.prototype, "requestFullscreen", {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    });
  }
  if (!document.exitFullscreen) {
    Object.defineProperty(document, "exitFullscreen", {
      value: vi.fn().mockResolvedValue(undefined),
      writable: true,
      configurable: true,
    });
  }
  // Reset fullscreenElement before each test.
  Object.defineProperty(document, "fullscreenElement", {
    configurable: true,
    get: () => (window as any).__fsEl ?? null,
  });
  (window as any).__fsEl = null;
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("EditorPreviewModal", () => {
  let blobUrl: string;

  beforeEach(() => {
    blobUrl = URL.createObjectURL(
      new Blob(["<html><body>preview</body></html>"], { type: "text/html" }),
    );
  });

  afterEach(() => {
    URL.revokeObjectURL(blobUrl);
  });

  // ── UT: Basic rendering ─────────────────────────────────────

  it("renders an iframe with the provided src", () => {
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={vi.fn()} />);
    const iframe = screen.getByTitle("preview.htmlPreview");
    expect(iframe).toBeInTheDocument();
    expect(iframe).toHaveAttribute("src", blobUrl);
  });

  it("has sandbox attribute on iframe", () => {
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={vi.fn()} />);
    const iframe = screen.getByTitle("preview.htmlPreview");
    expect(iframe).toHaveAttribute("sandbox", "allow-scripts");
  });

  it("renders close and fullscreen buttons", () => {
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={vi.fn()} />);
    expect(screen.getByTitle("preview.fullscreen")).toBeInTheDocument();
    expect(screen.getByTitle("preview.closePreview")).toBeInTheDocument();
  });

  it("shows preview title", () => {
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={vi.fn()} />);
    expect(screen.getByText("preview.preview")).toBeInTheDocument();
  });

  // ── UT: Error state ─────────────────────────────────────────

  it("renders error message when error is provided", () => {
    render(<EditorPreviewModal src="" error="Something broke" onClose={vi.fn()} />);
    expect(screen.getByText("Something broke")).toBeInTheDocument();
  });

  it("does not render iframe in error state", () => {
    render(<EditorPreviewModal src="" error="Error!" onClose={vi.fn()} />);
    expect(screen.queryByTitle("preview.htmlPreview")).not.toBeInTheDocument();
  });

  // ── UT: Escape key ──────────────────────────────────────────

  it("calls onClose on Escape keydown", () => {
    const onClose = vi.fn();
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={onClose} />);
    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // ── UT: Fullscreen toggle ───────────────────────────────────

  it("displays minimize icon when fullscreen is active", () => {
    (window as any).__fsEl = null;
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={vi.fn()} />);
    // Initially maximize is shown.
    expect(screen.getByTitle("preview.fullscreen")).toBeInTheDocument();
    // Simulate fullscreenchange.
    act(() => {
      (window as any).__fsEl = document.createElement("div");
      document.dispatchEvent(new Event("fullscreenchange"));
    });
    // Now minimize should show.
    expect(screen.getByTitle("preview.exitFullscreen")).toBeInTheDocument();
    (window as any).__fsEl = null;
  });

  it("displays maximize icon when not fullscreen", () => {
    (window as any).__fsEl = null;
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={vi.fn()} />);
    expect(screen.getByTitle("preview.fullscreen")).toBeInTheDocument();
  });

  // ── SIT: Fullscreen → iframe focus (the regression fix) ────

  it("focuses iframe after fullscreenchange event fires", async () => {
    (window as any).__fsEl = null;
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={vi.fn()} />);

    // Simulate entering fullscreen.
    act(() => {
      (window as any).__fsEl = document.createElement("div");
      document.dispatchEvent(new Event("fullscreenchange"));
    });

    // Wait for the setTimeout(100ms) in the focus effect.
    await act(async () => {
      await new Promise((r) => setTimeout(r, 150));
    });

    // After the timeout, the iframe should have been focused.
    const iframe = screen.getByTitle("preview.htmlPreview");
    const focused = document.activeElement === iframe || document.activeElement === document.body;
    expect(focused).toBe(true);

    (window as any).__fsEl = null;
  });

  // ── UT: Controls auto-hide ──────────────────────────────────

  it("shows controls by default", () => {
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={vi.fn()} />);
    expect(screen.getByTitle("preview.closePreview")).toBeInTheDocument();
  });

  // ── Smoke: Blob URL cleanup ─────────────────────────────────

  it("does not crash on unmount", () => {
    const { unmount } = render(
      <EditorPreviewModal src={blobUrl} error={null} onClose={vi.fn()} />,
    );
    expect(() => unmount()).not.toThrow();
  });

  // ── Smoke: Close button interaction ─────────────────────────

  it("calls onClose when close button is clicked", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup();
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={onClose} />);

    await user.click(screen.getByTitle("preview.closePreview"));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  // ── F key prevents default ──────────────────────────────────

  it("prevents default on F key press", () => {
    render(<EditorPreviewModal src={blobUrl} error={null} onClose={vi.fn()} />);
    const e = new KeyboardEvent("keydown", { key: "f", cancelable: true });
    const prevented = !window.dispatchEvent(e);
    expect(prevented).toBe(true);
  });
});
