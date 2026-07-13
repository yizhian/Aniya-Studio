import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { LoadingState, ErrorState, EmptyState } from "../StateRenderer";

describe("LoadingState", () => {
  it("renders a spinner", () => {
    render(<LoadingState />);
    const spinner = document.querySelector(".animate-spin");
    expect(spinner).toBeTruthy();
  });

  it("applies custom className", () => {
    const { container } = render(<LoadingState className="custom" />);
    expect(container.firstChild).toHaveClass("custom");
  });

  it("renders without className", () => {
    const { container } = render(<LoadingState />);
    expect(container.firstChild).toBeTruthy();
  });
});

describe("ErrorState", () => {
  it("renders the error message", () => {
    render(<ErrorState message="Something failed" onRetry={vi.fn()} retryLabel="Retry" />);
    expect(screen.getByText("Something failed")).toBeTruthy();
  });

  it("renders a retry button with label", () => {
    render(<ErrorState message="Failed" onRetry={vi.fn()} retryLabel="Try again" />);
    expect(screen.getByText("Try again")).toBeTruthy();
  });

  it("calls onRetry when button is clicked", () => {
    const onRetry = vi.fn();
    render(<ErrorState message="Failed" onRetry={onRetry} retryLabel="Retry" />);
    fireEvent.click(screen.getByText("Retry"));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("applies custom className", () => {
    const { container } = render(
      <ErrorState message="Failed" onRetry={vi.fn()} retryLabel="Retry" className="custom" />,
    );
    expect(container.firstChild).toHaveClass("custom");
  });
});

describe("EmptyState", () => {
  it("renders the title", () => {
    render(<EmptyState title="Nothing here" />);
    expect(screen.getByText("Nothing here")).toBeTruthy();
  });

  it("renders subtitle when provided", () => {
    render(<EmptyState title="Empty" subtitle="Add something to get started" />);
    expect(screen.getByText("Add something to get started")).toBeTruthy();
  });

  it("does not render subtitle when not provided", () => {
    const { container } = render(<EmptyState title="Empty" />);
    expect(container.querySelectorAll("p").length).toBe(1);
  });

  it("renders an icon when provided", () => {
    const Icon = ({ size, className }: any) => <svg data-testid="icon" width={size} className={className} />;
    render(<EmptyState title="Empty" icon={Icon} />);
    expect(screen.getByTestId("icon")).toBeTruthy();
  });

  it("does not break without an icon", () => {
    render(<EmptyState title="Empty" />);
    expect(screen.getByText("Empty")).toBeTruthy();
  });

  it("applies custom className", () => {
    const { container } = render(<EmptyState title="Empty" className="custom" />);
    expect(container.firstChild).toHaveClass("custom");
  });
});
