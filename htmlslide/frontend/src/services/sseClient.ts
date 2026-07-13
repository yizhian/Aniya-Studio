import type { SSEEvent } from "../models/chat";

/**
 * Parse a complete SSE text chunk into structured events.
 * Handles multi-event chunks split by double-newlines.
 * Ignores comment lines (starting with ":").
 */
export function parseSSEChunk(chunk: string): SSEEvent[] {
  const events: SSEEvent[] = [];
  const parts = chunk.split("\n\n");

  for (const part of parts) {
    if (!part.trim()) continue;

    const lines = part.split("\n");
    let eventType = "message";
    const dataLines: string[] = [];

    for (const line of lines) {
      if (line.startsWith("event: ")) {
        eventType = line.slice(7).trim();
      } else if (line.startsWith("data: ")) {
        dataLines.push(line.slice(6));
      }
      // comment lines (": ...") and empty lines are ignored
    }

    if (dataLines.length > 0) {
      events.push({
        event: eventType,
        data: dataLines.join("\n"),
      });
    }
  }

  return events;
}

/**
 * Async generator that yields SSEEvent objects from a fetch Response body.
 * Preserves incomplete chunks across iterations via an internal buffer.
 * Rejects if the response status is not OK.
 */
export async function* streamSSEEvents(
  response: Response,
  signal: AbortSignal,
): AsyncGenerator<SSEEvent> {
  if (!response.ok) {
    const body = await response.text().catch(() => "");
    let message = `HTTP ${response.status}`;
    try {
      const parsed = JSON.parse(body);
      if (parsed?.detail?.message) message = parsed.detail.message;
      else if (parsed?.detail) message = String(parsed.detail);
    } catch {
      // non-JSON error body — use status text
    }
    throw new Error(message);
  }

  if (!response.body) {
    throw new Error("Response body is empty");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8");
  let buffer = "";

  try {
    while (true) {
      if (signal.aborted) break;

      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      // Emit complete events (delimited by \n\n), keep the tail fragment
      const lastNewline = buffer.lastIndexOf("\n\n");
      if (lastNewline === -1) continue;

      const complete = buffer.slice(0, lastNewline + 2);
      buffer = buffer.slice(lastNewline + 2);

      for (const event of parseSSEChunk(complete)) {
        yield event;
      }
    }

    // Flush remaining buffer on clean stream end
    if (buffer.trim()) {
      for (const event of parseSSEChunk(buffer)) {
        yield event;
      }
    }
  } finally {
    reader.releaseLock();
  }
}
