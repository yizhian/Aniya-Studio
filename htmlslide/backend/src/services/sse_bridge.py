import asyncio
import json
from typing import AsyncIterator

from src.config import settings
from src.repositories.version_repo import VersionRepo


def format_sse(event_type: str, data: dict) -> str:
    """Produce standard SSE format (event: line prefix style, frontend-compatible)."""
    lines = [
        f"event: {event_type}",
        f"data: {json.dumps(data, ensure_ascii=False)}",
        "",
    ]
    return "\n".join(lines) + "\n"


async def bridge_sse(
    agentgo_stream: AsyncIterator[bytes],
    project_id: str,
    version_repo: VersionRepo,
) -> AsyncIterator[str]:
    """Bridge AgentGo flat-JSON SSE to frontend standard SSE.

    Input: AgentGo raw SSE byte stream (httpx streaming response lines)
    Output: Frontend-standardized SSE string stream (event: line prefix format)
    """
    tool_starts: dict[str, dict] = {}
    max_round_seen = 0
    heartbeat_interval = settings.sse_heartbeat_interval

    aiter = agentgo_stream

    try:
        while True:
            try:
                line = await asyncio.wait_for(
                    aiter.__anext__(),
                    timeout=heartbeat_interval,
                )
            except TimeoutError:
                yield ": keepalive\n\n"
                continue
            except StopAsyncIteration:
                break

            if not line:
                continue

            line_str = line.decode("utf-8") if isinstance(line, bytes) else line

            if not line_str.startswith("data: "):
                continue

            payload = json.loads(line_str[6:])
            event_type = payload.get("type")
            event_data = payload.get("data", {})
            round_num = payload.get("round", 0)
            max_round_seen = max(max_round_seen, round_num)

            match event_type:
                case "thinking":
                    yield format_sse("thinking", {
                        "content": event_data.get("text", ""),
                    })

                case "text":
                    yield format_sse("text", {
                        "content": event_data.get("text", ""),
                    })

                case "tool_call_start":
                    call_id = event_data.get("id")
                    if not call_id:
                        continue
                    tool_starts[call_id] = {
                        "name": event_data.get("name", "unknown"),
                        "tool_call_index": event_data.get("tool_call_index", 0),
                    }
                    yield format_sse("tool", {
                        "phase": "start",
                        "name": event_data.get("name"),
                        "call_id": event_data.get("id"),
                    })

                case "tool_call_complete":
                    pass  # not forwarded to frontend

                case "tool_result":
                    if not event_data.get("is_error"):
                        call_id = event_data.get("id", "")
                        ctx = tool_starts.pop(call_id, None)
                        content = event_data.get("content", "")
                        summary = content[:200] if len(content) > 200 else content
                        tool_event = {
                            "phase": "result",
                            "name": ctx["name"] if ctx else event_data.get("name", "unknown"),
                            "success": True,
                            "summary": summary,
                            "duration_ms": event_data.get("duration_ms", 0),
                        }
                        if ctx is None:
                            tool_event["orphan"] = True
                        yield format_sse("tool", tool_event)

                case "round":
                    pass

                case "round_end":
                    pass

                case "todo_write":
                    yield format_sse("todo_write", {
                        "todos": event_data.get("todos", []),
                    })

                case "error":
                    if event_data.get("tool_name"):
                        pass  # ponytail: tool errors logged to Docker, not shown in chat
                    else:
                        tool_starts.clear()
                        yield format_sse("error", {
                            "code": "agent_error",
                            "message": event_data.get("message", "Agent internal error"),
                            "recoverable": True,
                        })

    except Exception:
        tool_starts.clear()
        yield format_sse("error", {
            "code": "agent_unavailable",
            "message": "Agent connection unexpectedly dropped",
            "recoverable": False,
        })
        # Fall through to done synthesis — let the frontend load whatever content
        # the agent managed to write before the connection dropped.

    tool_starts.clear()
    try:
        latest_id = await version_repo.get_latest_version_id()
    except Exception:
        latest_id = None

    yield format_sse("done", {
        "project_id": project_id,
        "version": latest_id or "unknown",
        "total_rounds": max_round_seen,
    })
