import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { EditorBlockPanel } from "../EditorBlockPanel";

// Mock the useEditor hook
vi.mock("../../../hooks/useEditor", () => ({
  useEditor: vi.fn(),
}));

import { useEditor } from "../../../hooks/useEditor";

function mockBlockManager(getResult: unknown) {
  return {
    get: vi.fn(() => getResult),
    startDrag: vi.fn(),
  };
}

function mockEditorWithBlocks(blocks: Record<string, unknown>) {
  return {
    BlockManager: {
      get: vi.fn((id: string) => blocks[id] ?? null),
      startDrag: vi.fn(),
      endDrag: vi.fn(),
    },
  };
}

describe("EditorBlockPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ── Rendering ──

  it("renders 6 block buttons", () => {
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(null);
    render(<EditorBlockPanel />);

    const buttons = screen.getAllByRole("button").filter((btn) =>
      btn.className.includes("cursor-grab")
    );
    expect(buttons).toHaveLength(6);
  });

  it("each button has draggable attribute", () => {
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(null);
    render(<EditorBlockPanel />);

    const buttons = screen.getAllByRole("button").filter((btn) =>
      btn.className.includes("cursor-grab")
    );
    buttons.forEach((btn) => {
      expect(btn).toHaveAttribute("draggable", "true");
    });
  });

  it("renders correct drag-related CSS classes", () => {
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(null);
    render(<EditorBlockPanel />);

    const buttons = screen.getAllByRole("button").filter((btn) =>
      btn.className.includes("cursor-grab")
    );
    buttons.forEach((btn) => {
      expect(btn.className).toContain("cursor-grab");
      expect(btn.className).toContain("active:cursor-grabbing");
    });
  });

  // ── Drag error states ──

  it("shows error when editor is null on drag start", () => {
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(null);
    render(<EditorBlockPanel />);

    const firstBtn = screen.getAllByRole("button")[0];
    const dragEvent = new Event("dragstart", { bubbles: true }) as unknown as React.DragEvent;
    Object.defineProperty(dragEvent, "dataTransfer", {
      value: { effectAllowed: "", setData: vi.fn() },
    });
    Object.defineProperty(dragEvent, "preventDefault", { value: vi.fn() });

    fireEvent(firstBtn, dragEvent);

    expect(screen.getByText("Editor not initialized")).toBeInTheDocument();
  });

  it("shows error when block is not found", () => {
    const editor = mockEditorWithBlocks({});
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(editor);
    render(<EditorBlockPanel />);

    const firstBtn = screen.getAllByRole("button")[0];
    const dragEvent = new Event("dragstart", { bubbles: true }) as unknown as React.DragEvent;
    Object.defineProperty(dragEvent, "dataTransfer", {
      value: { effectAllowed: "", setData: vi.fn() },
    });
    Object.defineProperty(dragEvent, "preventDefault", { value: vi.fn() });

    fireEvent(firstBtn, dragEvent);

    expect(screen.getByText("Component not found")).toBeInTheDocument();
  });

  it("clears error on drag end", () => {
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(null);
    render(<EditorBlockPanel />);

    const firstBtn = screen.getAllByRole("button")[0];

    // Trigger drag start → show error
    const dragStartEvent = new Event("dragstart", { bubbles: true }) as unknown as React.DragEvent;
    Object.defineProperty(dragStartEvent, "dataTransfer", {
      value: { effectAllowed: "", setData: vi.fn() },
    });
    Object.defineProperty(dragStartEvent, "preventDefault", { value: vi.fn() });
    fireEvent(firstBtn, dragStartEvent);
    expect(screen.getByText("Editor not initialized")).toBeInTheDocument();

    // Trigger drag end → error dismissed
    const dragEndEvent = new Event("dragend", { bubbles: true }) as unknown as React.DragEvent;
    fireEvent(firstBtn, dragEndEvent);
    expect(screen.queryByText("Editor not initialized")).not.toBeInTheDocument();
  });

  // ── Successful drag start ──

  it("calls BlockManager.startDrag on successful drag", () => {
    const block = { id: "div-container", label: "blocks.container" };
    const editor = mockEditorWithBlocks({ "div-container": block });
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(editor);
    render(<EditorBlockPanel />);

    const firstBtn = screen.getAllByRole("button")[0];
    const nativeEvent = new Event("dragstart", { bubbles: true });
    const dragEvent = new Event("dragstart", { bubbles: true }) as unknown as React.DragEvent;
    Object.defineProperty(dragEvent, "dataTransfer", {
      value: {
        effectAllowed: "",
        setData: vi.fn(),
      },
    });
    Object.defineProperty(dragEvent, "nativeEvent", { value: nativeEvent });

    fireEvent(firstBtn, dragEvent);

    expect(editor.BlockManager.startDrag).toHaveBeenCalledWith(block, nativeEvent);
  });

  it("sets text/plain data transfer on successful drag", () => {
    const block = { id: "paragraph", label: "blocks.paragraph" };
    const editor = mockEditorWithBlocks({ paragraph: block });
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(editor);
    render(<EditorBlockPanel />);

    // Second button is "blocks.paragraph"
    const buttons = screen.getAllByRole("button");
    const paragraphBtn = buttons[1];

    const setDataMock = vi.fn();
    const dragEvent = new Event("dragstart", { bubbles: true }) as unknown as React.DragEvent;
    Object.defineProperty(dragEvent, "dataTransfer", {
      value: { effectAllowed: "", setData: setDataMock },
    });
    Object.defineProperty(dragEvent, "nativeEvent", {
      value: new Event("dragstart", { bubbles: true }),
    });

    fireEvent(paragraphBtn, dragEvent);

    expect(setDataMock).toHaveBeenCalledWith("text/plain", "paragraph");
  });

  it("calls BlockManager.endDrag on drag end", () => {
    const editor = mockEditorWithBlocks({});
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(editor);
    render(<EditorBlockPanel />);

    const firstBtn = screen.getAllByRole("button")[0];
    fireEvent(firstBtn, new Event("dragend", { bubbles: true }));

    expect(editor.BlockManager.endDrag).toHaveBeenCalledWith(true);
  });

  // ── Tooltip presence ──

  it("renders tooltip labels for each block", () => {
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(null);
    render(<EditorBlockPanel />);

    expect(screen.getByText("blocks.container")).toBeInTheDocument();
    expect(screen.getByText("blocks.paragraph")).toBeInTheDocument();
    expect(screen.getByText("blocks.image")).toBeInTheDocument();
    expect(screen.getByText("blocks.video")).toBeInTheDocument();
    expect(screen.getByText("blocks.chart")).toBeInTheDocument();
    expect(screen.getByText("blocks.divider")).toBeInTheDocument();
  });

  it("tooltips are hidden by default (opacity-0)", () => {
    (useEditor as unknown as ReturnType<typeof vi.fn>).mockReturnValue(null);
    render(<EditorBlockPanel />);

    const tooltip = screen.getByText("blocks.container");
    // The tooltip has opacity-0 class for hidden state.
    expect(tooltip.className).toContain("opacity-0");
  });
});
