export async function testConnection(payload: object): Promise<{
  ok: boolean;
  message: string;
  inList?: boolean;
  verified?: boolean;
}> {
  const res = await fetch("/api/v1/settings/test-connection", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json();
  return {
    ok: data.ok ?? false,
    message: data.message ?? "Unknown response",
    inList: data.in_list,
    verified: data.verified,
  };
}

export async function syncToAgent(payload: object): Promise<{
  ok: boolean;
  message: string;
}> {
  const res = await fetch("/api/v1/settings/sync", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json();
  return { ok: data.ok ?? false, message: data.message ?? "Unknown response" };
}

export async function fetchModels(payload: object): Promise<{
  models: string[];
  source: string;
  error?: string;
}> {
  const res = await fetch("/api/v1/settings/models", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await res.json();
  return {
    models: data.models ?? [],
    source: data.source ?? "none",
    error: data.error,
  };
}
