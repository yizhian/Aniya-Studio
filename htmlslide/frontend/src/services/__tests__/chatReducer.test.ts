import { describe, it, expect } from "vitest";
import { chatReducer, initialChatState, makeTimelineEntry } from "../chatReducer";
import type { ChatAction } from "../chatReducer";
import type { ChatState, TimelineEntry } from "../../models/chat";

const brief = "Project summary";
const baseState: ChatState = {
  ...initialChatState,
  projectBrief: brief,
};

function dispatch(actions: ChatAction[], state: ChatState = baseState): ChatState {
  return actions.reduce((s, a) => chatReducer(s, a), state);
}

// ─── START ───

describe("START", () => {
  it("transitions to connecting and appends user_message", () => {
    const s = dispatch([{ type: "START", userMessage: "Hello" }]);
    expect(s.streamStatus).toBe("connecting");
    expect(s.lastPrompt).toBe("Hello");
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].event).toBe("user_message");
    expect(s.timeline[0].data.content).toContain("Hello");
  });

  it("prepends projectBrief to the first user message", () => {
    const s = dispatch([{ type: "START", userMessage: "Hello" }]);
    expect(s.timeline[0].data.content).toBe("Project summary\n\nHello");
  });

  it("does not duplicate brief if it matches the user message", () => {
    const s = dispatch([{ type: "START", userMessage: brief }]);
    expect(s.timeline[0].data.content).toBe(brief);
  });

  it("replaces a pending idle user_message instead of appending", () => {
    const s1 = dispatch([
      { type: "START", userMessage: "Q1" },
      { type: "ABORT" },
      { type: "START", userMessage: "Q2" },
    ]);
    expect(s1.timeline).toHaveLength(1);
    expect(s1.timeline[0].data.content).toBe("Project summary\n\nQ2");
  });

  it("does NOT replace when a response has started", () => {
    const s1 = dispatch([
      { type: "START", userMessage: "Q1" },
      { type: "TEXT", content: "Reply" },
      { type: "ABORT" },
      { type: "START", userMessage: "Q2" },
    ]);
    expect(s1.timeline).toHaveLength(3);
  });

  it("includes domContext and attachments", () => {
    const s = dispatch([
      {
        type: "START",
        userMessage: "Fix this",
        domContext: { css_path: ".box", tag: "div", text: "x", styles: {} },
        attachments: [{ original_name: "doc.pdf" }],
      },
    ]);
    expect(s.lastSelectedDom?.css_path).toBe(".box");
    expect(s.timeline[0].data.attachments).toEqual([{ original_name: "doc.pdf" }]);
  });

  it("clears error, doneMeta, and skillRecommendations", () => {
    const dirty: ChatState = {
      ...baseState,
      error: { code: "E", message: "x", recoverable: true },
      doneMeta: { projectId: "p", version: "v", totalRounds: 1 },
      skillRecommendations: [{ name: "s", description: "d", reason: "r" }],
    };
    const s = chatReducer(dirty, { type: "START", userMessage: "Hi" });
    expect(s.error).toBeNull();
    expect(s.doneMeta).toBeNull();
    expect(s.skillRecommendations).toBeNull();
  });

  it("merges attachments from the replaced entry", () => {
    const s1 = dispatch([
      { type: "START", userMessage: "Q1", attachments: [{ original_name: "a.pdf" }] },
      { type: "ABORT" },
    ]);
    const s2 = chatReducer(s1, { type: "START", userMessage: "Q2" });
    expect(s2.timeline).toHaveLength(1);
    expect(s2.timeline[0].data.attachments).toEqual([{ original_name: "a.pdf" }]);
  });
});

// ─── LOAD_HISTORY ───

describe("LOAD_HISTORY", () => {
  const entry: TimelineEntry = makeTimelineEntry("user_message", { content: "Old" });

  it("sets projectBrief", () => {
    const s = chatReducer(initialChatState, {
      type: "LOAD_HISTORY",
      timeline: [entry],
      projectBrief: "Brief from load",
    });
    expect(s.projectBrief).toBe("Brief from load");
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].event).toBe("user_message");
  });

  it("prepends brief to the first user_message", () => {
    const s = chatReducer(initialChatState, {
      type: "LOAD_HISTORY",
      timeline: [entry],
      projectBrief: "Brief",
    });
    expect(s.timeline[0].data.content).toBe("Brief\n\nOld");
  });

  it("does not double-prepend", () => {
    const e = makeTimelineEntry("user_message", { content: "Brief\n\nOld" });
    const s = chatReducer(initialChatState, {
      type: "LOAD_HISTORY",
      timeline: [e],
      projectBrief: "Brief",
    });
    expect(s.timeline[0].data.content).toBe("Brief\n\nOld");
  });

  it("falls back to existing projectBrief", () => {
    const s = chatReducer(baseState, {
      type: "LOAD_HISTORY",
      timeline: [entry],
    });
    expect(s.projectBrief).toBe(brief);
    expect(s.timeline[0].data.content).toBe("Project summary\n\nOld");
  });

  it("preserves lastPrompt and lastSelectedDom", () => {
    const s = chatReducer(initialChatState, {
      type: "LOAD_HISTORY",
      timeline: [entry],
      lastPrompt: "LP",
      lastSelectedDom: { css_path: ".x", tag: "p", text: "t", styles: {} },
    });
    expect(s.lastPrompt).toBe("LP");
    expect(s.lastSelectedDom?.css_path).toBe(".x");
  });
});

// ─── THINKING ───

describe("THINKING", () => {
  it("creates a thinking entry", () => {
    const s = dispatch([{ type: "THINKING", content: "Hmm" }]);
    expect(s.streamStatus).toBe("streaming");
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].event).toBe("thinking");
    expect(s.timeline[0].data.content).toBe("Hmm");
  });

  it("appends to the last thinking entry", () => {
    const s = dispatch([
      { type: "THINKING", content: "Hello " },
      { type: "THINKING", content: "World" },
    ]);
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].data.content).toBe("Hello World");
  });

  it("creates a new thinking entry when last event is not thinking", () => {
    const s = dispatch([
      { type: "THINKING", content: "T1" },
      { type: "TEXT", content: "Reply" },
      { type: "THINKING", content: "T2" },
    ]);
    expect(s.timeline).toHaveLength(3);
    expect(s.timeline[2].data.content).toBe("T2");
  });
});

// ─── TEXT ───

describe("TEXT", () => {
  it("creates a text entry", () => {
    const s = dispatch([{ type: "TEXT", content: "Hi" }]);
    expect(s.streamStatus).toBe("streaming");
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].event).toBe("text");
  });

  it("appends to the last text entry", () => {
    const s = dispatch([
      { type: "TEXT", content: "Hello " },
      { type: "TEXT", content: "World" },
    ]);
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].data.content).toBe("Hello World");
  });

  it("creates a new text entry when last event is not text", () => {
    const s = dispatch([
      { type: "TEXT", content: "T1" },
      { type: "THINKING", content: "..." },
      { type: "TEXT", content: "T2" },
    ]);
    expect(s.timeline).toHaveLength(3);
    expect(s.timeline[2].data.content).toBe("T2");
  });
});

// ─── TOOL_START / TOOL_RESULT / TODO_WRITE / HOOK_WARN ───

describe("TOOL events", () => {
  it("TOOL_START adds a tool start entry", () => {
    const s = dispatch([{ type: "TOOL_START", callId: "c1", name: "read" }]);
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].event).toBe("tool");
    expect(s.timeline[0].data).toEqual({ phase: "start", call_id: "c1", name: "read" });
  });

  it("TOOL_RESULT matches and updates the start entry", () => {
    const s = dispatch([
      { type: "TOOL_START", callId: "c1", name: "read" },
      { type: "TOOL_RESULT", callId: "c1", name: "read", success: true, summary: "OK", durationMs: 42 },
    ]);
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].data).toEqual({
      phase: "result", call_id: "c1", name: "read",
      success: true, summary: "OK", duration_ms: 42,
    });
  });

  it("TOOL_RESULT leaves start entry unchanged when callId does not match", () => {
    const s = dispatch([
      { type: "TOOL_START", callId: "c1", name: "read" },
      { type: "TOOL_RESULT", callId: "c2", name: "write", success: false, summary: "NOK", durationMs: 5 },
    ]);
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].data.phase).toBe("start");
    expect(s.timeline[0].data.call_id).toBe("c1");
  });

  it("TODO_WRITE adds a todo_write entry", () => {
    const todos = [{ content: "Fix bug", status: "pending", active_form: "Fixing bug" }];
    const s = dispatch([{ type: "TODO_WRITE", todos }]);
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].event).toBe("todo_write");
    expect(s.timeline[0].data.todos).toEqual(todos);
  });

  it("HOOK_WARN adds a hook:warn entry", () => {
    const s = dispatch([{ type: "HOOK_WARN", message: "Warning!" }]);
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].event).toBe("hook:warn");
    expect(s.timeline[0].data.message).toBe("Warning!");
  });
});

// ─── SKILL_RECOMMENDATIONS ───

describe("SKILL_RECOMMENDATIONS", () => {
  const recs = [{ name: "s1", description: "d", reason: "r" }];

  it("sets recommendations and adds user_message if none exists", () => {
    const s = chatReducer(initialChatState, {
      type: "SKILL_RECOMMENDATIONS",
      recommendations: recs,
      lastPrompt: "Prompt",
    });
    expect(s.skillRecommendations).toEqual(recs);
    expect(s.lastPrompt).toBe("Prompt");
    expect(s.timeline).toHaveLength(1);
    expect(s.timeline[0].event).toBe("user_message");
  });

  it("does not add a second user_message when one exists", () => {
    const s = dispatch([
      { type: "START", userMessage: "Hi" },
      { type: "SKILL_RECOMMENDATIONS", recommendations: recs, lastPrompt: "Hi" },
    ]);
    expect(s.timeline).toHaveLength(1);
  });

  it("sets projectBrief if not already set", () => {
    const s = chatReducer(initialChatState, {
      type: "SKILL_RECOMMENDATIONS",
      recommendations: recs,
      lastPrompt: "P",
    });
    expect(s.projectBrief).toBe("P");
  });

  it("preserves existing projectBrief", () => {
    const s = chatReducer(baseState, {
      type: "SKILL_RECOMMENDATIONS", recommendations: recs, lastPrompt: "New",
    });
    expect(s.projectBrief).toBe(brief);
  });
});

describe("CLEAR_SKILL_RECOMMENDATIONS", () => {
  it("clears skillRecommendations", () => {
    const withRecs: ChatState = {
      ...initialChatState,
      skillRecommendations: [{ name: "s", description: "d", reason: "r" }],
    };
    const s = chatReducer(withRecs, { type: "CLEAR_SKILL_RECOMMENDATIONS" });
    expect(s.skillRecommendations).toBeNull();
  });
});

// ─── ERROR / DONE / ABORT / RESET ───

describe("ERROR", () => {
  it("sets streamStatus to error and stores the error", () => {
    const s = chatReducer(initialChatState, {
      type: "ERROR",
      error: { code: "E1", message: "Oops", recoverable: false },
    });
    expect(s.streamStatus).toBe("error");
    expect(s.error).toEqual({ code: "E1", message: "Oops", recoverable: false });
  });
});

describe("DONE", () => {
  it("sets streamStatus to done and stores meta", () => {
    const s = chatReducer(initialChatState, {
      type: "DONE",
      meta: { projectId: "p1", version: "v2", totalRounds: 3 },
    });
    expect(s.streamStatus).toBe("done");
    expect(s.doneMeta).toEqual({ projectId: "p1", version: "v2", totalRounds: 3 });
  });
});

describe("ABORT", () => {
  it("resets streamStatus to idle", () => {
    const streaming: ChatState = { ...initialChatState, streamStatus: "streaming" };
    const s = chatReducer(streaming, { type: "ABORT" });
    expect(s.streamStatus).toBe("idle");
  });
});

describe("RESET", () => {
  it("resets to initial state with empty projectBrief", () => {
    const dirty: ChatState = {
      ...baseState,
      streamStatus: "done",
      timeline: [makeTimelineEntry("text", { content: "x" })],
      error: { code: "E", message: "x", recoverable: true },
      doneMeta: { projectId: "p", version: "v", totalRounds: 1 },
      lastPrompt: "Q",
    };
    const s = chatReducer(dirty, { type: "RESET" });
    expect(s).toEqual({ ...initialChatState, projectBrief: "" });
  });
});

// ─── makeTimelineEntry ───

describe("makeTimelineEntry", () => {
  it("returns a timeline entry with ISO timestamp", () => {
    const entry = makeTimelineEntry("text", { content: "Hi" });
    expect(entry.event).toBe("text");
    expect(entry.data).toEqual({ content: "Hi" });
    expect(() => new Date(entry.timestamp)).not.toThrow();
    expect(new Date(entry.timestamp).getTime()).toBeGreaterThan(0);
  });
});

// ─── unknown action ───

describe("unknown action", () => {
  it("returns state unchanged", () => {
    const s = chatReducer(initialChatState, { type: "UNKNOWN" } as any);
    expect(s).toBe(initialChatState);
  });
});
