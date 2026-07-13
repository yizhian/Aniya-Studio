import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AnimatedPanel } from "../AnimatedPanel";

vi.mock("motion/react", () => ({
  AnimatePresence: ({ children }: any) => children,
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  },
}));

describe("AnimatedPanel", () => {
  const onClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when closed", () => {
    const { container } = render(
      <AnimatedPanel isOpen={false} onClose={onClose}>
        <span>panel content</span>
      </AnimatedPanel>,
    );
    expect(container.textContent).toBe("");
  });

  it("renders children when open", () => {
    render(
      <AnimatedPanel isOpen onClose={onClose}>
        <span>panel content</span>
      </AnimatedPanel>,
    );
    expect(screen.getByText("panel content")).toBeTruthy();
  });

  it("calls onClose when backdrop is clicked", () => {
    render(
      <AnimatedPanel isOpen onClose={onClose}>
        <span>content</span>
      </AnimatedPanel>,
    );
    // Rendered via Portal into document.body, not the RTL container.
    const backdrop = document.body.querySelector('[class*="bg-black"]');
    expect(backdrop).toBeTruthy();
    fireEvent.click(backdrop!);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("calls onClose on Escape key by default", () => {
    render(
      <AnimatedPanel isOpen onClose={onClose}>
        <span>content</span>
      </AnimatedPanel>,
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("does not call onClose on Escape when closeOnEscape is false", () => {
    render(
      <AnimatedPanel isOpen onClose={onClose} closeOnEscape={false}>
        <span>content</span>
      </AnimatedPanel>,
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("does not call onClose on non-Escape key", () => {
    render(
      <AnimatedPanel isOpen onClose={onClose}>
        <span>content</span>
      </AnimatedPanel>,
    );
    fireEvent.keyDown(document, { key: "Enter" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("cleans up Escape listener on close", () => {
    const { rerender } = render(
      <AnimatedPanel isOpen onClose={onClose}>
        <span>content</span>
      </AnimatedPanel>,
    );
    rerender(
      <AnimatedPanel isOpen={false} onClose={onClose}>
        <span>content</span>
      </AnimatedPanel>,
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("renders left panel by default", () => {
    render(
      <AnimatedPanel isOpen onClose={onClose}>
        <span>content</span>
      </AnimatedPanel>,
    );
    const panel = document.body.querySelector('[class*="left-0"]');
    expect(panel).toBeTruthy();
  });

  it("renders right panel when position is right", () => {
    render(
      <AnimatedPanel isOpen onClose={onClose} position="right">
        <span>content</span>
      </AnimatedPanel>,
    );
    const panel = document.body.querySelector('[class*="right-0"]');
    expect(panel).toBeTruthy();
  });
});
