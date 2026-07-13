import { describe, it, expect, vi, beforeEach } from "vitest";
import { getVersions, restoreVersion } from "../versions";

beforeEach(() => {
  vi.restoreAllMocks();
});

function mockFetch() {
  return vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
    ok: true,
    json: vi.fn().mockResolvedValue({}),
  } as any);
}

describe("getVersions", () => {
  it("fetches versions for a project", async () => {
    mockFetch();
    await getVersions("proj-1");
    expect(fetch).toHaveBeenCalledWith("/api/v1/projects/proj-1/versions");
  });

  it("URL-encodes the project ID", async () => {
    mockFetch();
    await getVersions("a/b");
    expect(fetch).toHaveBeenCalledWith("/api/v1/projects/a%2Fb/versions");
  });
});

describe("restoreVersion", () => {
  it("POSTs to restore a specific version", async () => {
    mockFetch();
    await restoreVersion("proj-1", "v2");
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/projects/proj-1/versions/v2/restore",
      { method: "POST" },
    );
  });

  it("URL-encodes both project ID and version ID", async () => {
    mockFetch();
    await restoreVersion("a/b", "v 1");
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/projects/a%2Fb/versions/v%201/restore",
      { method: "POST" },
    );
  });
});
