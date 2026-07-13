"""P5-14~17: Resilience tests — AgentGo crash, keepalive, concurrent requests.

P5-14 (AgentGo unreachable) and P5-16 (keepalive) are already covered by:
  - tests/test_chat_api.py::test_chat_agent_unavailable (P5-14)
  - tests/test_sse_bridge.py::test_heartbeat_keepalive (P5-16)

This file covers P5-15 (mid-stream crash) and P5-17 (concurrent requests).
"""
import asyncio
import json
import tempfile
import threading
import time
from pathlib import Path

import pytest
import respx
from httpx import Response


# ─── P5-15: Mid-stream crash ───


class TestMidStreamCrash:
    """When the AgentGo stream raises mid-flight, BFF emits an error SSE event."""

    @pytest.mark.asyncio
    async def test_stream_exception_yields_error_event(self):
        """bridge_sse catches exceptions from the agent stream and emits error."""
        from src.repositories.version_repo import VersionRepo
        from src.services.sse_bridge import bridge_sse

        vr = VersionRepo(Path(tempfile.mkdtemp()), "test-project")

        async def crashing_stream():
            yield b'data: {"type":"thinking","data":{"text":"Starting..."},"round":1}\n'
            raise ConnectionError("TCP connection reset")

        events = []
        async for evt in bridge_sse(crashing_stream(), "proj-crash", vr):
            events.append(evt)

        # First event should be the thinking event
        assert any("event: thinking" in e for e in events)

        # Should have error event for the crash
        error_events = [e for e in events if "event: error" in e]
        assert len(error_events) == 1
        assert "agent_unavailable" in error_events[0]
        assert "dropped" in error_events[0].lower()
        assert "recoverable" in error_events[0]
        # Recoverable should be false for connection drops
        data_line = [l for l in error_events[0].split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert data["recoverable"] is False

        # Should still emit done event so frontend can load whatever content exists
        done_events = [e for e in events if "event: done" in e]
        assert len(done_events) == 1

    @pytest.mark.asyncio
    async def test_stream_exception_clears_tool_state(self):
        """Tool call starts should be cleared on stream exception."""
        from src.repositories.version_repo import VersionRepo
        from src.services.sse_bridge import bridge_sse

        vr = VersionRepo(Path(tempfile.mkdtemp()), "test-project")

        async def crashing_stream():
            yield b'data: {"type":"tool_call_start","data":{"name":"write_file","id":"t1","tool_call_index":0},"round":1}\n'
            yield b'data: {"type":"thinking","data":{"text":"mid thought"},"round":1}\n'
            raise BrokenPipeError("Connection lost")

        events = []
        async for evt in bridge_sse(crashing_stream(), "p1", vr):
            events.append(evt)

        # Should have tool start
        assert any(
            "event: tool" in e and "start" in e for e in events
        )

        # Should have both error and done (bridge always emits done so frontend can load content)
        assert any("event: error" in e for e in events)
        assert any("event: done" in e for e in events)


# ─── P5-17: Concurrent chat requests ───


class TestConcurrentChat:
    """Two concurrent POST /chat requests for the same project — no server 500."""

    def test_concurrent_chat_requests_no_crash(self, api_client):
        """Simultaneous chat requests don't cause server errors."""
        # Create project
        resp = api_client.post("/api/v1/projects", json={"name": "Concurrent"})
        assert resp.status_code == 201
        project_id = resp.json()["id"]

        errors: list[Exception] = []
        results: list[int] = []

        def do_chat():
            try:
                with respx.mock:
                    # Mock AgentGo with a slow response
                    sse_body = (
                        'data: {"type":"thinking","data":{"text":"thinking..."},"round":1}\n'
                        'data: {"type":"text","data":{"text":"result"},"round":1}\n'
                    )
                    respx.post("http://agentgo:8080/chat").mock(
                        return_value=Response(200, content=sse_body)
                    )

                    with api_client.stream(
                        "POST",
                        "/api/v1/chat",
                        json={
                            "project_id": project_id,
                            "prompt": "hello",
                        },
                    ) as r:
                        results.append(r.status_code)
                        # Consume stream
                        for _ in r.iter_lines():
                            pass
            except Exception as exc:
                errors.append(exc)

        # Launch two threads concurrently
        t1 = threading.Thread(target=do_chat)
        t2 = threading.Thread(target=do_chat)
        t1.start()
        t2.start()
        t1.join()
        t2.join()

        assert len(errors) == 0, f"Concurrent chat caused exceptions: {errors}"
        assert all(status == 200 for status in results), f"Non-200 statuses: {results}"
        assert len(results) == 2
