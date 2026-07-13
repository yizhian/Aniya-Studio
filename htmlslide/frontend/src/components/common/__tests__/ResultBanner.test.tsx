import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ResultBanner } from "../ResultBanner";

describe("ResultBanner", () => {
  it("renders nothing when result is null", () => {
    const { container } = render(<ResultBanner result={null} />);
    expect(container.innerHTML).toBe("");
  });

  it("renders success message", () => {
    const { container } = render(<ResultBanner result={{ ok: true, message: "All good" }} />);
    expect(screen.getByText("All good")).toBeTruthy();
    const svg = container.querySelector("svg");
    expect(svg).toBeTruthy();
  });

  it("renders error message", () => {
    const { container } = render(<ResultBanner result={{ ok: false, message: "Failed" }} />);
    expect(screen.getByText("Failed")).toBeTruthy();
    const svg = container.querySelector("svg");
    expect(svg).toBeTruthy();
  });
});
