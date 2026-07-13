"""Additional SSE bridge edge tests: malformed JSON, missing fields, unknown types."""
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


class TestSSEBridgeMalformed:
    @pytest.mark.asyncio
    async def test_malformed_json_produces_error(self):
        """Invalid JSON in data: line should be caught by outer except → agent_unavailable."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {invalid json!!!}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        error_events = [e for e in events if "event: error" in e]
        assert len(error_events) == 1
        data_line = [l for l in error_events[0].split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert data["code"] == "agent_unavailable"
        assert data["recoverable"] is False

    @pytest.mark.asyncio
    async def test_missing_type_field(self):
        """SSE event with no 'type' field should be silently ignored (no match case)."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {"data":{"text":"no type field"},"round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        non_keepalive = [e for e in events if not e.startswith(":")]
        assert len(non_keepalive) == 1
        assert "event: done" in non_keepalive[0]

    @pytest.mark.asyncio
    async def test_type_is_none(self):
        """type: null should not match any case and be ignored."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {"type":null,"data":{"text":"x"},"round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        non_keepalive = [e for e in events if not e.startswith(":")]
        assert len(non_keepalive) == 1  # only done

    @pytest.mark.asyncio
    async def test_tool_call_start_missing_id(self):
        """tool_call_start without 'id' key → silently skipped (not a crash)."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {"type":"tool_call_start","data":{"name":"write"},"round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        # No error event — the malformed tool_call_start is silently skipped
        error_events = [e for e in events if "event: error" in e]
        assert len(error_events) == 0
        # Stream completes normally with done
        done_events = [e for e in events if "event: done" in e]
        assert len(done_events) == 1

    @pytest.mark.asyncio
    async def test_data_field_not_a_dict(self):
        """data field that's not a dict — .get() on string raises AttributeError → agent_unavailable."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {"type":"text","data":"just a string","round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        error_events = [e for e in events if "event: error" in e]
        assert len(error_events) == 1
        data_line = [l for l in error_events[0].split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert data["code"] == "agent_unavailable"

    @pytest.mark.asyncio
    async def test_round_num_not_an_int(self):
        """round field that's not an int — max(0, 'abc') raises TypeError → outer except."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {"type":"text","data":{"text":"ok"},"round":"abc"}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        error_events = [e for e in events if "event: error" in e]
        assert len(error_events) == 1
        data_line = [l for l in error_events[0].split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert data["code"] == "agent_unavailable"
        # But actually: max(0, "abc") would raise TypeError. Is this caught?
        # Let's check... this test verifies behavior

    @pytest.mark.asyncio
    async def test_error_event_clears_tool_starts(self):
        """SSE error event should clear pending tool_starts."""
        vr = _make_version_repo()
        stream = _make_stream([
            'data: {"type":"tool_call_start","data":{"name":"write","id":"t1"},"round":1}',
            'data: {"type":"error","data":{"message":"Something went wrong"},"round":1}',
            'data: {"type":"tool_result","data":{"id":"t1","content":"result","is_error":false,"duration_ms":10},"round":1}',
        ])
        events = [e async for e in bridge_sse(stream, "p1", vr)]
        tool_events = [e for e in events if "event: tool" in e]
        # start event + result event (orphan because tool_starts was cleared by error)
        assert len(tool_events) == 2
        result_event = [e for e in tool_events if '"phase": "result"' in e][0]
        data_line = [l for l in result_event.split("\n") if l.startswith("data: ")][0]
        data = json.loads(data_line[6:])
        assert data["orphan"] is True  # tool_starts was cleared, so ctx is None
