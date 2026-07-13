import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import React from "react";
import { useEditorViewModel } from "../useEditorViewModel";

// Mock react-router
const mockNavigate = vi.fn();
vi.mock("react-router", async () => {
  const actual = await vi.importActual("react-router");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({ state: null, pathname: "/editor/proj-test" }),
    useParams: () => ({ projectId: "proj-test" }),
  };
});

// Mock useChatStream
vi.mock("../../hooks/useChatStream", () => ({
  useChatStream: () => ({
    chatState: {
      streamStatus: "idle",
      timeline: [],
      error: null,
      doneMeta: null,
      skillRecommendations: [],
      lastPrompt: null,
    },
    send: vi.fn(),
    stopChat: vi.fn(),
    resetChat: vi.fn(),
    loadChatHistory: vi.fn(),
    loadProjectBrief: vi.fn(),
  }),
}));

// Mock useEditorUpload
vi.mock("../useEditorUpload", () => ({
  useEditorUpload: () => ({
    files: [],
    setFiles: vi.fn(),
    parsedFiles: [],
    setParsedFiles: vi.fn(),
    uploadProgress: null,
    setUploadProgress: vi.fn(),
    uploadStatus: "idle" as const,
    setUploadStatus: vi.fn(),
    parseElapsed: 0,
    fileChips: [],
    fileInputRef: { current: null },
    handleUpload: vi.fn(),
    documentViewerOpen: false,
    setDocumentViewerOpen: vi.fn(),
  }),
}));

// Mock deckAdapter
vi.mock("../../services/deckAdapter", () => ({
  readDeckState: vi.fn(() => ({
    pages: [{ id: "page-1", title: "Page 1" }],
    currentIndex: 0,
    totalSlides: 1,
  })),
  goToDeckSlide: vi.fn(),
  clearEditorSelection: vi.fn(),
}));

// Mock editorApi with postSaveVersion
vi.mock("../../services/editorApi", () => ({
  setHtmlAndCss: vi.fn(),
  serializePublishHtml: vi.fn(() => "<html><body>test</body></html>"),
  importHtmlDocument: vi.fn(),
  postSaveVersion: vi.fn(),
  resetCssBaseline: vi.fn(),
}));

// Create a minimal mock editor
function createMockEditor() {
  return {
    getModel: () => ({ get: () => "" }),
    getWrapper: () => ({ getEl: () => null }),
    getHtml: () => "<div>test</div>",
    getCss: () => "",
    setComponents: vi.fn(),
    getSelected: () => null,
    getSelectedAll: () => [],
    select: vi.fn(),
    on: vi.fn(),
    off: vi.fn(),
    Components: { getWrapper: () => null },
    Canvas: { getDocument: () => null, getFrameEl: () => null },
    BlockManager: { add: vi.fn(), get: vi.fn(() => []) },
    StyleManager: { addSector: vi.fn(), addProperty: vi.fn() },
    Panels: { addPanel: vi.fn() },
    destroy: vi.fn(),
    refresh: vi.fn(),
    getContainer: () => document.createElement("div"),
    Commands: { add: vi.fn(), run: vi.fn() },
    UndoManager: { add: vi.fn() },
  };
}

function createWrapper() {
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(
      MemoryRouter,
      { initialEntries: ["/editor/proj-test"] },
      children
    );
  };
}

const mockFetch = vi.fn();

describe("useEditorViewModel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockReset();
    vi.stubGlobal("fetch", mockFetch);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function setup(editor: ReturnType<typeof createMockEditor> | null = null) {
    const mockEditor = editor || createMockEditor();
    return renderHook(() => useEditorViewModel(mockEditor as any), {
      wrapper: createWrapper(),
    });
  }

  describe("initial state", () => {
    it("initializes with default panel states closed", () => {
      const { result } = setup();
      expect(result.current.isTerminalOpen).toBe(false);
      expect(result.current.isVersionOpen).toBe(false);
      expect(result.current.isAdvancedDesignOpen).toBe(false);
    });

    it("starts in Design mode", () => {
      const { result } = setup();
      expect(result.current.mode).toBe("design");
    });

    it("initializes with empty magic prompt", () => {
      const { result } = setup();
      expect(result.current.magicPrompt).toBe("");
    });

    it("returns the project ID from route params", () => {
      const { result } = setup();
      expect(result.current.routeProjectId).toBe("proj-test");
    });
  });

  describe("export functions", () => {
    it("handleExportPdf fetches the PDF export endpoint via GET", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        blob: () => Promise.resolve(new Blob(["fake-pdf"])),
      });

      const { result } = setup();

      await act(async () => {
        await result.current.handleExportPdf();
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/export/pdf")
      );
    });

    it("handleExportPptx fetches the PPTX export endpoint via GET", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        blob: () => Promise.resolve(new Blob(["fake-pptx"])),
      });

      const { result } = setup();

      await act(async () => {
        await result.current.handleExportPptx();
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/export/pptx")
      );
    });

    it("export PDF failure does NOT set magicError (silently swallowed - known issue)", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      const { result } = setup();

      await act(async () => {
        await result.current.handleExportPdf();
      });

      // NOTE: magicError is NOT set on export failure (error silently swallowed)
      // This confirms a known issue tracked in the improvement plan.
      expect(result.current.magicError).toBeNull();
    });
  });

  describe("save functionality", () => {
    it("handleSaveAndReturnOk returns false when not dirty", async () => {
      const { result } = setup();

      let ok = true;
      await act(async () => {
        ok = await result.current.handleSaveAndReturnOk();
      });

      // Not dirty, should return false without calling API
      expect(ok).toBe(false);
    });
  });
});
