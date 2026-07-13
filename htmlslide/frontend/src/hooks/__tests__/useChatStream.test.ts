import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { useChatStream } from "../useChatStream";
import type { ChatState } from "../../models/chat";

// Helpers to build SSE byte streams
function sseEvent(event: string, data: object): string {
  return `event: ${event}\ndata: ${JSON.stringify(data)}\n\n`;
}

function sseBytes(...chunks: string[]): ReadableStream<Uint8Array> {
  const encoder = new TextEncoder();
  let index = 0;
  return new ReadableStream({
    pull(controller) {
      if (index < chunks.length) {
        controller.enqueue(encoder.encode(chunks[index]));
        index++;
      } else {
        controller.close();
      }
    },
  });
}

describe("useChatStream", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: true, body: null }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function currentState(result: { current: { chatState: ChatState } }) {
    return result.current.chatState;
  }

  it("transitions through statuses on a normal SSE stream", async () => {
    const encoder = new TextEncoder();
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(
          encoder.encode(
            sseEvent("thinking", { content: "Let me plan..." }) +
              sseEvent("text", { content: "Hello" }),
          ),
        );
        controller.enqueue(
          encoder.encode(
            sseEvent("tool", {
              phase: "start",
              call_id: "c1",
              name: "write_file",
            }) +
              sseEvent("tool", {
                phase: "result",
                call_id: "c1",
                name: "write_file",
                success: true,
                summary: "wrote index.html",
                duration_ms: 120,
              }),
          ),
        );
        controller.enqueue(
          encoder.encode(
            sseEvent("done", {
              project_id: "proj-test",
              version: "V1",
              total_rounds: 1,
            }),
          ),
        );
        controller.close();
      },
    });

    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        body: stream,
      }),
    );

    const { result } = renderHook(() => useChatStream());

    await act(async () => {
      result.current.startChat("proj-test", "make a slide");
    });

    await waitFor(() => {
      expect(currentState(result).streamStatus).toBe("done");
    });

    const state = currentState(result);
    expect(state.timeline.length).toBeGreaterThan(0);
    expect(state.timeline[0].event).toBe("user_message");
    const toolResults = state.timeline.filter(
      (e) => e.event === "tool" && e.data.phase === "result",
    );
    expect(toolResults.length).toBe(1);
    expect(toolResults[0].data.name).toBe("write_file");
    expect(state.doneMeta).toEqual({
      projectId: "proj-test",
      version: "V1",
      totalRounds: 1,
    });
  });

  it("handles HTTP 4xx as error with recoverable:true", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 422,
        text: () =>
          Promise.resolve(
            JSON.stringify({
              detail: { code: "VALIDATION_ERROR", message: "Bad input" },
            }),
          ),
      }),
    );

    const { result } = renderHook(() => useChatStream());

    await act(async () => {
      result.current.startChat("proj-test", "bad prompt");
    });

    await waitFor(() => {
      expect(currentState(result).streamStatus).toBe("error");
    });

    expect(currentState(result).error).toMatchObject({
      recoverable: true,
    });
  });

  it("handles network interruption as recoverable error", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new TypeError("Failed to fetch")),
    );

    const { result } = renderHook(() => useChatStream());

    await act(async () => {
      result.current.startChat("proj-test", "test");
    });

    await waitFor(() => {
      expect(currentState(result).streamStatus).toBe("error");
    });

    expect(currentState(result).error).toMatchObject({
      code: "stream_error",
      recoverable: true,
    });
  });

  it("returns to idle after abort", async () => {
    // Use a stream that never closes to simulate an in-flight request
    const stream = new ReadableStream({
      start() {
        // never push data — hangs until abort
      },
    });

    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        body: stream,
      }),
    );

    const onError = vi.fn();
    const { result } = renderHook(() => useChatStream({ onError }));

    await act(async () => {
      result.current.startChat("proj-test", "test");
    });

    // Should be connecting/streaming
    expect(["connecting", "streaming"]).toContain(
      currentState(result).streamStatus,
    );

    // Abort
    await act(async () => {
      result.current.stopChat();
    });

    expect(currentState(result).streamStatus).toBe("idle");
    // onError should NOT be called for user-initiated abort
    expect(onError).not.toHaveBeenCalled();
  });

  it("calls onDone callback when stream completes", async () => {
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(
          new TextEncoder().encode(
            sseEvent("done", {
              project_id: "proj-x",
              version: "V2",
              total_rounds: 2,
            }),
          ),
        );
        controller.close();
      },
    });

    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        body: stream,
      }),
    );

    const onDone = vi.fn();
    const { result } = renderHook(() => useChatStream({ onDone }));

    await act(async () => {
      result.current.startChat("proj-x", "hi");
    });

    await waitFor(() => {
      expect(onDone).toHaveBeenCalled();
    });

    expect(onDone).toHaveBeenCalledWith({
      projectId: "proj-x",
      version: "V2",
      totalRounds: 2,
    });
  });

  it("dispatches error for non-recoverable errors from AgentGo", async () => {
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(
          new TextEncoder().encode(
            sseEvent("error", {
              code: "agent_unavailable",
              message: "AgentGo is down",
              recoverable: false,
            }),
          ),
        );
        controller.close();
      },
    });

    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        body: stream,
      }),
    );

    const onError = vi.fn();
    const { result } = renderHook(() => useChatStream({ onError }));

    await act(async () => {
      result.current.startChat("proj-test", "test");
    });

    await waitFor(() => {
      expect(currentState(result).streamStatus).toBe("error");
    });

    expect(currentState(result).error).toMatchObject({
      code: "agent_unavailable",
      recoverable: false,
    });
    expect(onError).toHaveBeenCalled();
  });

  it("does not include skill field in POST body (removed from ChatRequestPayload)", async () => {
    const fetchSpy = vi.fn().mockResolvedValue({
      ok: true,
      body: new ReadableStream({
        start(controller) {
          controller.enqueue(
            new TextEncoder().encode(
              sseEvent("done", {
                project_id: "proj-test",
                version: "V1",
                total_rounds: 1,
              }),
            ),
          );
          controller.close();
        },
      }),
    });
    vi.stubGlobal("fetch", fetchSpy);

    const { result } = renderHook(() => useChatStream());

    await act(async () => {
      result.current.startChat("proj-test", "make a slide");
    });

    await waitFor(() => {
      expect(currentState(result).streamStatus).toBe("done");
    });

    const body = JSON.parse(fetchSpy.mock.calls[0][1].body);
    expect(body.skill).toBeUndefined();
  });

  it("dispatches SKILL_RECOMMENDATIONS via recommendStyles REST call", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          recommendations: [
            { name: "coral-deck", description: "Warm coral theme", reason: "Matches warm request", scenario: "presentation", has_assets: false },
            { name: "blue-deck", description: "Cool blue theme", reason: "Good alternative", scenario: "corporate", has_assets: false },
          ],
        }),
      }),
    );

    const { result } = renderHook(() => useChatStream());

    await act(async () => {
      await result.current.recommendStyles("proj-test", "warm presentation");
    });

    await waitFor(() => {
      expect(currentState(result).skillRecommendations).not.toBeNull();
    });

    expect(currentState(result).skillRecommendations).toEqual([
      { name: "coral-deck", description: "Warm coral theme", reason: "Matches warm request", scenario: "presentation", has_assets: false },
      { name: "blue-deck", description: "Cool blue theme", reason: "Good alternative", scenario: "corporate", has_assets: false },
    ]);
    expect(currentState(result).lastPrompt).toBe("warm presentation");
  });

  it("clears skill recommendations on CLEAR_SKILL_RECOMMENDATIONS", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          recommendations: [{ name: "coral-deck", description: "Warm coral theme", reason: "Matches", scenario: "presentation", has_assets: false }],
        }),
      }),
    );

    const { result } = renderHook(() => useChatStream());

    await act(async () => {
      await result.current.recommendStyles("proj-test", "warm");
    });

    await waitFor(() => {
      expect(currentState(result).skillRecommendations).not.toBeNull();
    });

    // Clear them
    await act(async () => {
      result.current.clearSkillRecommendations();
    });

    expect(currentState(result).skillRecommendations).toBeNull();
  });

  it("START action resets skillRecommendations and sets lastPrompt/lastSelectedDom", async () => {
    // First, get skill recommendations via REST
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          recommendations: [{ name: "coral-deck", description: "Warm theme", reason: "Matches", scenario: "presentation", has_assets: false }],
        }),
      }),
    );

    const { result } = renderHook(() => useChatStream());

    await act(async () => {
      await result.current.recommendStyles("proj-test", "first");
    });

    await waitFor(() => {
      expect(currentState(result).skillRecommendations).not.toBeNull();
    });

    // Second START should clear recommendations
    const stream2 = new ReadableStream({
      start(controller) {
        controller.enqueue(
          new TextEncoder().encode(
            sseEvent("done", { project_id: "proj-test", version: "V2", total_rounds: 1 }),
          ),
        );
        controller.close();
      },
    });

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, body: stream2 }));

    await act(async () => {
      result.current.startChat("proj-test", "second", { css_path: "div.other", tag: "section", text: "other", styles: {} });
    });

    await waitFor(() => {
      expect(currentState(result).streamStatus).toBe("done");
    });

    expect(currentState(result).skillRecommendations).toBeNull();
    expect(currentState(result).lastPrompt).toBe("second");
    expect(currentState(result).lastSelectedDom).toEqual({ css_path: "div.other", tag: "section", text: "other", styles: {} });
  });

  it("merges brief into display content when first START differs from stored lastPrompt", async () => {
    // Phase 1: recommendStyles → sets lastPrompt via SKILL_RECOMMENDATIONS
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          recommendations: [{ name: "coral-deck", description: "Warm theme", reason: "Matches", scenario: "presentation", has_assets: false }],
        }),
      }),
    );

    const { result } = renderHook(() => useChatStream());

    await act(async () => {
      await result.current.recommendStyles("proj-test", "make a Bauhaus presentation");
    });

    await waitFor(() => {
      expect(currentState(result).lastPrompt).toBe("make a Bauhaus presentation");
    });

    // Phase 2: Flow B — user types a different message directly
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(
          new TextEncoder().encode(
            sseEvent("done", { project_id: "proj-test", version: "V1", total_rounds: 1 }),
          ),
        );
        controller.close();
      },
    });

    const fetchSpy = vi.fn().mockResolvedValue({ ok: true, body: stream });
    vi.stubGlobal("fetch", fetchSpy);

    await act(async () => {
      result.current.startChat("proj-test", "Minimalist black white");
    });

    await waitFor(() => {
      expect(currentState(result).streamStatus).toBe("done");
    });

    const userMessages = currentState(result).timeline.filter(e => e.event === "user_message");
    expect(userMessages.length).toBe(1);
    expect(userMessages[0].data.content).toBe("make a Bauhaus presentation\n\nMinimalist black white");

    // POST body should still be the refine, not the merged display content
    const body = JSON.parse(fetchSpy.mock.calls[0][1].body);
    expect(body.prompt).toBe("Minimalist black white");
  });

  it("does not merge when brief and refine are the same (Flow A guard)", async () => {
    // Phase 1: recommendStyles → sets lastPrompt
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          recommendations: [{ name: "coral-deck", description: "Warm theme", reason: "Matches", scenario: "presentation", has_assets: false }],
        }),
      }),
    );

    const { result } = renderHook(() => useChatStream());

    await act(async () => {
      await result.current.recommendStyles("proj-test", "corporate deck");
    });

    await waitFor(() => {
      expect(currentState(result).lastPrompt).toBe("corporate deck");
    });

    // Phase 2: Flow A — pick skill → startChat with same brief
    const stream = new ReadableStream({
      start(controller) {
        controller.enqueue(
          new TextEncoder().encode(
            sseEvent("done", { project_id: "proj-test", version: "V1", total_rounds: 1 }),
          ),
        );
        controller.close();
      },
    });

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, body: stream }));

    await act(async () => {
      result.current.startChat("proj-test", "corporate deck");
    });

    await waitFor(() => {
      expect(currentState(result).streamStatus).toBe("done");
    });

    const userMessages = currentState(result).timeline.filter(e => e.event === "user_message");
    expect(userMessages.length).toBe(1);
    expect(userMessages[0].data.content).toBe("corporate deck");
  });

  it("preserves brief after done via loadChatHistory merge", async () => {
    // Phase 1: recommendStyles → sets lastPrompt + projectBriefRef
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({
          recommendations: [{ name: "coral-deck", description: "Warm theme", reason: "Matches", scenario: "presentation", has_assets: false }],
        }),
      }),
    );

    const { result } = renderHook(() => useChatStream());

    await act(async () => {
      await result.current.recommendStyles("proj-test", "brief");
    });

    await waitFor(() => {
      expect(currentState(result).lastPrompt).toBe("brief");
    });

    // Phase 2: /chat SSE with done, then /chat-history with refine-only timeline
    const sseStream = new ReadableStream({
      start(controller) {
        controller.enqueue(
          new TextEncoder().encode(
            sseEvent("done", { project_id: "proj-test", version: "V1", total_rounds: 1 }),
          ),
        );
        controller.close();
      },
    });

    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, body: sseStream })
      .mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          entries: [
            { event: "user_message", data: { content: "refine" }, timestamp: "2025-01-01T00:00:00Z" },
            { event: "thinking", data: { content: "ok" }, timestamp: "2025-01-01T00:00:01Z" },
            { event: "text", data: { content: "done" }, timestamp: "2025-01-01T00:00:02Z" },
          ],
        }),
      });
    vi.stubGlobal("fetch", fetchMock);

    await act(async () => {
      result.current.startChat("proj-test", "refine");
    });

    await waitFor(() => {
      expect(currentState(result).streamStatus).toBe("done");
    });

    // After done + loadChatHistory, should have one user_message with merged content
    const userMessages = currentState(result).timeline.filter(e => e.event === "user_message");
    expect(userMessages.length).toBe(1);
    expect(userMessages[0].data.content).toBe("brief\n\nrefine");
  });
});
