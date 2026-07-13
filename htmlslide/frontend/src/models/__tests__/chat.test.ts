import { describe, it, expect } from "vitest";

describe("ChatRequestPayload", () => {
  it("skill field is optional — can be omitted", () => {
    const payload = {
      project_id: "proj-1",
      prompt: "hello",
    };
    expect(payload.project_id).toBe("proj-1");
    expect(payload.prompt).toBe("hello");
    expect("skill" in payload).toBe(false);
  });

  it("minimal payload serializes without skill field", () => {
    const payload = {
      project_id: "proj-1",
      prompt: "make a slide",
    };
    const json = JSON.stringify(payload);
    const parsed = JSON.parse(json);
    expect(parsed).toEqual({
      project_id: "proj-1",
      prompt: "make a slide",
    });
  });

  it("full payload with skill serializes correctly", () => {
    const payload = {
      project_id: "proj-1",
      prompt: "make a slide",
      selected_dom: {
        css_path: "body > div",
        tag: "div",
        text: "hello",
        styles: { color: "red" },
      },
      skill: "warm-style",
    };
    const json = JSON.stringify(payload);
    const parsed = JSON.parse(json);
    expect(parsed.project_id).toBe("proj-1");
    expect(parsed.prompt).toBe("make a slide");
    expect(parsed.selected_dom).toEqual({
      css_path: "body > div",
      tag: "div",
      text: "hello",
      styles: { color: "red" },
    });
    expect(parsed.skill).toBe("warm-style");
  });

  it("skill field with empty string is included in JSON", () => {
    const payload = {
      project_id: "proj-1",
      prompt: "test",
      skill: "",
    };
    const json = JSON.stringify(payload);
    const parsed = JSON.parse(json);
    expect(parsed.skill).toBe("");
  });

  it("skill field with special characters serializes properly", () => {
    const payload = {
      project_id: "proj-1",
      prompt: "test",
      skill: 'html-ppt-zhangzara-coral-v2.1',
    };
    const json = JSON.stringify(payload);
    const parsed = JSON.parse(json);
    expect(parsed.skill).toBe('html-ppt-zhangzara-coral-v2.1');
  });

  it("skill field with unicode serializes properly", () => {
    const payload = {
      project_id: "proj-1",
      prompt: "test",
      skill: "暖色系-café",
    };
    const json = JSON.stringify(payload);
    const parsed = JSON.parse(json);
    expect(parsed.skill).toBe("暖色系-café");
  });

  it("handles null skill by omitting it", () => {
    // When skill is undefined, JSON.stringify omits it.
    const payload: Record<string, unknown> = {
      project_id: "proj-1",
      prompt: "test",
      skill: undefined,
    };
    const json = JSON.stringify(payload);
    const parsed = JSON.parse(json);
    expect("skill" in parsed).toBe(false);
  });
});

describe("SkillRecommendation", () => {
  it("has all fields including has_preview", () => {
    const rec = {
      name: "coral-deck",
      description: "Warm coral theme",
      reason: "Matches user request for warm colors",
      scenario: "marketing",
      has_assets: true,
      has_preview: true,
    };
    expect(rec.name).toBe("coral-deck");
    expect(rec.description).toBe("Warm coral theme");
    expect(rec.reason).toBe("Matches user request for warm colors");
    expect(rec.scenario).toBe("marketing");
    expect(rec.has_assets).toBe(true);
    expect(rec.has_preview).toBe(true);
  });

  it("has_preview can be omitted (undefined)", () => {
    const rec = {
      name: "simple-deck",
      description: "Simple deck",
      reason: "Simple match",
    };
    expect(rec.name).toBe("simple-deck");
    expect("has_preview" in rec).toBe(false);
  });

  it("serializes to JSON including has_preview", () => {
    const rec = {
      name: "coral-deck",
      description: "Warm coral theme",
      reason: "Matches",
      has_preview: false,
    };
    const json = JSON.stringify(rec);
    const parsed = JSON.parse(json);
    expect(parsed.has_preview).toBe(false);
    expect(parsed.has_assets).toBeUndefined();
  });

  it("serializes with has_assets and has_preview both present", () => {
    const rec = {
      name: "full-deck",
      description: "Full featured deck",
      reason: "Best match",
      scenario: "education",
      has_assets: true,
      has_preview: true,
    };
    const json = JSON.stringify(rec);
    const parsed = JSON.parse(json);
    expect(parsed.name).toBe("full-deck");
    expect(parsed.has_assets).toBe(true);
    expect(parsed.has_preview).toBe(true);
    expect(parsed.scenario).toBe("education");
  });
});
