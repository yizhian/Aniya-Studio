import type {
  AttachmentMeta,
  ChatState,
  DomContext,
  TimelineEntry,
  ChatError,
  SkillRecommendation,
} from "../models/chat";

// ─── Actions ───

export type ChatAction =
  | { type: "START"; userMessage: string; domContext?: DomContext; attachments?: AttachmentMeta[] }
  | { type: "LOAD_HISTORY"; timeline: TimelineEntry[]; lastPrompt?: string; lastSelectedDom?: DomContext; projectBrief?: string }
  | { type: "TODO_WRITE"; todos: Array<{ content: string; status: string; active_form: string }> }
  | { type: "THINKING"; content: string }
  | { type: "TEXT"; content: string }
  | { type: "TOOL_START"; callId: string; name: string }
  | { type: "TOOL_RESULT"; callId: string; name: string; success: boolean; summary: string; durationMs: number }
  | { type: "HOOK_WARN"; message: string }
  | { type: "SKILL_RECOMMENDATIONS"; recommendations: SkillRecommendation[]; lastPrompt: string; attachments?: AttachmentMeta[] }
  | { type: "CLEAR_SKILL_RECOMMENDATIONS" }
  | { type: "ERROR"; error: ChatError }
  | { type: "DONE"; meta: { projectId: string; version: string; totalRounds: number } }
  | { type: "ABORT" }
  | { type: "RESET" };

// ─── Helpers ───

export function makeTimelineEntry(event: TimelineEntry["event"], data: Record<string, any>): TimelineEntry {
  return { event, timestamp: new Date().toISOString(), data };
}

// ─── Initial state ───

export const initialChatState: ChatState = {
  streamStatus: "idle",
  timeline: [],
  error: null,
  doneMeta: null,
  skillRecommendations: null,
  lastPrompt: "",
  lastSelectedDom: undefined,
  projectBrief: "",
};

// ─── Reducer ───

export function chatReducer(state: ChatState, action: ChatAction): ChatState {
  switch (action.type) {
    case "START": {
      const brief = state.projectBrief || state.lastPrompt;
      const last = state.timeline[state.timeline.length - 1];
      const replacePending =
        last?.event === "user_message" &&
        state.streamStatus === "idle" &&
        !state.timeline.some((e) => e.event === "text" || e.event === "thinking" || e.event === "tool");
      const hasPriorUserMessage = state.timeline.some(e => e.event === "user_message");
      const shouldMergeBrief =
        Boolean(brief && brief !== action.userMessage && (!hasPriorUserMessage || replacePending));
      const displayContent = shouldMergeBrief
        ? `${brief}\n\n${action.userMessage}`
        : action.userMessage;

      const userEntry = makeTimelineEntry("user_message", {
        content: displayContent,
        dom_context: action.domContext,
        attachments: action.attachments?.length ? action.attachments : undefined,
      });

      const timeline = replacePending
        ? [...state.timeline.slice(0, -1), {
            ...userEntry,
            data: {
              ...userEntry.data,
              attachments:
                userEntry.data.attachments ??
                (last.data.attachments as AttachmentMeta[] | undefined),
            },
          }]
        : [...state.timeline, userEntry];

      return {
        ...state,
        streamStatus: "connecting",
        error: null,
        doneMeta: null,
        lastPrompt: action.userMessage,
        lastSelectedDom: action.domContext,
        skillRecommendations: null,
        timeline,
      };
    }

    case "LOAD_HISTORY": {
      const brief = action.projectBrief || state.projectBrief;
      let timeline = action.timeline;
      if (brief) {
        const idx = timeline.findIndex(e => e.event === "user_message");
        if (idx >= 0) {
          const content = timeline[idx].data?.content || "";
          if (content !== brief && !content.startsWith(`${brief}\n\n`)) {
            timeline = [...timeline];
            timeline[idx] = {
              ...timeline[idx],
              data: { ...timeline[idx].data, content: `${brief}\n\n${content}` },
            };
          }
        }
      }
      return {
        ...state,
        timeline,
        lastPrompt: action.lastPrompt ?? state.lastPrompt,
        lastSelectedDom: action.lastSelectedDom ?? state.lastSelectedDom,
        projectBrief: brief || state.projectBrief,
      };
    }

    case "THINKING": {
      const timeline = [...state.timeline];
      const last = timeline[timeline.length - 1];
      if (last && last.event === "thinking") {
        timeline[timeline.length - 1] = {
          ...last,
          data: { ...last.data, content: (last.data.content || "") + action.content },
        };
      } else {
        timeline.push(makeTimelineEntry("thinking", { content: action.content }));
      }
      return { ...state, streamStatus: "streaming", timeline };
    }

    case "TEXT": {
      const timeline = [...state.timeline];
      const last = timeline[timeline.length - 1];
      if (last && last.event === "text") {
        timeline[timeline.length - 1] = {
          ...last,
          data: { ...last.data, content: (last.data.content || "") + action.content },
        };
      } else {
        timeline.push(makeTimelineEntry("text", { content: action.content }));
      }
      return { ...state, streamStatus: "streaming", timeline };
    }

    case "TOOL_START": {
      return {
        ...state,
        streamStatus: "streaming",
        timeline: [...state.timeline, makeTimelineEntry("tool", {
          phase: "start",
          call_id: action.callId,
          name: action.name,
        })],
      };
    }

    case "TODO_WRITE":
      return {
        ...state,
        timeline: [...state.timeline, makeTimelineEntry("todo_write", {
          todos: action.todos,
        })],
      };

    case "TOOL_RESULT": {
      const timeline = state.timeline.map((entry) => {
        if (
          entry.event === "tool" &&
          entry.data.call_id === action.callId &&
          entry.data.phase === "start"
        ) {
          return {
            ...entry,
            data: {
              phase: "result",
              call_id: action.callId,
              name: action.name,
              success: action.success,
              summary: action.summary,
              duration_ms: action.durationMs,
            },
          };
        }
        return entry;
      });
      return { ...state, streamStatus: "streaming", timeline };
    }

    case "HOOK_WARN":
      return {
        ...state,
        streamStatus: "streaming",
        timeline: [...state.timeline, makeTimelineEntry("hook:warn", {
          message: action.message,
        })],
      };

    case "SKILL_RECOMMENDATIONS": {
      const hasUserMessage = state.timeline.some((e) => e.event === "user_message");
      const timeline = hasUserMessage
        ? state.timeline
        : [
            ...state.timeline,
            makeTimelineEntry("user_message", {
              content: action.lastPrompt,
              attachments: action.attachments?.length ? action.attachments : undefined,
            }),
          ];
      return {
        ...state,
        skillRecommendations: action.recommendations,
        lastPrompt: action.lastPrompt || state.lastPrompt,
        projectBrief: state.projectBrief || action.lastPrompt || "",
        timeline,
      };
    }

    case "CLEAR_SKILL_RECOMMENDATIONS":
      return {
        ...state,
        skillRecommendations: null,
      };

    case "ERROR":
      return {
        ...state,
        streamStatus: "error",
        error: action.error,
      };

    case "DONE":
      return {
        ...state,
        streamStatus: "done",
        doneMeta: action.meta,
      };

    case "ABORT":
      return { ...state, streamStatus: "idle" };

    case "RESET":
      return { ...initialChatState, projectBrief: "" };

    default:
      return state;
  }
}
