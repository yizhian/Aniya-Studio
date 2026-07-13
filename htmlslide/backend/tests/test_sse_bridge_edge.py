"""SSE bridge edge cases and uncovered code paths."""
import json
import tempfile
from pathlib import Path

import pytest

from src.repositories.version_repo import VersionRepo
from src.services.sse_bridge import bridge_sse


async def _make_stream(lines: list[str]):
    for line in lines:
        yield line.encode("utf-8")


def _make_version_repo():
    tmpdir = tempfile.mkdtemp()
    return VersionRepo(Path(tmpdir), "test-project")


class TestSSEBridgeEdges:
    @pytest.mark.asyncio
    async def test_empty_line_skipped(self):
        """Coverage: line 48-49 — empty line ignored."""
        vr = _make_version_repo()
        stream = _make_stream([
            "",
            'data: {"type":"thinking","data":{"text":"ok"},"round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        assert len([e for e in events if "event: thinking" in e]) == 1

    @pytest.mark.asyncio
    async def test_non_data_line_skipped(self):
        """Coverage: line 53-54 — non data: line ignored."""
        vr = _make_version_repo()
        stream = _make_stream([
            'event: open',  # style with event: prefix should be skipped
            'data: {"type":"text","data":{"text":"hi"},"round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        assert len([e for e in events if "event: text" in e]) == 1

    @pytest.mark.asyncio
    async def test_orphan_tool_result(self):
        """Coverage: line 99-100 — tool_result without matching start (orphan)."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {"type":"tool_result","data":{"id":"orphan1","content":"result","is_error":false,"duration_ms":50},"round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        tool_events = [e for e in events if "event: tool" in e]
        assert len(tool_events) == 1
        # Check orphan flag
        for evt in tool_events:
            data_line = [l for l in evt.split("\n") if l.startswith("data: ")][0]
            data = json.loads(data_line[6:])
            assert data["orphan"] is True
            assert data["phase"] == "result"

    @pytest.mark.asyncio
    async def test_round_end_filtered(self):
        """Coverage: line 106-107 — round_end filtered."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {"type":"round_end","data":{},"round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        non_keepalive = [e for e in events if not e.startswith(":")]
        # Only done should remain
        assert all("event: done" in e for e in non_keepalive)

    @pytest.mark.asyncio
    async def test_stream_exception_produces_error(self):
        """Coverage: line 117-124 — stream exception yields agent_unavailable."""
        vr = _make_version_repo()

        async def broken_stream():
            yield 'data: {"type":"thinking","data":{"text":"start"},"round":1}'.encode()
            raise RuntimeError("Connection reset by peer")

        events = [e async for e in bridge_sse(broken_stream(), "p1", vr)]
        error_events = [e for e in events if "event: error" in e]
        assert len(error_events) == 1
        data_line = [l for l in error_events[0].split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert data["code"] == "agent_unavailable"
        assert data["recoverable"] is False
        # Done event should still be emitted so frontend can load whatever content exists
        assert any("event: done" in e for e in events)

    @pytest.mark.asyncio
    async def test_get_latest_version_id_exception_in_done(self, monkeypatch):
        """Coverage: line 129-130 — version lookup raises, done uses 'unknown'."""
        vr = _make_version_repo()

        async def _raise(*args, **kwargs):
            raise OSError("Disk read error")

        monkeypatch.setattr(vr, "get_latest_version_id", _raise)
        stream = _make_stream([
            'data: {"type":"text","data":{"text":"ok"},"round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        done_events = [e for e in events if "event: done" in e]
        assert len(done_events) == 1
        data_line = [l for l in done_events[0].split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert data["version"] == "unknown"

    @pytest.mark.asyncio
    async def test_tool_result_long_content_truncated(self):
        """tool_result content > 200 chars should be truncated to summary."""
        vr = _make_version_repo()
        long_content = "x" * 500
        line = json.dumps({
            "type": "tool_result",
            "data": {
                "id": "t1",
                "content": long_content,
                "is_error": False,
                "duration_ms": 100,
            },
            "round": 1,
        })
        stream = _make_stream([
            'data: {"type":"tool_call_start","data":{"name":"write","id":"t1"},"round":1}',
            f"data: {line}",
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        result_events = [e for e in events if '"phase": "result"' in e]
        assert len(result_events) == 1
        data_line = [l for l in result_events[0].split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert len(data["summary"]) == 200

    @pytest.mark.asyncio
    async def test_tool_error_result(self):
        """tool_result with is_error=True should have success=False."""
        vr = _make_version_repo()
        line = json.dumps({
            "type": "tool_result",
            "data": {
                "id": "t1",
                "content": "command failed",
                "is_error": True,
                "duration_ms": 500,
            },
            "round": 1,
        })
        stream = _make_stream([
            'data: {"type":"tool_call_start","data":{"name":"exec","id":"t1"},"round":1}',
            f"data: {line}",
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        result_events = [e for e in events if '"phase": "result"' in e]
        assert len(result_events) == 1
        data_line = [l for l in result_events[0].split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert data["success"] is False

    @pytest.mark.asyncio
    async def test_round_counting(self):
        """max_round_seen should be tracked correctly."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {"type":"thinking","data":{"text":"r1"},"round":1}',
            'data: {"type":"text","data":{"text":"r2"},"round":2}',
            'data: {"type":"text","data":{"text":"r3"},"round":3}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        done_events = [e for e in events if "event: done" in e]
        data_line = [l for l in done_events[0].split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert data["total_rounds"] == 3

    @pytest.mark.asyncio
    async def test_unknown_event_type_ignored(self):
        """Unknown AgentGo event types should be silently ignored."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {"type":"unknown_future_event","data":{"foo":"bar"},"round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        # Only done event should be produced
        non_keepalive = [e for e in events if not e.startswith(":")]
        assert len(non_keepalive) == 1
        assert "event: done" in non_keepalive[0]
