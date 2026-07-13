export async function precipitateStream(
  projectId: string,
  htmlContent: string,
): Promise<Response> {
  return fetch("/api/v1/skills/precipitate/stream", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ project_id: projectId, html_content: htmlContent }),
  });
}

export async function precipitateConfirm(payload: {
  project_id: string;
  skill_name: string;
  scenario: string;
  skill_md: string;
  example_html: string;
}): Promise<Response> {
  return fetch("/api/v1/skills/precipitate/confirm", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

export async function fetchSkillList(mode: string): Promise<Response> {
  return fetch(`/api/v1/skills?mode=${encodeURIComponent(mode)}`);
}

export async function fetchSkillContent(skillName: string): Promise<Response> {
  return fetch(`/api/v1/skills/${encodeURIComponent(skillName)}/content`);
}
