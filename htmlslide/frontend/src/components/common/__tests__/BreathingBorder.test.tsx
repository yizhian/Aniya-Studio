import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { BreathingBorder } from "../BreathingBorder";

describe("BreathingBorder", () => {
  it("renders children", () => {
    render(
      <BreathingBorder active={false}>
        <span>Content</span>
      </BreathingBorder>,
    );
    expect(screen.getByText("Content")).toBeTruthy();
  });

  it("renders active overlay elements when active is true", () => {
    const { container } = render(
      <BreathingBorder active>
        <span>Content</span>
      </BreathingBorder>,
    );
    // The active state adds two overlay divs before children
    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper.children.length).toBeGreaterThanOrEqual(3); // 2 overlays + children
    expect(screen.getByText("Content")).toBeTruthy();
  });

  it("renders only children when active is false", () => {
    const { container } = render(
      <BreathingBorder active={false}>
        <span>Content</span>
      </BreathingBorder>,
    );
    const wrapper = container.firstChild as HTMLElement;
    // Only the children, no overlay divs
    expect(wrapper.querySelectorAll('[style*="animation"]').length).toBe(0);
    expect(screen.getByText("Content")).toBeTruthy();
  });
});
