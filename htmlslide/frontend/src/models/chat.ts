/** SSE stream lifecycle */
type StreamStatus = "idle" | "connecting" | "streaming" | "done" | "error";

/** All user-facing timeline event types from the AgentGo unified timeline. */
type TimelineEventType =
  | "user_message"
  | "thinking"
  | "text"
  | "tool"
  | "todo_write"
  | "hook:warn"
  | "skill_recommendations";

/** A single timeline entry — the canonical ordered record of a conversation. */
export interface TimelineEntry {
  event: TimelineEventType;
  timestamp: string;
  data: Record<string, any>;
}

export interface SkillRecommendation {
  name: string;
  description: string;
  reason: string;
  scenario?: string;
  has_assets?: boolean;
  has_preview?: boolean;
}

/** Central chat state — driven by SSE events via useChatStream reducer */
export interface ChatState {
  streamStatus: StreamStatus;
  timeline: TimelineEntry[];
  error: ChatError | null;
  doneMeta: DoneMeta | null;
  skillRecommendations: SkillRecommendation[] | null;
  lastPrompt: string;
  lastSelectedDom: DomContext | undefined;
  projectBrief: string;
}

export interface ChatError {
  code: string;
  message: string;
  recoverable: boolean;
}

export interface DoneMeta {
  projectId: string;
  version: string;
  totalRounds: number;
}

/** Parsed SSE event from BFF */
export interface SSEEvent {
  event: string;
  data: string;
}

/** Attachment snapshot stored on user_message timeline entries. */
export interface AttachmentMeta {
  original_name: string;
  saved_path_rel?: string;
  type?: string;
  pages?: number;
  char_count?: number;
  width?: number;
  height?: number;
  format?: string;
  error?: string;
}

/** BFF POST /chat request body — matches backend ChatRequest schema */
export interface ChatRequestPayload {
  project_id: string;
  prompt: string;
  selected_dom?: DomContext;
  attachments?: AttachmentMeta[];
}

export interface DomContext {
  css_path: string;
  tag: string;
  text: string;
  styles: Record<string, string>;
}

/** BFF ProjectResponse */
export interface ProjectResponse {
  id: string;
  name: string;
  current_version: string | null;
  file_size_bytes: number;
  slide_count: number;
  has_html: boolean;
  brief: string;
  design_skill: string;
  created_at: string;
}

