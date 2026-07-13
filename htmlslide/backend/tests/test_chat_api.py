from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import respx
from httpx import Response

from src.utils.chat_utils import format_chat_message, messages_to_timeline
from src.models.schemas import DomContext


class TestFormatChatMessage:
    def test_no_dom_returns_prompt_unchanged(self):
        result = format_chat_message("make it red", None)
        assert result == "make it red"

    def test_with_dom_prefixes_text(self):
        dom = DomContext(
            css_path=".slide > h1",
            tag="h1",
            text="Hello World",
            styles={"color": "#fff", "font-size": "48px"},
        )
        result = format_chat_message("make it blue", dom)
        assert "【DOM 元素选中】" in result
        assert ".slide > h1" in result
        assert "h1" in result
        assert "Hello World" in result
        assert "color: #fff" in result
        assert "font-size: 48px" in result
        assert "用户指令：" in result
        assert result.endswith("make it blue")


class TestChatSSE:
    def test_chat_with_selected_dom(self, api_client):
        """Test chat endpoint: create project, mock AgentGo, verify SSE response."""
        # Create project first
        create_resp = api_client.post("/api/v1/projects", json={"name": "ChatTest"})
        pid = create_resp.json()["id"]

        with respx.mock:
            # Mock AgentGo /chat SSE response
            sse_body = (
                'data: {"type":"thinking","data":{"text":"Thinking..."},"round":1}\n'
                'data: {"type":"text","data":{"text":"Here is HTML"},"round":1}\n'
            )
            respx.post("http://agentgo:8080/chat").mock(
                return_value=Response(200, content=sse_body)
            )

            resp = api_client.post(
                "/api/v1/chat",
                json={
                    "project_id": pid,
                    "prompt": "make a slide",
                    "selected_dom": None,
                },
            )

            assert resp.status_code == 200
            assert "text/event-stream" in resp.headers["content-type"]
            body = resp.text
            assert "event: thinking" in body
            assert "event: text" in body
            assert "Here is HTML" in body
            # Must have done event
            assert "event: done" in body

    def test_chat_project_not_found(self, api_client):
        resp = api_client.post(
            "/api/v1/chat",
            json={
                "project_id": "proj-nonexistent",
                "prompt": "hello",
            },
        )
        assert resp.status_code == 404

    def test_chat_agent_unavailable(self, api_client):
        """When AgentGo returns 502, the outer try/except captures it."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "AgentDown"})
        pid = create_resp.json()["id"]

        with respx.mock:
            respx.post("http://agentgo:8080/chat").mock(return_value=Response(502))

            resp = api_client.post(
                "/api/v1/chat",
                json={"project_id": pid, "prompt": "hello"},
            )

            # StreamingResponse starts with 200 even if stream errors
            # The error is delivered as SSE event
            assert resp.status_code == 200
            body = resp.text
            assert "event: error" in body
            assert "agent_unavailable" in body


class TestMessagesToTimeline:
    """Test the messages_to_timeline backward-compat fallback."""

    def test_empty_messages(self):
        assert messages_to_timeline({"messages": []}) == []

    def test_no_messages_key(self):
        assert messages_to_timeline({}) == []

    def test_system_message_skipped(self):
        result = messages_to_timeline({
            "messages": [{"role": "system", "content": "You are an assistant."}],
        })
        assert result == []

    def test_user_message(self):
        result = messages_to_timeline({
            "messages": [{"role": "user", "content": "make a slide"}],
        })
        assert len(result) == 1
        assert result[0]["event"] == "user_message"
        assert result[0]["data"]["content"] == "make a slide"

    def test_assistant_with_reasoning_and_content(self):
        result = messages_to_timeline({
            "messages": [{
                "role": "assistant",
                "reasoning_content": "I think...",
                "content": "Here is HTML",
            }],
        })
        assert len(result) == 2
        assert result[0]["event"] == "thinking"
        assert result[0]["data"]["content"] == "I think..."
        assert result[1]["event"] == "text"
        assert result[1]["data"]["content"] == "Here is HTML"

    def test_assistant_content_only(self):
        result = messages_to_timeline({
            "messages": [{"role": "assistant", "content": "HTML only"}],
        })
        assert len(result) == 1
        assert result[0]["event"] == "text"

    def test_tool_message_success(self):
        result = messages_to_timeline({
            "messages": [
                {"role": "assistant", "tool_calls": [
                    {"id": "call_1", "function": {"name": "write_file"}},
                ]},
                {"role": "tool", "tool_call_id": "call_1",
                 "content": "File written successfully"},
            ],
        })
        tool_entries = [e for e in result if e["event"] == "tool"]
        assert len(tool_entries) == 1
        assert tool_entries[0]["data"]["name"] == "write_file"
        assert tool_entries[0]["data"]["success"] is True

    def test_tool_message_error(self):
        result = messages_to_timeline({
            "messages": [
                {"role": "assistant", "tool_calls": [
                    {"id": "call_1", "function": {"name": "read_file"}},
                ]},
                {"role": "tool", "tool_call_id": "call_1",
                 "content": "error: file not found"},
            ],
        })
        tool_entries = [e for e in result if e["event"] == "tool"]
        assert tool_entries[0]["data"]["success"] is False

    def test_tool_long_content_truncated(self):
        long_content = "x" * 500
        result = messages_to_timeline({
            "messages": [
                {"role": "assistant", "tool_calls": [
                    {"id": "call_1", "function": {"name": "read_file"}},
                ]},
                {"role": "tool", "tool_call_id": "call_1", "content": long_content},
            ],
        })
        tool_entries = [e for e in result if e["event"] == "tool"]
        assert len(tool_entries[0]["data"]["summary"]) == 200

    def test_unknown_tool_name(self):
        result = messages_to_timeline({
            "messages": [
                {"role": "tool", "tool_call_id": "call_999", "content": "done"},
            ],
        })
        tool_entries = [e for e in result if e["event"] == "tool"]
        assert tool_entries[0]["data"]["name"] == "unknown"

    def test_no_tool_name_in_map(self):
        result = messages_to_timeline({
            "messages": [
                {"role": "assistant", "tool_calls": []},
                {"role": "tool", "tool_call_id": "call_x", "content": "result"},
            ],
        })
        tool_entries = [e for e in result if e["event"] == "tool"]
        assert tool_entries[0]["data"]["name"] == "unknown"


class TestRecommendStylesEndpoint:
    """Test the /recommend-styles endpoint."""

    def test_recommend_with_limit(self, api_client):
        mock_client = AsyncMock()
        mock_client.recommend_styles = AsyncMock(return_value={
            "styles": [{"name": "Dark"}],
        })
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/recommend-styles",
                json={"brief": "dark theme", "limit": 2},
            )
        assert resp.status_code == 200
        mock_client.recommend_styles.assert_awaited_once_with("dark theme", 2)

    def test_recommend_default_limit(self, api_client):
        mock_client = AsyncMock()
        mock_client.recommend_styles = AsyncMock(return_value={"styles": []})
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/recommend-styles",
                json={"brief": "test"},
            )
        assert resp.status_code == 200
        mock_client.recommend_styles.assert_awaited_once_with("test", 3)

    def test_recommend_empty_brief_rejected(self, api_client):
        resp = api_client.post(
            "/api/v1/recommend-styles",
            json={"brief": ""},
        )
        assert resp.status_code == 422


class TestChatHistoryEndpoint:
    """Test the GET /projects/{project_id}/chat-history endpoint."""

    def test_project_not_found(self, api_client):
        resp = api_client.get("/api/v1/projects/nonexistent/chat-history")
        assert resp.status_code == 404

    def test_empty_session(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "Hist"})
        pid = create_resp.json()["id"]

        mock_client = AsyncMock()
        mock_client.get_session = AsyncMock(return_value=None)
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.get(f"/api/v1/projects/{pid}/chat-history")
        assert resp.status_code == 200
        data = resp.json()
        assert data["entries"] == []

    def test_with_timeline(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "Timeline"})
        pid = create_resp.json()["id"]

        mock_client = AsyncMock()
        mock_client.get_session = AsyncMock(return_value={
            "timeline": [
                {"event": "user_message", "data": {"content": "hello"}},
                {"event": "text", "data": {"content": "Hi!"}},
            ],
        })
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.get(f"/api/v1/projects/{pid}/chat-history")
        assert resp.status_code == 200
        data = resp.json()
        assert len(data["entries"]) == 2

    def test_fallback_to_messages(self, api_client):
        """When timeline is empty, fall back to old-format messages."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "Fallback"})
        pid = create_resp.json()["id"]

        mock_client = AsyncMock()
        mock_client.get_session = AsyncMock(return_value={
            "timeline": [],
            "messages": [{"role": "user", "content": "make a slide"}],
        })
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.get(f"/api/v1/projects/{pid}/chat-history")
        assert resp.status_code == 200
        data = resp.json()
        assert len(data["entries"]) == 1
        assert data["entries"][0]["event"] == "user_message"
