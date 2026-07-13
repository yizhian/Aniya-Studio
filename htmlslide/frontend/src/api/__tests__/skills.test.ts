import { describe, it, expect, vi, beforeEach } from "vitest";
import { precipitateStream, precipitateConfirm, fetchSkillList, fetchSkillContent } from "../skills";

beforeEach(() => {
  vi.restoreAllMocks();
});

function mockFetch(ok = true, data?: unknown) {
  return vi.spyOn(globalThis, "fetch").mockResolvedValueOnce({
    ok,
    status: ok ? 200 : 500,
    json: vi.fn().mockResolvedValue(data ?? {}),
  } as any);
}

describe("precipitateStream", () => {
  it("POSTs to the stream endpoint with project_id and html_content", async () => {
    mockFetch();
    await precipitateStream("p1", "<html></html>");
    expect(fetch).toHaveBeenCalledWith("/api/v1/skills/precipitate/stream", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ project_id: "p1", html_content: "<html></html>" }),
    });
  });

  it("returns the raw Response", async () => {
    const mockResponse = { ok: true, status: 200, json: vi.fn() };
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(mockResponse as any);
    const res = await precipitateStream("p1", "<html></html>");
    expect(res).toBe(mockResponse);
  });
});

describe("precipitateConfirm", () => {
  const payload = {
    project_id: "p1",
    skill_name: "my-skill",
    scenario: "marketing",
    skill_md: "# Skill",
    example_html: "<div></div>",
  };

  it("POSTs to confirm with full payload", async () => {
    mockFetch();
    await precipitateConfirm(payload);
    expect(fetch).toHaveBeenCalledWith("/api/v1/skills/precipitate/confirm", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
  });
});

describe("fetchSkillList", () => {
  it("fetches skills with mode query param", async () => {
    mockFetch();
    await fetchSkillList("deck");
    expect(fetch).toHaveBeenCalledWith("/api/v1/skills?mode=deck");
  });

  it("URL-encodes the mode parameter", async () => {
    mockFetch();
    await fetchSkillList("slide deck");
    expect(fetch).toHaveBeenCalledWith("/api/v1/skills?mode=slide%20deck");
  });
});

describe("fetchSkillContent", () => {
  it("fetches skill content by name", async () => {
    mockFetch();
    await fetchSkillContent("my-skill");
    expect(fetch).toHaveBeenCalledWith("/api/v1/skills/my-skill/content");
  });

  it("URL-encodes the skill name", async () => {
    mockFetch();
    await fetchSkillContent("my skill/v2");
    expect(fetch).toHaveBeenCalledWith("/api/v1/skills/my%20skill%2Fv2/content");
  });
});
