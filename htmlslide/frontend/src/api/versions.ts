export async function getVersions(projectId: string): Promise<Response> {
  return fetch(`/api/v1/projects/${encodeURIComponent(projectId)}/versions`);
}

export async function restoreVersion(
  projectId: string,
  versionId: string,
): Promise<Response> {
  return fetch(
    `/api/v1/projects/${encodeURIComponent(projectId)}/versions/${encodeURIComponent(versionId)}/restore`,
    { method: "POST" },
  );
}
