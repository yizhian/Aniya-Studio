import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useBlockDrag } from "../useBlockDrag";

function makeFakeDragEvent(): React.DragEvent {
  return {
    preventDefault: vi.fn(),
    dataTransfer: {
      effectAllowed: "",
      setData: vi.fn(),
      getData: vi.fn(),
    },
    nativeEvent: {} as DragEvent,
  } as any;
}

function makeMockEditor(blockExists = true) {
  return {
    BlockManager: {
      get: vi.fn((_id: string) => (blockExists ? { id: "block-1" } : null)),
      startDrag: vi.fn(),
      endDrag: vi.fn(),
    },
  } as any;
}

describe("useBlockDrag", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("returns initial state with null dragError", () => {
    const editor = makeMockEditor();
    const { result } = renderHook(() => useBlockDrag({ editor }));
    expect(result.current.dragError).toBeNull();
    expect(typeof result.current.handleDragStart).toBe("function");
    expect(typeof result.current.handleDragEnd).toBe("function");
  });

  it("sets dragError when editor is null", () => {
    const onError = vi.fn();
    const { result } = renderHook(() => useBlockDrag({ editor: null, onError }));

    const e = makeFakeDragEvent();
    act(() => {
      result.current.handleDragStart("block-1", e);
    });

    expect(result.current.dragError).toBe("Editor not initialized");
    expect(e.preventDefault).toHaveBeenCalled();
    expect(onError).toHaveBeenCalledWith("Editor not initialized");
  });

  it("sets dragError when block is not found", () => {
    const onError = vi.fn();
    const editor = makeMockEditor(false);
    const { result } = renderHook(() => useBlockDrag({ editor, onError }));

    const e = makeFakeDragEvent();
    act(() => {
      result.current.handleDragStart("missing-block", e);
    });

    expect(result.current.dragError).toBe("Component not found");
    expect(e.preventDefault).toHaveBeenCalled();
    expect(onError).toHaveBeenCalledWith("Component not found");
  });

  it("starts drag successfully and sets dataTransfer", () => {
    const editor = makeMockEditor(true);
    const { result } = renderHook(() => useBlockDrag({ editor }));

    const e = makeFakeDragEvent();
    act(() => {
      result.current.handleDragStart("block-1", e);
    });

    expect(result.current.dragError).toBeNull();
    expect(e.dataTransfer.effectAllowed).toBe("copy");
    expect(e.dataTransfer.setData).toHaveBeenCalledWith("text/plain", "block-1");
    expect(editor.BlockManager.startDrag).toHaveBeenCalledWith(
      { id: "block-1" },
      e.nativeEvent,
    );
  });

  it("catches startDrag errors", () => {
    const onError = vi.fn();
    const editor = makeMockEditor(true);
    editor.BlockManager.startDrag = vi.fn(() => {
      throw new Error("boom");
    });
    const { result } = renderHook(() => useBlockDrag({ editor, onError }));

    const e = makeFakeDragEvent();
    act(() => {
      result.current.handleDragStart("block-1", e);
    });

    expect(result.current.dragError).toBe("Drag failed");
    expect(e.preventDefault).toHaveBeenCalled();
    expect(onError).toHaveBeenCalledWith("Drag failed");
  });

  it("handleDragEnd clears error and calls endDrag", () => {
    const editor = makeMockEditor(true);
    const { result } = renderHook(() => useBlockDrag({ editor }));

    act(() => {
      result.current.handleDragEnd();
    });

    expect(result.current.dragError).toBeNull();
    expect(editor.BlockManager.endDrag).toHaveBeenCalledWith(true);
  });

  it("handleDragEnd is safe with null editor", () => {
    const { result } = renderHook(() => useBlockDrag({ editor: null }));

    expect(() => {
      act(() => {
        result.current.handleDragEnd();
      });
    }).not.toThrow();
  });

  it("setDragError can manually set an error", () => {
    const editor = makeMockEditor();
    const { result } = renderHook(() => useBlockDrag({ editor }));

    act(() => {
      result.current.setDragError("Custom error");
    });

    expect(result.current.dragError).toBe("Custom error");
  });
});
