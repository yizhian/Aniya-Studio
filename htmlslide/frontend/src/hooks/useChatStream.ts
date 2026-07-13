import { useCallback, useReducer, useRef } from "react";
import type {
  AttachmentMeta,
  DomContext,
  TimelineEntry,
  ChatRequestPayload,
  DoneMeta,
  ChatError,
} from "../models/chat";
import { streamSSEEvents } from "../services/sseClient";
import { useLocale } from "../context/LocaleContext";
import {
  chatReducer,
  initialChatState,
  type ChatAction,
} from "../services/chatReducer";
import {
  startChatSSE,
  fetchChatHistory,
  fetchProjectBrief,
  fetchStyleRecommendations,
} from "../services/chatApi";

interface UseChatStreamOptions {
  onDone?: (meta: DoneMeta) => void;
  onError?: (error: ChatError) => void;
}

export function useChatStream(options: UseChatStreamOptions = {}) {
  const { onDone, onError } = options;
  const { t } = useLocale();
  const [chatState, dispatch] = useReducer(chatReducer, initialChatState);
  const abortRef = useRef<AbortController | null>(null);
  const projectIdRef = useRef<string>("");
  const loadChatHistoryRef = useRef<(projectId: string) => Promise<void>>();
  const projectBriefRef = useRef<string>("");
  const onDoneRef = useRef(onDone);
  onDoneRef.current = onDone;
  const onErrorRef = useRef(onError);
  onErrorRef.current = onError;

  const stopChat = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    dispatch({ type: "ABORT" });
  }, []);

  const resetChat = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    dispatch({ type: "RESET" });
  }, []);

  const clearSkillRecommendations = useCallback(() => {
    dispatch({ type: "CLEAR_SKILL_RECOMMENDATIONS" });
  }, []);

  const startChat = useCallback(
    async (
      projectId: string,
      prompt: string,
      selectedDom?: ChatRequestPayload["selected_dom"],
      attachments?: AttachmentMeta[],
    ) => {
      abortRef.current?.abort();

      const controller = new AbortController();
      abortRef.current = controller;
      projectIdRef.current = projectId;

      dispatch({ type: "START", userMessage: prompt, domContext: selectedDom, attachments });

      const body: ChatRequestPayload = {
        project_id: projectId,
        prompt,
      };
      if (selectedDom) body.selected_dom = selectedDom;
      if (attachments?.length) body.attachments = attachments;

      try {
        const response = await startChatSSE(body, controller.signal);

        for await (const sseEvent of streamSSEEvents(response, controller.signal)) {
          const data = JSON.parse(sseEvent.data);

          switch (sseEvent.event) {
            case "thinking":
              dispatch({ type: "THINKING", content: data.content });
              break;
            case "text":
              dispatch({ type: "TEXT", content: data.content });
              break;
            case "tool":
              if (data.phase === "start") {
                dispatch({ type: "TOOL_START", callId: data.call_id, name: data.name });
              } else {
                dispatch({
                  type: "TOOL_RESULT",
                  callId: data.call_id,
                  name: data.name,
                  success: data.success,
                  summary: data.summary || "",
                  durationMs: data.duration_ms || 0,
                });
              }
              break;
            case "todo_write":
              dispatch({ type: "TODO_WRITE", todos: data.todos || [] });
              break;
            case "hook:warn":
              dispatch({ type: "HOOK_WARN", message: (data.message as string) || "" });
              break;
            case "error": {
              const chatError: ChatError = {
                code: data.code || "unknown",
                message: data.message || t.errors.unknownError,
                recoverable: data.recoverable ?? false,
              };
              dispatch({ type: "ERROR", error: chatError });
              onErrorRef.current?.(chatError);
              break;
            }
            case "done": {
              const meta: DoneMeta = {
                projectId: data.project_id,
                version: data.version || "unknown",
                totalRounds: data.total_rounds || 0,
              };
              dispatch({ type: "DONE", meta });
              onDoneRef.current?.(meta);
              loadChatHistoryRef.current?.(projectIdRef.current);
              return;
            }
          }
        }
      } catch (err: unknown) {
        if (err instanceof DOMException && err.name === "AbortError") return;
        const chatError: ChatError = {
          code: "stream_error",
          message: err instanceof Error ? err.message : t.errors.sseDisconnected,
          recoverable: true,
        };
        dispatch({ type: "ERROR", error: chatError });
        onErrorRef.current?.(chatError);
      }
    },
    [],
  );

  const loadChatHistory = useCallback(async (projectId: string) => {
    try {
      const response = await fetchChatHistory(projectId);
      if (!response.ok) return;
      const data = await response.json();
      const entries: TimelineEntry[] = data.entries || [];
      if (entries.length === 0) return;

      // Merge tool start+result pairs and consecutive thinking/text entries.
      const timeline: TimelineEntry[] = [];
      for (const e of entries) {
        const evt = e.event;
        if (evt === "tool" && e.data?.phase === "result") {
          let merged = false;
          for (let i = timeline.length - 1; i >= 0; i--) {
            const prev = timeline[i];
            if (
              prev.event === "tool" &&
              prev.data?.call_id === e.data.call_id &&
              prev.data?.phase === "start"
            ) {
              timeline[i] = { ...prev, data: { ...e.data, phase: "result" } };
              merged = true;
              break;
            }
          }
          if (!merged) timeline.push(e);
        } else if (evt === "thinking" || evt === "text") {
          const last = timeline[timeline.length - 1];
          if (last && last.event === evt) {
            last.data.content = (last.data.content || "") + (e.data?.content || "");
          } else {
            timeline.push(e);
          }
        } else {
          timeline.push(e);
        }
      }

      // Resolve project brief for merge.
      let brief = projectBriefRef.current;
      if (!brief) {
        try {
          const metaResp = await fetchProjectBrief(projectId);
          if (metaResp.ok) {
            const meta = await metaResp.json();
            if (meta.brief) {
              brief = meta.brief;
              projectBriefRef.current = brief;
            }
          }
        } catch {
          // Silently ignore
        }
      }

      let histLastPrompt = "";
      let histLastDom: DomContext | undefined;
      for (let i = entries.length - 1; i >= 0; i--) {
        if (entries[i].event === "user_message") {
          histLastPrompt = entries[i].data?.content || "";
          histLastDom = entries[i].data?.dom_context;
          break;
        }
      }

      dispatch({
        type: "LOAD_HISTORY",
        timeline,
        lastPrompt: histLastPrompt,
        lastSelectedDom: histLastDom,
        projectBrief: brief || undefined,
      });

      // Restore skill recommendations if not yet acted upon.
      let lastSkillRecIdx = -1;
      let lastHTMLWriteIdx = -1;
      for (let i = entries.length - 1; i >= 0; i--) {
        const e = entries[i];
        if (e.event === "skill_recommendations" && lastSkillRecIdx === -1) lastSkillRecIdx = i;
        if (
          e.event === "tool" &&
          e.data?.name === "write_file" &&
          e.data?.success &&
          typeof e.data?.summary === "string" &&
          e.data.summary.includes(".html") &&
          lastHTMLWriteIdx === -1
        ) {
          lastHTMLWriteIdx = i;
        }
      }

      if (lastSkillRecIdx > lastHTMLWriteIdx) {
        const recEntry = entries[lastSkillRecIdx];
        const recs = recEntry.data?.recommendations;
        if (recs && recs.length > 0) {
          let skillRecPrompt = histLastPrompt;
          for (let j = lastSkillRecIdx - 1; j >= 0; j--) {
            if (entries[j].event === "user_message") {
              skillRecPrompt = entries[j].data?.content || "";
              break;
            }
          }
          dispatch({
            type: "SKILL_RECOMMENDATIONS",
            recommendations: recs,
            lastPrompt: skillRecPrompt,
          });
        }
      }
    } catch {
      // Silently ignore
    }
  }, []);

  const recommendStyles = useCallback(async (_projectId: string, brief: string, attachments?: AttachmentMeta[]) => {
    projectBriefRef.current = brief;
    try {
      const response = await fetchStyleRecommendations(brief);
      if (!response.ok) return;
      const data = await response.json();
      dispatch({
        type: "SKILL_RECOMMENDATIONS",
        recommendations: data.recommendations || [],
        lastPrompt: brief,
        attachments,
      });
    } catch {
      // Silently ignore
    }
  }, []);

  loadChatHistoryRef.current = loadChatHistory;

  return { chatState, startChat, stopChat, resetChat, loadChatHistory, clearSkillRecommendations, recommendStyles };
}
