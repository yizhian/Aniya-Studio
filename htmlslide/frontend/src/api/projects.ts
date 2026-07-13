import type { ParsedFile } from "../services/upload";
import { extractErrorMessage } from "../models/apiResponses";

export async function createProject(name: string): Promise<{ id: string }> {
  const res = await fetch("/api/v1/projects", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  if (!res.ok) {
    throw new Error(await extractErrorMessage(res));
  }
  return res.json();
}

export async function updateProject(
  projectId: string,
  payload: Record<string, string>,
): Promise<void> {
  const res = await fetch(`/api/v1/projects/${encodeURIComponent(projectId)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    throw new Error(await extractErrorMessage(res));
  }
}

export async function fetchProjectUploads(
  agentgoBaseUrl: string,
  projectId: string,
): Promise<ParsedFile[]> {
  const res = await fetch(
    `${agentgoBaseUrl}/projects/${encodeURIComponent(projectId)}/uploads`,
  );
  if (!res.ok) return [];
  const data = await res.json();
  return data.files ?? [];
}
