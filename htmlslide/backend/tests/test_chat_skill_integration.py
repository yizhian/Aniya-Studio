"""Integration tests for chat endpoint with design_skill (project-level)."""
from unittest.mock import AsyncMock, MagicMock, patch

import respx
from httpx import Response

from src.models.schemas import ChatRequest, DomContext


class TestChatRequestSchema:
    """Validate the ChatRequest schema fields."""

    def test_chat_request_required_fields(self):
        """ChatRequest requires project_id and prompt."""
        req = ChatRequest(project_id="proj-1", prompt="make a slide")
        assert req.project_id == "proj-1"
        assert req.prompt == "make a slide"

    def test_chat_request_accepts_selected_dom(self):
        """ChatRequest accepts optional selected_dom field."""
        dom = DomContext(css_path=".slide", tag="div", text="Hello")
        req = ChatRequest(project_id="proj-1", prompt="edit", selected_dom=dom)
        assert req.selected_dom is not None
        assert req.selected_dom.css_path == ".slide"

    def test_chat_request_selected_dom_none_by_default(self):
        """ChatRequest selected_dom defaults to None when not provided."""
        req = ChatRequest(project_id="proj-1", prompt="make a slide")
        assert req.selected_dom is None

    def test_chat_request_rejects_empty_project_id(self):
        """ChatRequest rejects empty project_id."""
        from pydantic import ValidationError
        import pytest
        with pytest.raises(ValidationError):
            ChatRequest(project_id="", prompt="make a slide")


class TestChatEndpoint:
    """Verify the chat endpoint behavior."""

    def test_chat_endpoint_passes_message_to_agent_client(self, api_client):
        """Chat endpoint passes formatted message to agent_client.chat_stream."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "ChatTest"})
        assert create_resp.status_code == 201
        pid = create_resp.json()["id"]

        mock_client = AsyncMock()
        mock_agentgo_stream = AsyncMock()
        mock_agentgo_stream.__aiter__ = AsyncMock(return_value=mock_agentgo_stream)
        mock_agentgo_stream.__anext__ = AsyncMock(side_effect=[
            b'data: {"type":"text","data":{"text":"Hello"},"round":1}',
            StopAsyncIteration,
        ])
        mock_client.chat_stream = MagicMock(return_value=mock_agentgo_stream)
        mock_client.get_session = AsyncMock(return_value=None)

        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            api_client.post(
                "/api/v1/chat",
                json={
                    "project_id": pid,
                    "prompt": "make a slide",
                },
            )

        call_kwargs = mock_client.chat_stream.call_args.kwargs
        assert call_kwargs["session_id"] == pid

    def test_chat_endpoint_no_skill_param(self, api_client):
        """When prompt is sent without DOM context, it's passed directly."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "PlainChat"})
        assert create_resp.status_code == 201
        pid = create_resp.json()["id"]

        mock_client = AsyncMock()
        mock_agentgo_stream = AsyncMock()
        mock_agentgo_stream.__aiter__ = AsyncMock(return_value=mock_agentgo_stream)
        mock_agentgo_stream.__anext__ = AsyncMock(side_effect=[
            b'data: {"type":"text","data":{"text":"Hello"},"round":1}',
            StopAsyncIteration,
        ])
        mock_client.chat_stream = MagicMock(return_value=mock_agentgo_stream)
        mock_client.get_session = AsyncMock(return_value=None)

        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            api_client.post(
                "/api/v1/chat",
                json={
                    "project_id": pid,
                    "prompt": "make a slide",
                },
            )

        call_kwargs = mock_client.chat_stream.call_args.kwargs
        assert "message" in call_kwargs

    def test_chat_endpoint_full_sse_flow(self, api_client):
        """Full SSE flow works end-to-end."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "FullChat"})
        assert create_resp.status_code == 201
        pid = create_resp.json()["id"]

        with respx.mock:
            sse_body = (
                'data: {"type":"thinking","data":{"text":"Designing..."},"round":1}\n'
                'data: {"type":"text","data":{"text":"<div class=\'slide\'>Output</div>"},"round":1}\n'
            )
            respx.post("http://agentgo:8080/chat").mock(
                return_value=Response(200, content=sse_body)
            )

            resp = api_client.post(
                "/api/v1/chat",
                json={
                    "project_id": pid,
                    "prompt": "design a title slide",
                },
            )

        assert resp.status_code == 200
        assert "text/event-stream" in resp.headers["content-type"]
        body = resp.text
        assert "event: thinking" in body
        assert "event: text" in body
        assert 'class=\'slide\'>Output' in body
        assert "event: done" in body

    def test_chat_endpoint_project_not_found(self, api_client):
        """404 when project doesn't exist."""
        resp = api_client.post(
            "/api/v1/chat",
            json={
                "project_id": "proj-nonexistent",
                "prompt": "hello",
            },
        )
        assert resp.status_code == 404

    def test_chat_endpoint_with_dom_context(self, api_client):
        """Chat endpoint includes DOM context in message when provided."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "DomChat"})
        assert create_resp.status_code == 201
        pid = create_resp.json()["id"]

        mock_client = AsyncMock()
        mock_agentgo_stream = AsyncMock()
        mock_agentgo_stream.__aiter__ = AsyncMock(return_value=mock_agentgo_stream)
        mock_agentgo_stream.__anext__ = AsyncMock(side_effect=[
            b'data: {"type":"text","data":{"text":"ok"},"round":1}',
            StopAsyncIteration,
        ])
        mock_client.chat_stream = MagicMock(return_value=mock_agentgo_stream)
        mock_client.get_session = AsyncMock(return_value=None)

        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            api_client.post(
                "/api/v1/chat",
                json={
                    "project_id": pid,
                    "prompt": "edit this",
                    "selected_dom": {
                        "css_path": ".slide.active",
                        "tag": "div",
                        "text": "Title",
                        "styles": {"color": "red"},
                    },
                },
            )

        call_kwargs = mock_client.chat_stream.call_args.kwargs
        assert "DOM" in call_kwargs["message"]

    def test_chat_endpoint_unicode_prompt(self, api_client):
        """Chat endpoint handles unicode/Chinese prompts."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "UnicodeChat"})
        assert create_resp.status_code == 201
        pid = create_resp.json()["id"]

        mock_client = AsyncMock()
        mock_agentgo_stream = AsyncMock()
        mock_agentgo_stream.__aiter__ = AsyncMock(return_value=mock_agentgo_stream)
        mock_agentgo_stream.__anext__ = AsyncMock(side_effect=[
            b'data: {"type":"text","data":{"text":"ok"},"round":1}',
            StopAsyncIteration,
        ])
        mock_client.chat_stream = MagicMock(return_value=mock_agentgo_stream)
        mock_client.get_session = AsyncMock(return_value=None)

        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            api_client.post(
                "/api/v1/chat",
                json={
                    "project_id": pid,
                    "prompt": "做一个暖色系的标题页",
                },
            )

        call_kwargs = mock_client.chat_stream.call_args.kwargs
        assert "做一个暖色系的标题页" in call_kwargs["message"]

    def test_chat_endpoint_long_prompt(self, api_client):
        """Chat endpoint handles prompts up to 4000 chars."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "LongPrompt"})
        assert create_resp.status_code == 201
        pid = create_resp.json()["id"]

        mock_client = AsyncMock()
        mock_agentgo_stream = AsyncMock()
        mock_agentgo_stream.__aiter__ = AsyncMock(return_value=mock_agentgo_stream)
        mock_agentgo_stream.__anext__ = AsyncMock(side_effect=[
            b'data: {"type":"text","data":{"text":"ok"},"round":1}',
            StopAsyncIteration,
        ])
        mock_client.chat_stream = MagicMock(return_value=mock_agentgo_stream)
        mock_client.get_session = AsyncMock(return_value=None)

        long_prompt = "test " * 1000  # 5000 chars > 4000 limit
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/chat",
                json={
                    "project_id": pid,
                    "prompt": long_prompt,
                },
            )

        # Should get a validation error
        assert resp.status_code == 422

    def test_chat_endpoint_empty_prompt(self, api_client):
        """Chat endpoint rejects empty prompt."""
        resp = api_client.post(
            "/api/v1/chat",
            json={
                "project_id": "proj-test",
                "prompt": "",
            },
        )
        assert resp.status_code == 422
