/** Standard error response shape returned by the FastAPI backend. */
export interface ApiErrorResponse {
  detail?: string;
  message?: string;
  error?: string;
  code?: string;
}

/**
 * Safely parse a JSON response body, returning an empty object on failure.
 * Replaces the ad-hoc `res.json().catch(() => ({}))` pattern found across the codebase.
 */
export async function safeJson<T = unknown>(res: Response): Promise<T> {
  try {
    return (await res.json()) as T;
  } catch {
    return {} as T;
  }
}

/**
 * Extract a human-readable error message from an API response.
 * Tries `detail`, then `message`, then `error`, then falls back to the HTTP status.
 */
export async function extractErrorMessage(res: Response): Promise<string> {
  const body = await safeJson<ApiErrorResponse>(res);
  return body.detail || body.message || body.error || `HTTP ${res.status}`;
}
