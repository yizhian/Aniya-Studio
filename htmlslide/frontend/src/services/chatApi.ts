import type { ChatRequestPayload } from "../models/chat";

export async function startChatSSE(
  body: ChatRequestPayload,
  signal: AbortSignal,
): Promise<Response> {
  return fetch("/api/v1/chat", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    signal,
  });
}

export async function fetchChatHistory(projectId: string): Promise<Response> {
  return fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/chat-history`);
}

export async function fetchProjectBrief(projectId: string): Promise<Response> {
  return fetch(`/api/v1/projects/${encodeURIComponent(projectId)}`);
}

export async function fetchStyleRecommendations(brief: string): Promise<Response> {
  return fetch("/api/v1/recommend-styles", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ brief }),
  });
}
