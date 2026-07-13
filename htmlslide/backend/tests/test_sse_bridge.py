import asyncio
import json
import tempfile
from pathlib import Path

import pytest

from src.repositories.version_repo import VersionRepo
from src.services.sse_bridge import bridge_sse, format_sse


class TestFormatSSE:
    def test_format_thinking(self):
        result = format_sse("thinking", {"content": "hello"})
        assert result.startswith("event: thinking\n")
        assert "data: " in result
        assert result.endswith("\n\n")

    def test_format_done(self):
        result = format_sse("done", {"project_id": "p1", "version": "v001"})
        parsed = {}
        for line in result.strip().split("\n"):
            if line.startswith("event: "):
                parsed["event"] = line[7:]
            elif line.startswith("data: "):
                parsed["data"] = json.loads(line[6:])
        assert parsed["event"] == "done"
        assert parsed["data"]["project_id"] == "p1"


async def _make_stream(lines: list[str]):
    """Simulate AgentGo raw byte stream."""
    for line in lines:
        yield line.encode("utf-8")


class TestBridgeSSE:
    def _make_version_repo(self):
        tmpdir = tempfile.mkdtemp()
        return VersionRepo(Path(tmpdir), "test-project")

    @pytest.mark.asyncio
    async def test_thinking_event(self):
        vr = self._make_version_repo()
        stream = _make_stream([
            'data: {"type":"thinking","data":{"text":"Analyzing..."},"round":1}',
        ])
        events = []
        async for evt in bridge_sse(stream, "proj-1", vr):
            events.append(evt)

        # Should have thinking + done
        assert any("event: thinking" in e for e in events)
        assert any("event: done" in e for e in events)

    @pytest.mark.asyncio
    async def test_text_event(self):
        vr = self._make_version_repo()
        stream = _make_stream([
            'data: {"type":"text","data":{"text":"Generated HTML"},"round":1}',
        ])
        events = []
        async for evt in bridge_sse(stream, "proj-1", vr):
            events.append(evt)

        assert any("event: text" in e for e in events)
        assert any("Generated HTML" in e for e in events)

    @pytest.mark.asyncio
    async def test_tool_start_and_result(self):
        vr = self._make_version_repo()
        stream = _make_stream([
            'data: {"type":"tool_call_start","data":{"name":"write_file","id":"t1","tool_call_index":0},"round":2}',
            'data: {"type":"tool_result","data":{"id":"t1","content":"done","is_error":false,"duration_ms":120},"round":2}',
        ])
        events = []
        async for evt in bridge_sse(stream, "proj-1", vr):
            events.append(evt)

        tool_events = [e for e in events if "event: tool" in e]
        assert len(tool_events) == 2
        assert any("start" in e for e in tool_events)
        assert any("result" in e for e in tool_events)

    @pytest.mark.asyncio
    async def test_tool_call_complete_filtered(self):
        vr = self._make_version_repo()
        stream = _make_stream([
            'data: {"type":"tool_call_complete","data":{"arguments":"..."},"round":2}',
        ])
        events = []
        async for evt in bridge_sse(stream, "proj-1", vr):
            events.append(evt)
        # tool_call_complete should be filtered (only done remains)
        tool_events = [e for e in events if "event: tool" in e]
        assert len(tool_events) == 0

    @pytest.mark.asyncio
    async def test_round_filtered(self):
        vr = self._make_version_repo()
        stream = _make_stream([
            'data: {"type":"round","data":{},"round":1}',
        ])
        events = []
        async for evt in bridge_sse(stream, "proj-1", vr):
            events.append(evt)
        # round and round_end should be filtered (only done remains)
        non_done = [e for e in events if "event: done" not in e and not e.startswith(":")]
        assert len(non_done) == 0

    @pytest.mark.asyncio
    async def test_done_synthesized_after_stream(self):
        vr = self._make_version_repo()
        stream = _make_stream([
            'data: {"type":"thinking","data":{"text":"Done"},"round":1}',
        ])
        events = []
        async for evt in bridge_sse(stream, "proj-123", vr):
            events.append(evt)

        assert any("event: done" in e for e in events)
        done = [e for e in events if "event: done" in e]
        assert len(done) == 1
        data_line = [l for l in done[0].split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert data["project_id"] == "proj-123"

    @pytest.mark.asyncio
    async def test_error_event(self):
        vr = self._make_version_repo()
        stream = _make_stream([
            'data: {"type":"error","data":{"message":"Something failed"},"round":1}',
        ])
        events = []
        async for evt in bridge_sse(stream, "proj-1", vr):
            events.append(evt)

        assert any("event: error" in e for e in events)
        assert any("agent_error" in e for e in events)

    @pytest.mark.asyncio
    async def test_heartbeat_keepalive(self):
        vr = self._make_version_repo()

        async def slow_stream():
            yield 'data: {"type":"thinking","data":{"text":"Hmm"},"round":1}'.encode()
            # Sleep long enough to trigger heartbeat (but short enough for test)
            await asyncio.sleep(0.1)
            raise StopAsyncIteration

        # Override heartbeat for fast test
        import src.services.sse_bridge as sse_mod
        old_interval = sse_mod.settings.sse_heartbeat_interval
        sse_mod.settings.sse_heartbeat_interval = 0.05

        try:
            events = []
            async for evt in bridge_sse(slow_stream(), "p1", vr):
                events.append(evt)
            # Should have thinking + keepalive + done
            keepalives = [e for e in events if ": keepalive" in e]
            assert len(keepalives) >= 1
        finally:
            sse_mod.settings.sse_heartbeat_interval = old_interval
