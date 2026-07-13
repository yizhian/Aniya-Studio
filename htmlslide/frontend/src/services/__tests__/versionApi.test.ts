import { describe, it, expect, vi, beforeEach } from "vitest";
import { postSaveVersion, postRestoreVersion } from "../editor/versionApi";

describe("postSaveVersion", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("calls POST with correct body and returns response", async () => {
    const mockResponse = {
      project_id: "proj-1",
      version_id: "v2",
      version_tag: "v1.0.1",
      title: "My Version",
      html: "<div>test</div>",
      css: "",
      created_at: "2025-01-01T00:00:00Z",
    };
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    } as Response);

    const result = await postSaveVersion(
      "proj-1",
      "My Version",
      "<div>test</div>",
    );

    expect(result).toEqual(mockResponse);
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/projects/proj-1/versions",
      expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json" },
      }),
    );
    const body = JSON.parse(
      (fetch as any).mock.calls[0][1].body,
    );
    expect(body).toEqual({ title: "My Version", html: "<div>test</div>" });
  });

  it("throws with detail from error response", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ detail: "Invalid request" }),
    } as Response);

    await expect(
      postSaveVersion("proj-1", "Bad", "<div>"),
    ).rejects.toThrow("Invalid request");
  });

  it("throws with message when detail is absent", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ message: "Server error" }),
    } as Response);

    await expect(
      postSaveVersion("proj-1", "Title", "<div>"),
    ).rejects.toThrow("Server error");
  });

  it("throws with HTTP status when no body detail", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: false,
      status: 503,
      json: () => Promise.resolve({}),
    } as Response);

    await expect(
      postSaveVersion("proj-1", "Title", "<div>"),
    ).rejects.toThrow("HTTP 503");
  });

  it("url-encodes project ID with special characters", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          project_id: "proj/1",
          version_id: "v1",
          version_tag: "v1",
          title: "t",
          html: "",
          css: "",
          created_at: "2025-01-01T00:00:00Z",
        }),
    } as Response);

    await postSaveVersion("proj/1", "t", "");
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/projects/proj%2F1/versions",
      expect.anything(),
    );
  });
});

describe("postRestoreVersion", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("calls POST with correct URL and returns response", async () => {
    const mockResponse = {
      project_id: "proj-1",
      restored_to: "v1",
      new_version_id: "v3",
      new_version_tag: "v1.0.2",
      html: "<div>restored</div>",
      css: "",
    };
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    } as Response);

    const result = await postRestoreVersion("proj-1", "v1");

    expect(result).toEqual(mockResponse);
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/projects/proj-1/versions/v1/restore",
      { method: "POST" },
    );
  });

  it("throws on non-ok response", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ detail: "Version not found" }),
    } as Response);

    await expect(
      postRestoreVersion("proj-1", "v99"),
    ).rejects.toThrow("Version not found");
  });

  it("url-encodes both project ID and version ID", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: true,
      json: () =>
        Promise.resolve({
          project_id: "proj/1",
          restored_to: "v/hash",
          new_version_id: "v2",
          new_version_tag: "v2",
          html: "",
          css: "",
        }),
    } as Response);

    await postRestoreVersion("proj/1", "v/hash");
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/projects/proj%2F1/versions/v%2Fhash/restore",
      { method: "POST" },
    );
  });
});
