"""Additional edge case tests for schemas, deps, agent client, and full integration."""
import io
import json
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import respx
from httpx import Response

from src.models.schemas import (
    ChatRequest,
    CreateProjectRequest,
    DomContext,
    ExportResponse,
    FileUploadResponse,
)
from src.services.agent_client import AgentClient


class TestSchemaValidation:
    def test_create_project_name_default(self):
        """Default name should be used when not provided."""
        req = CreateProjectRequest()
        assert req.name == "未命名项目"

    def test_create_project_name_too_long(self):
        """Name exceeding 200 chars should fail validation."""
        with pytest.raises(Exception):
            CreateProjectRequest(name="A" * 201)

    def test_chat_request_prompt_too_long(self):
        """Prompt exceeding 4000 chars should fail validation."""
        with pytest.raises(Exception):
            ChatRequest(project_id="proj-1", prompt="A" * 4001)

    def test_chat_request_minimal(self):
        """Minimal valid chat request."""
        req = ChatRequest(project_id="p", prompt="hi")
        assert req.project_id == "p"
        assert req.prompt == "hi"
        assert req.selected_dom is None

    def test_dom_context_empty_styles(self):
        """DomContext with empty styles dict."""
        dom = DomContext(css_path=".a", tag="div", text="hello")
        assert dom.styles == {}

    def test_dom_context_text_max_length(self):
        """Text field max_length=200."""
        dom = DomContext(text="A" * 200)
        assert len(dom.text) == 200

    def test_file_upload_response_fields(self):
        """All FileUploadResponse fields should serialize correctly."""
        resp = FileUploadResponse(
            project_id="proj-abc",
            file_name="deck.html",
            file_size_bytes=1024,
            html="<div class='slide'>hi</div>",
            css="body{color:red}",
            slide_count=1,
            is_deck=True,
        )
        d = resp.model_dump()
        assert d["project_id"] == "proj-abc"
        assert d["is_deck"] is True
        assert d["slide_count"] == 1

    def test_export_response_roundtrip(self):
        """ExportResponse JSON roundtrip."""
        orig = ExportResponse(
            project_id="proj-1",
            version="v001",
            html="<div>hi</div>",
            css="div{color:red}",
            slide_count=3,
            file_size_bytes=500,
        )
        data = orig.model_dump()
        reloaded = ExportResponse(**data)
        assert reloaded == orig


class TestAgentClientBytesBranch:
    """Cover agent_client.py line 37 else branch: line already bytes."""

    @pytest.mark.asyncio
    async def test_chat_stream_lines_are_bytes(self):
        """When aiter_lines yields bytes instead of str, the else branch is taken."""
        client = AgentClient(base_url="http://agentgo:8080", timeout=30.0)

        mock_bytes_line = (
            b'data: {"type":"thinking","data":{"text":"think"},"round":1}'
        )

        async def mock_aiter_lines():
            yield mock_bytes_line

        mock_resp = MagicMock()
        mock_resp.aiter_lines = mock_aiter_lines
        mock_resp.raise_for_status = MagicMock()

        mock_ctx = MagicMock()
        mock_ctx.__aenter__ = AsyncMock(return_value=mock_resp)
        mock_ctx.__aexit__ = AsyncMock(return_value=None)

        with patch.object(client._client, "stream", return_value=mock_ctx):
            lines = []
            async for line in client.chat_stream("msg", "s1"):
                lines.append(line)

        assert len(lines) == 1
        assert lines[0] == mock_bytes_line  # unchanged, no encode


class TestFormatChatMessageEdges:
    def test_empty_styles_in_dom(self):
        """When styles dict is empty, styles_str should be empty string."""
        from src.utils.chat_utils import format_chat_message

        dom = DomContext(css_path=".x", tag="span", text="t", styles={})
        result = format_chat_message("fix it", dom)
        assert "当前样式: " in result
        # Empty styles produces empty string after the colon
        assert "当前样式: \n" in result

    def test_all_dom_fields_filled(self):
        """All DomContext fields present in formatted message."""
        from src.utils.chat_utils import format_chat_message

        dom = DomContext(
            css_path=".header > nav",
            tag="nav",
            text="Menu",
            styles={"display": "flex", "gap": "10px"},
        )
        result = format_chat_message("center it", dom)
        assert "CSS 路径: .header > nav" in result
        assert "标签: nav" in result
        assert '文字内容: "Menu"' in result
        assert "当前样式: display: flex, gap: 10px" in result
        assert result.endswith("center it")

    def test_dom_with_none_text(self):
        """DomContext with explicitly empty text."""
        from src.utils.chat_utils import format_chat_message

        dom = DomContext(css_path=".a", tag="div", text="")
        result = format_chat_message("change", dom)
        assert '文字内容: ""' in result


class TestDepsCacheBehavior:
    def test_cache_clear_and_recreate(self, temp_workspace):
        """After clearing caches and changing workspace, new instances use new path."""
        from src.config import settings
        from src.api import deps

        settings.workspace_path = temp_workspace

        deps._workspace_service = None
        deps._file_repo_cache.clear()
        deps._version_repo_cache.clear()
        deps._project_service_cache.clear()

        from src.api.deps import get_file_repo, get_workspace

        ws = get_workspace()
        fr = get_file_repo("test-project")

        assert ws._root == temp_workspace.resolve()
        assert fr._root == (temp_workspace / "projects" / "test-project").resolve()

    def test_get_workspace_returns_same_instance(self, temp_workspace):
        """get_workspace should return cached singleton."""
        from src.config import settings
        from src.api import deps
        from src.api.deps import get_workspace

        settings.workspace_path = temp_workspace
        deps._workspace_service = None

        a = get_workspace()
        b = get_workspace()
        assert a is b


class TestFilePathUploadEdgeCases:
    def test_upload_preserves_html_with_special_chars(self, api_client):
        """Upload HTML with Unicode/special characters should preserve them."""
        html = "<html><style>.a{}</style><div class='slide'>你好世界🌍</div></html>"
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html.encode("utf-8")), "text/html")},
            data={"project_name": "UnicodeTest"},
        )
        assert resp.status_code == 200
        body = resp.json()
        assert "你好世界🌍" in body["html"]

    def test_upload_htm_extension_accepted(self, api_client):
        """.htm files should be accepted."""
        html = b"<html><div class='slide'>content</div></html>"
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.htm", io.BytesIO(html), "text/html")},
            data={"project_name": "HTMTest"},
        )
        assert resp.status_code == 200

    def test_project_id_non_existent_in_upload(self, api_client):
        """Upload with non-existent project_id returns 404."""
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(b"<html></html>"), "text/html")},
            data={"project_id": "proj-fake123"},
        )
        assert resp.status_code == 404


class TestFullIntegrationWorkflow:
    """End-to-end workflow simulating the full user journey."""

    def test_full_workflow(self, api_client, temp_workspace):
        """Create project → upload → export → download → versions."""
        # Step 1: Create project
        c = api_client.post("/api/v1/projects", json={"name": "FullWorkflow"})
        assert c.status_code == 201
        pid = c.json()["id"]

        # Step 2: Verify empty project (no template HTML)
        g = api_client.get(f"/api/v1/projects/{pid}")
        assert g.status_code == 200
        assert g.json()["has_html"] is False
        assert g.json()["slide_count"] == 0

        # Step 3: Upload HTML deck
        html = (
            b"<html><style>.s1{color:red}.s2{color:blue}</style>"
            b"<div class='slide'>Slide One</div>"
            b"<div class='slide'>Slide Two</div></html>"
        )
        u = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )
        assert u.status_code == 200
        body = u.json()
        assert body["slide_count"] == 2
        assert body["is_deck"] is True
        assert "Slide One" in body["html"]
        assert ".s1{color:red}" in body["css"]

        # Step 4: Get project should now show HTML
        g2 = api_client.get(f"/api/v1/projects/{pid}")
        assert g2.status_code == 200
        info = g2.json()
        assert info["has_html"] is True
        assert info["slide_count"] == 2
        assert info["file_size_bytes"] > 0

        # Step 5: Export
        e = api_client.get(f"/api/v1/projects/{pid}/export")
        assert e.status_code == 200
        export_data = e.json()
        assert export_data["slide_count"] == 2
        assert ".s1{color:red}" in export_data["css"]
        assert "Slide Two" in export_data["html"]

        # Step 6: Download (raw HTML)
        d = api_client.get(f"/api/v1/projects/{pid}/download")
        assert d.status_code == 200
        assert b"<style>" in d.content
        assert b"Slide One" in d.content
        assert d.headers["content-type"].startswith("text/html")

        # Step 7: Version listing (empty since no AgentGo generated versions)
        v = api_client.get(f"/api/v1/projects/{pid}/versions")
        assert v.status_code == 200
        versions_data = v.json()
        assert versions_data["project_id"] == pid
