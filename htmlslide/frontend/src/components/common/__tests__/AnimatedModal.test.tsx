import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AnimatedModal } from "../AnimatedModal";

vi.mock("motion/react", () => ({
  AnimatePresence: ({ children }: any) => children,
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  },
}));

describe("AnimatedModal", () => {
  const onClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when closed", () => {
    const { container } = render(
      <AnimatedModal isOpen={false} onClose={onClose}>
        <span>modal content</span>
      </AnimatedModal>,
    );
    expect(container.textContent).toBe("");
  });

  it("renders children when open", () => {
    render(
      <AnimatedModal isOpen onClose={onClose}>
        <span>modal content</span>
      </AnimatedModal>,
    );
    expect(screen.getByText("modal content")).toBeTruthy();
  });

  it("renders title when provided", () => {
    render(
      <AnimatedModal isOpen onClose={onClose} title="Settings">
        <span>modal content</span>
      </AnimatedModal>,
    );
    expect(screen.getByText("Settings")).toBeTruthy();
  });

  it("does not render title when not provided", () => {
    render(
      <AnimatedModal isOpen onClose={onClose}>
        <span>modal content</span>
      </AnimatedModal>,
    );
    expect(screen.queryByRole("heading")).toBeNull();
  });

  it("calls onClose when backdrop is clicked", () => {
    render(
      <AnimatedModal isOpen onClose={onClose}>
        <span>content</span>
      </AnimatedModal>,
    );
    // Rendered via Portal into document.body, not the RTL container.
    const backdrop = document.body.querySelector('[class*="bg-black"]');
    expect(backdrop).toBeTruthy();
    fireEvent.click(backdrop!);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("calls onClose on Escape key by default", () => {
    render(
      <AnimatedModal isOpen onClose={onClose}>
        <span>content</span>
      </AnimatedModal>,
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("does not call onClose on Escape when closeOnEscape is false", () => {
    render(
      <AnimatedModal isOpen onClose={onClose} closeOnEscape={false}>
        <span>content</span>
      </AnimatedModal>,
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("calls onClose when X button is clicked", () => {
    render(
      <AnimatedModal isOpen onClose={onClose} title="Settings">
        <span>content</span>
      </AnimatedModal>,
    );
    // X button in the title bar
    const closeBtn = screen.getByRole("button");
    fireEvent.click(closeBtn);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("cleans up Escape listener on close", () => {
    const { rerender } = render(
      <AnimatedModal isOpen onClose={onClose}>
        <span>content</span>
      </AnimatedModal>,
    );
    rerender(
      <AnimatedModal isOpen={false} onClose={onClose}>
        <span>content</span>
      </AnimatedModal>,
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });
});
