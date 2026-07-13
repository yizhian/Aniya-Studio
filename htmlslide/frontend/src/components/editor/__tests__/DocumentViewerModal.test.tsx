import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, within, waitFor, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DocumentViewerModal } from "../../../components/editor/DocumentViewerModal";
import type { ParsedFile } from "../../../services/upload";

// ── helpers ──────────────────────────────────────────────────────────

function makeDoc(overrides: Partial<ParsedFile> = {}): ParsedFile {
  return {
    original_name: "test.pdf",
    type: "pdf",
    saved_path_rel: "uploads/docs/test.pdf",
    original_path_rel: "uploads/originals/test.pdf",
    char_count: 100,
    ...overrides,
  };
}

function baseProps(overrides: Record<string, unknown> = {}) {
  return {
    isOpen: true,
    onClose: vi.fn(),
    documents: [] as ParsedFile[],
    projectId: "proj-test",
    agentgoBaseUrl: "http://localhost:8080",
    ...overrides,
  };
}

// ── file type detection ──────────────────────────────────────────────

describe("DocumentViewerModal — file type detection", () => {
  it("renders pdf type with text-red-400 icon", () => {
    const doc = makeDoc({ original_name: "report.pdf", type: "pdf" });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);
    // Filename appears in both header and sidebar — at least 1 match.
    expect(screen.getAllByText("report.pdf").length).toBeGreaterThanOrEqual(1);
    // Sidebar icon should have the red PDF class.
    const sidebar = document.querySelector(".w-56") as HTMLElement;
    expect(sidebar.querySelector(".text-red-400")).not.toBeNull();
  });

  it("renders image type with text-green-400 icon", () => {
    const doc = makeDoc({ original_name: "photo.png", type: "image", original_path_rel: "uploads/originals/photo.png" });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);
    expect(screen.getAllByText("photo.png").length).toBeGreaterThanOrEqual(1);
    const sidebar = document.querySelector(".w-56") as HTMLElement;
    expect(sidebar.querySelector(".text-green-400")).not.toBeNull();
  });

  it("renders markdown type with text-purple-400 icon", () => {
    const doc = makeDoc({ original_name: "notes.md", type: "markdown", original_path_rel: "uploads/originals/notes.md" });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);
    expect(screen.getAllByText("notes.md").length).toBeGreaterThanOrEqual(1);
    const sidebar = document.querySelector(".w-56") as HTMLElement;
    expect(sidebar.querySelector(".text-purple-400")).not.toBeNull();
  });

  it("renders docx type with text-blue-400 icon", () => {
    const doc = makeDoc({ original_name: "spec.docx", type: "docx", original_path_rel: "uploads/originals/spec.docx" });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);
    expect(screen.getAllByText("spec.docx").length).toBeGreaterThanOrEqual(1);
    const sidebar = document.querySelector(".w-56") as HTMLElement;
    expect(sidebar.querySelector(".text-blue-400")).not.toBeNull();
  });

  it("renders text type with default icon", () => {
    const doc = makeDoc({ original_name: "readme.txt", type: "text", original_path_rel: "uploads/originals/readme.txt" });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);
    expect(screen.getAllByText("readme.txt").length).toBeGreaterThanOrEqual(1);
  });
});

// ── modal open/close ────────────────────────────────────────────────

describe("DocumentViewerModal — open/close behavior", () => {
  it("renders nothing when isOpen is false", () => {
    render(<DocumentViewerModal {...baseProps({ isOpen: false })} />);
    expect(screen.queryByText("Documents")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true", () => {
    const doc = makeDoc({ original_name: "open-doc.pdf" });
    render(<DocumentViewerModal {...baseProps({ isOpen: true, documents: [doc] })} />);
    expect(screen.getAllByText("open-doc.pdf").length).toBeGreaterThanOrEqual(1);
  });

  it("closes on Escape key", () => {
    const onClose = vi.fn();
    render(<DocumentViewerModal {...baseProps({ documents: [makeDoc()], onClose })} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("closes on backdrop click", () => {
    const onClose = vi.fn();
    render(<DocumentViewerModal {...baseProps({ documents: [makeDoc()], onClose })} />);
    const backdrop = document.querySelector(".bg-black\\/50") as HTMLElement;
    expect(backdrop).not.toBeNull();
    fireEvent.click(backdrop);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("closes on X button click", () => {
    const onClose = vi.fn();
    render(<DocumentViewerModal {...baseProps({ documents: [makeDoc()], onClose })} />);
    // The X button in the header
    const header = document.querySelector(".flex.items-center.justify-between") as HTMLElement;
    const buttons = header?.querySelectorAll("button");
    const closeBtn = buttons?.[buttons.length - 1]; // last button = close (X)
    if (closeBtn) fireEvent.click(closeBtn);
    expect(onClose).toHaveBeenCalledOnce();
  });
});

// ── sidebar document list ───────────────────────────────────────────

describe("DocumentViewerModal — sidebar", () => {
  it("shows all documents in sidebar", () => {
    const docs = [
      makeDoc({ original_name: "doc1.pdf" }),
      makeDoc({ original_name: "doc2.pdf" }),
      makeDoc({ original_name: "doc3.pdf" }),
    ];
    render(<DocumentViewerModal {...baseProps({ documents: docs })} />);
    // Each doc name appears in sidebar span.
    const sidebar = document.querySelector(".w-56") as HTMLElement;
    expect(within(sidebar).getByText("doc1.pdf")).toBeInTheDocument();
    expect(within(sidebar).getByText("doc2.pdf")).toBeInTheDocument();
    expect(within(sidebar).getByText("doc3.pdf")).toBeInTheDocument();
  });

  it("shows empty state when no documents", () => {
    render(<DocumentViewerModal {...baseProps({ documents: [] })} />);
    expect(screen.getByText("No documents uploaded yet.")).toBeInTheDocument();
  });

  it("highlights selected document", () => {
    const docs = [
      makeDoc({ original_name: "first.pdf" }),
      makeDoc({ original_name: "second.pdf" }),
    ];
    render(<DocumentViewerModal {...baseProps({ documents: docs })} />);

    const sidebar = document.querySelector(".w-56") as HTMLElement;
    const firstBtn = within(sidebar).getByText("first.pdf").closest("button");
    const secondBtn = within(sidebar).getByText("second.pdf").closest("button");
    // Selected item has the solid background (not just hover).
    expect(firstBtn?.className).toContain("bg-[var(--editor-control-hover)]");
    // Unselected item has muted text color.
    expect(secondBtn?.className).toContain("text-[var(--editor-text-muted)]");
  });

  it("switches selection on document click", async () => {
    const docs = [
      makeDoc({ original_name: "first.pdf" }),
      makeDoc({ original_name: "second.pdf" }),
    ];
    render(<DocumentViewerModal {...baseProps({ documents: docs })} />);

    const sidebar = document.querySelector(".w-56") as HTMLElement;
    await userEvent.click(within(sidebar).getByText("second.pdf"));

    const firstBtn = within(sidebar).getByText("first.pdf").closest("button");
    const secondBtn = within(sidebar).getByText("second.pdf").closest("button");
    expect(secondBtn?.className).toContain("bg-[var(--editor-control-hover)]");
    expect(firstBtn?.className).toContain("text-[var(--editor-text-muted)]");
  });
});

// ── content area ────────────────────────────────────────────────────

describe("DocumentViewerModal — content area", () => {
  it("shows 'No documents' when selected is undefined", () => {
    render(<DocumentViewerModal {...baseProps({ documents: [] })} />);
    expect(screen.getByText("No documents uploaded yet.")).toBeInTheDocument();
  });

  it("shows 'Preview not available' for unknown file types", () => {
    const doc = {
      ...makeDoc(),
      original_name: "data.bin",
      type: "binary",
      original_path_rel: undefined,
    } as unknown as ParsedFile;
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);
    expect(screen.getByText("Preview not available for this file type.")).toBeInTheDocument();
  });

  it("shows loading state for text files", () => {
    const doc = makeDoc({ original_name: "readme.txt", type: "text", original_path_rel: "uploads/originals/readme.txt" });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("shows loading state for markdown files", () => {
    const doc = makeDoc({ original_name: "notes.md", type: "markdown", original_path_rel: "uploads/originals/notes.md" });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);
    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("shows header with selected document name", () => {
    const doc = makeDoc({ original_name: "header-doc.pdf" });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);
    // The header <h2> has the doc name
    const h2 = document.querySelector("h2");
    expect(h2?.textContent).toBe("header-doc.pdf");
  });

  it("shows 'Documents' header when no document selected", () => {
    render(<DocumentViewerModal {...baseProps({ documents: [] })} />);
    const h2 = document.querySelector("h2");
    expect(h2?.textContent).toBe("Documents");
  });
});

// ── selection edge cases ────────────────────────────────────────────

describe("DocumentViewerModal — selection edge cases", () => {
  it("resets selectedIdx when modal opens with new documents", async () => {
    const docs = [
      makeDoc({ original_name: "alpha.pdf" }),
      makeDoc({ original_name: "beta.pdf" }),
    ];
    const { rerender } = render(<DocumentViewerModal {...baseProps({ documents: docs })} />);

    const sidebar = document.querySelector(".w-56") as HTMLElement;
    await fireEvent.click(within(sidebar).getByText("beta.pdf"));

    const newDocs = [
      makeDoc({ original_name: "gamma.pdf" }),
      makeDoc({ original_name: "delta.pdf" }),
    ];
    rerender(<DocumentViewerModal {...baseProps({ documents: newDocs, isOpen: true })} />);

    // Header should show first new doc
    const h2 = document.querySelector("h2");
    expect(h2?.textContent).toBe("gamma.pdf");
  });
});

// ── cleanup ─────────────────────────────────────────────────────────

describe("DocumentViewerModal — cleanup", () => {
  it("does not fire onClose when escape pressed while closed", () => {
    const onClose = vi.fn();
    render(<DocumentViewerModal {...baseProps({ isOpen: false, onClose })} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("removes escape listener on unmount", () => {
    const onClose = vi.fn();
    const { unmount } = render(
      <DocumentViewerModal {...baseProps({ documents: [makeDoc()], onClose })} />,
    );
    unmount();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });
});

// ── integration: viewer content rendering with mocked fetch ─────────

describe("DocumentViewerModal — viewer integration", () => {
  const originalFetch = globalThis.fetch;

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("TextFileViewer renders fetched text content", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve("Hello world text content"),
    });

    const doc = makeDoc({
      original_name: "readme.txt",
      type: "text",
      original_path_rel: "uploads/originals/readme.txt",
    });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);

    await waitFor(() => {
      expect(screen.getByText("Hello world text content")).toBeInTheDocument();
    });
  });

  it("TextFileViewer shows error on fetch failure", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
    });

    const doc = makeDoc({
      original_name: "broken.txt",
      type: "text",
      original_path_rel: "uploads/originals/broken.txt",
    });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);

    await waitFor(() => {
      expect(screen.getByText(/Failed to load/)).toBeInTheDocument();
    });
  });

  it("MarkdownViewer renders parsed markdown in iframe", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve("# Hello Markdown\n\nSome **bold** text."),
    });

    const doc = makeDoc({
      original_name: "notes.md",
      type: "markdown",
      original_path_rel: "uploads/originals/notes.md",
    });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);

    // MarkdownViewer renders an iframe after marked.parse completes.
    await waitFor(() => {
      const iframe = document.querySelector("iframe[title='Markdown Viewer']");
      expect(iframe).not.toBeNull();
    });
  });

  it("MarkdownViewer shows error on fetch failure", async () => {
    globalThis.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    const doc = makeDoc({
      original_name: "broken.md",
      type: "markdown",
      original_path_rel: "uploads/originals/broken.md",
    });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);

    await waitFor(() => {
      expect(screen.getByText(/Failed to load/)).toBeInTheDocument();
    });
  });

  it("PDF viewer renders iframe with file URL", () => {
    const doc = makeDoc({
      original_name: "slides.pdf",
      type: "pdf",
      original_path_rel: "uploads/originals/slides.pdf",
    });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);

    // PDF viewer uses a <iframe> to the file URL.
    const iframe = document.querySelector("iframe[title='PDF Viewer']");
    expect(iframe).not.toBeNull();
    expect((iframe as HTMLIFrameElement).src).toContain("originals/slides.pdf");
  });

  it("Image viewer renders <img> with file URL", () => {
    const doc = makeDoc({
      original_name: "photo.png",
      type: "image",
      original_path_rel: "uploads/originals/photo.png",
    });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);

    const img = document.querySelector("img");
    expect(img).not.toBeNull();
    expect((img as HTMLImageElement).src).toContain("originals/photo.png");
  });

  it("falls back to text viewer for old pdf/docx without original", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve("parsed content from old upload"),
    });

    const doc = makeDoc({
      original_name: "old-report.pdf",
      type: "pdf",
      saved_path_rel: "uploads/docs/old-report.pdf",
      original_path_rel: undefined, // no original — falls back to text viewer
    });
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);

    // Should render as text, not PDF iframe.
    await waitFor(() => {
      expect(screen.getByText("parsed content from old upload")).toBeInTheDocument();
    });
    expect(document.querySelector("iframe[title='PDF Viewer']")).toBeNull();
  });

  it("shows 'Preview not available' for other type with no original_path_rel", () => {
    const doc = {
      ...makeDoc(),
      original_name: "data.csv",
      type: "other",
      original_path_rel: undefined,
    } as unknown as ParsedFile;
    render(<DocumentViewerModal {...baseProps({ documents: [doc] })} />);

    expect(
      screen.getByText("Preview not available for this file type."),
    ).toBeInTheDocument();
  });
});

// ── smoke: full viewer lifecycle ────────────────────────────────────

describe("DocumentViewerModal — smoke", () => {
  const originalFetch = globalThis.fetch;

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it("opens, shows document, selects another, closes", async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve("content"),
    });

    const docs = [
      makeDoc({ original_name: "a.txt", type: "text", original_path_rel: "uploads/originals/a.txt" }),
      makeDoc({ original_name: "b.txt", type: "text", original_path_rel: "uploads/originals/b.txt" }),
    ];
    const onClose = vi.fn();
    const { rerender } = render(
      <DocumentViewerModal {...baseProps({ isOpen: false, documents: docs, onClose })} />,
    );

    // Closed — nothing rendered.
    expect(screen.queryByText("a.txt")).not.toBeInTheDocument();

    // Open.
    rerender(<DocumentViewerModal {...baseProps({ isOpen: true, documents: docs, onClose })} />);
    expect(screen.getAllByText("a.txt").length).toBeGreaterThanOrEqual(1);

    // Switch to second document.
    const sidebar = document.querySelector(".w-56") as HTMLElement;
    fireEvent.click(within(sidebar).getByText("b.txt"));

    // Text content should render.
    await waitFor(() => {
      expect(screen.getByText("content")).toBeInTheDocument();
    });

    // Close via Escape.
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });
});
