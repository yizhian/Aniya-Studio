import type { SaveVersionResponse, VersionRestoreResponse } from "../../models/editor";
import { extractErrorMessage } from "../../models/apiResponses";

export async function postSaveVersion(
  projectId: string,
  title: string,
  html: string,
): Promise<SaveVersionResponse> {
  const res = await fetch(
    `/api/v1/projects/${encodeURIComponent(projectId)}/versions`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title, html }),
    },
  );
  if (!res.ok) {
    throw new Error(await extractErrorMessage(res));
  }
  return res.json();
}

export async function postRestoreVersion(
  projectId: string,
  versionId: string,
): Promise<VersionRestoreResponse> {
  const res = await fetch(
    `/api/v1/projects/${encodeURIComponent(projectId)}/versions/${encodeURIComponent(versionId)}/restore`,
    { method: "POST" },
  );
  if (!res.ok) {
    throw new Error(await extractErrorMessage(res));
  }
  return res.json();
}
