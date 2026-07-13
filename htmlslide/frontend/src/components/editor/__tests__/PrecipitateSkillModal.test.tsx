import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, act } from "@testing-library/react";
import { PrecipitateSkillModal } from "../PrecipitateSkillModal";

// Mock the skills API module
vi.mock("../../../api/skills", () => ({
  precipitateStream: vi.fn(),
  precipitateConfirm: vi.fn(),
}));

import { precipitateStream, precipitateConfirm } from "../../../api/skills";

const mockPrecipitateStream = precipitateStream as ReturnType<typeof vi.fn>;
const mockPrecipitateConfirm = precipitateConfirm as ReturnType<typeof vi.fn>;

// Helper to build a mock SSE ReadableStream
function createSSEStream(chunks: Array<Record<string, unknown>>): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  const text = chunks
    .map((chunk) => `data: ${JSON.stringify(chunk)}\n`)
    .join("");
  return new ReadableStream({
    start(controller) {
      controller.enqueue(encoder.encode(text));
      controller.close();
    },
  });
}

describe("PrecipitateSkillModal", () => {
  const defaultProps = {
    projectId: "proj-1",
    htmlContent: "<div>test</div>",
    onClose: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // Default: successful precipitate stream with preview data
    const previewData = {
      suggested_name: "my-skill",
      description: "A test skill",
      scenario: "marketing",
      skill_md: "# SKILL\ncontent",
      example_html: "<html></html>",
    };
    const stream = createSSEStream([
      { type: "text", data: { text: "Analyzing design..." } },
      { type: "precipitate_result", data: previewData },
    ]);
    const mockResponse = {
      ok: true,
      status: 200,
      body: stream,
    };
    mockPrecipitateStream.mockResolvedValue(mockResponse);
    mockPrecipitateConfirm.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({ skill_name: "my-skill", dir_path: "/skills/my-skill" }),
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    render(<PrecipitateSkillModal {...defaultProps} />);
    // The modal should show the extracting text from locale mock
    expect(screen.getByText("precipitate.extracting")).toBeDefined();
  });

  it("transitions from loading to preview phase on successful stream", async () => {
    render(<PrecipitateSkillModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("precipitate.nameLabel")).toBeDefined();
    });

    // Should show the name input with suggested name
    const nameInput = screen.getByPlaceholderText("my-skill-name") as HTMLInputElement;
    expect(nameInput.value).toBe("my-skill");
  });

  it("shows error when precipitate stream fails", async () => {
    mockPrecipitateStream.mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ detail: "Server error" }),
    });

    render(<PrecipitateSkillModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("Server error")).toBeDefined();
    });
  });

  it("shows generic error when stream response has no body", async () => {
    mockPrecipitateStream.mockResolvedValue({
      ok: true,
      status: 200,
      body: null,
    });

    render(<PrecipitateSkillModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("No response body")).toBeDefined();
    });
  });

  it("confirms and transitions to done phase", async () => {
    render(<PrecipitateSkillModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("precipitate.nameLabel")).toBeDefined();
    });

    // Click confirm button
    const confirmButton = screen.getByText("precipitate.confirm");
    await act(async () => {
      fireEvent.click(confirmButton);
    });

    await waitFor(() => {
      expect(mockPrecipitateConfirm).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(screen.getByText("precipitate.saved")).toBeDefined();
    });
  });

  it("handles 409 conflict on confirm", async () => {
    mockPrecipitateConfirm.mockResolvedValue({
      ok: false,
      status: 409,
      json: () => Promise.resolve({ error: "Name already exists" }),
    });

    render(<PrecipitateSkillModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("precipitate.nameLabel")).toBeDefined();
    });

    const confirmButton = screen.getByText("precipitate.confirm");
    await act(async () => {
      fireEvent.click(confirmButton);
    });

    await waitFor(() => {
      expect(screen.getByText("Name already exists")).toBeDefined();
    });
  });

  it("calls onClose when Escape key is pressed", async () => {
    render(<PrecipitateSkillModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("precipitate.extracting")).toBeDefined();
    });

    fireEvent.keyDown(document, { key: "Escape" });
    expect(defaultProps.onClose).toHaveBeenCalledTimes(1);
  });

  it("calls regenerate when regenerate button is clicked", async () => {
    render(<PrecipitateSkillModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("precipitate.nameLabel")).toBeDefined();
    });

    const regenerateButton = screen.getByText("precipitate.regenerate");
    await act(async () => {
      fireEvent.click(regenerateButton);
    });

    // Should have called precipitateStream again (initial call + regenerate)
    await waitFor(() => {
      expect(mockPrecipitateStream).toHaveBeenCalledTimes(2);
    });
  });

  it("updates edited fields on user input", async () => {
    render(<PrecipitateSkillModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("precipitate.nameLabel")).toBeDefined();
    });

    const nameInput = screen.getByPlaceholderText("my-skill-name") as HTMLInputElement;
    await act(async () => {
      fireEvent.change(nameInput, { target: { value: "new-name" } });
    });
    expect(nameInput.value).toBe("new-name");
  });

  it("disables confirm button when name is empty", async () => {
    render(<PrecipitateSkillModal {...defaultProps} />);

    await waitFor(() => {
      expect(screen.getByText("precipitate.nameLabel")).toBeDefined();
    });

    const nameInput = screen.getByPlaceholderText("my-skill-name") as HTMLInputElement;
    await act(async () => {
      fireEvent.change(nameInput, { target: { value: "" } });
    });

    const confirmButton = screen.getByText("precipitate.confirm").closest("button")!;
    expect(confirmButton.disabled).toBe(true);
  });
});
