import { describe, it, expect, vi, beforeEach } from "vitest";
import { createProject, updateProject, fetchProjectUploads } from "../projects";

beforeEach(() => {
  vi.restoreAllMocks();
});

function mockFetch(status: number, data: unknown) {
  return vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    json: vi.fn().mockResolvedValue(data),
  } as any);
}

describe("createProject", () => {
  it("returns the created project on success", async () => {
    mockFetch(201, { id: "proj-1" });
    const result = await createProject("My Project");
    expect(result).toEqual({ id: "proj-1" });
    expect(fetch).toHaveBeenCalledWith("/api/v1/projects", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "My Project" }),
    });
  });

  it("throws on server error with message from body", async () => {
    mockFetch(500, { message: "Database down" });
    await expect(createProject("X")).rejects.toThrow("Database down");
  });

  it("throws with HTTP status when body has no message", async () => {
    mockFetch(500, {});
    await expect(createProject("X")).rejects.toThrow("HTTP 500");
  });

  it("throws with HTTP status when body parse fails", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
      ok: false,
      status: 502,
      json: vi.fn().mockRejectedValue(new Error("parse error")),
    } as any);
    await expect(createProject("X")).rejects.toThrow("HTTP 502");
  });
});

describe("updateProject", () => {
  it("sends PATCH with payload and resolves on success", async () => {
    mockFetch(200, {});
    await expect(updateProject("p1", { name: "New" })).resolves.toBeUndefined();
    expect(fetch).toHaveBeenCalledWith("/api/v1/projects/p1", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "New" }),
    });
  });

  it("throws on error", async () => {
    mockFetch(400, { message: "Bad input" });
    await expect(updateProject("p1", {})).rejects.toThrow("Bad input");
  });

  it("URL-encodes the project ID", async () => {
    mockFetch(200, {});
    await updateProject("a/b", {});
    expect(fetch).toHaveBeenCalledWith("/api/v1/projects/a%2Fb", expect.any(Object));
  });
});

describe("fetchProjectUploads", () => {
  it("returns parsed files on success", async () => {
    mockFetch(200, { files: [{ original_name: "doc.pdf" }] });
    const result = await fetchProjectUploads("https://agent.example.com", "p1");
    expect(result).toEqual([{ original_name: "doc.pdf" }]);
    expect(fetch).toHaveBeenCalledWith("https://agent.example.com/projects/p1/uploads");
  });

  it("returns empty array on error status", async () => {
    mockFetch(500, {});
    const result = await fetchProjectUploads("https://agent.example.com", "p1");
    expect(result).toEqual([]);
  });

  it("returns empty array when files field is absent", async () => {
    mockFetch(200, {});
    const result = await fetchProjectUploads("https://agent.example.com", "p1");
    expect(result).toEqual([]);
  });
});
