import { describe, it, expect, vi, beforeEach } from "vitest";
import { testConnection, syncToAgent, fetchModels } from "../settings";

beforeEach(() => {
  vi.restoreAllMocks();
});

function mockFetch(data: unknown, ok = true) {
  return vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
    ok,
    json: vi.fn().mockResolvedValue(data),
  } as any);
}

describe("testConnection", () => {
  it("returns connection result on success", async () => {
    mockFetch({ ok: true, message: "Connected", in_list: true, verified: true });
    const result = await testConnection({ key: "sk-xxx" });
    expect(result).toEqual({ ok: true, message: "Connected", inList: true, verified: true });
    expect(fetch).toHaveBeenCalledWith("/api/v1/settings/test-connection", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: "sk-xxx" }),
    });
  });

  it("falls back to defaults for missing fields", async () => {
    mockFetch({});
    const result = await testConnection({});
    expect(result).toEqual({ ok: false, message: "Unknown response", inList: undefined, verified: undefined });
  });
});

describe("syncToAgent", () => {
  it("returns sync result on success", async () => {
    mockFetch({ ok: true, message: "Synced" });
    const result = await syncToAgent({ url: "http://x" });
    expect(result).toEqual({ ok: true, message: "Synced" });
  });

  it("falls back to defaults for missing fields", async () => {
    mockFetch({});
    const result = await syncToAgent({});
    expect(result).toEqual({ ok: false, message: "Unknown response" });
  });
});

describe("fetchModels", () => {
  it("returns models list", async () => {
    mockFetch({ models: ["gpt-4", "claude-4"], source: "remote" });
    const result = await fetchModels({ provider: "openai" });
    expect(result).toEqual({ models: ["gpt-4", "claude-4"], source: "remote", error: undefined });
  });

  it("returns empty array and none source on missing fields", async () => {
    mockFetch({});
    const result = await fetchModels({});
    expect(result).toEqual({ models: [], source: "none", error: undefined });
  });

  it("passes through error field", async () => {
    mockFetch({ error: "Auth failed" });
    const result = await fetchModels({});
    expect(result.error).toBe("Auth failed");
  });
});
