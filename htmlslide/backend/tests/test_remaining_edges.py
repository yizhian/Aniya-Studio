"""Tests for WorkspaceService, AgentClient, and other remaining edge cases."""
import io
import json
import os
from pathlib import Path

import pytest
import respx
from httpx import Response


class TestWorkspaceCorruptedMeta:
    """Cover workspace.py edge cases: corrupted/malformed project.json."""

    @pytest.mark.asyncio
    async def test_read_meta_corrupted_json(self, temp_workspace):
        """Corrupted project.json should raise json.JSONDecodeError."""
        from src.services.workspace import WorkspaceService

        ws = WorkspaceService(temp_workspace)
        meta = await ws.create_project("CorruptTest")
        proj_dir = ws._project_dir(meta["id"])
        (proj_dir / "project.json").write_text("not valid {{{ json")

        with pytest.raises(json.JSONDecodeError):
            await ws.read_meta(meta["id"])

    @pytest.mark.asyncio
    async def test_read_meta_empty_file(self, temp_workspace):
        """Empty project.json should raise json.JSONDecodeError."""
        from src.services.workspace import WorkspaceService

        ws = WorkspaceService(temp_workspace)
        meta = await ws.create_project("EmptyTest")
        proj_dir = ws._project_dir(meta["id"])
        (proj_dir / "project.json").write_text("")

        with pytest.raises(json.JSONDecodeError):
            await ws.read_meta(meta["id"])

    @pytest.mark.asyncio
    async def test_create_project_creates_root(self, temp_workspace):
        """create_project should create root dir if it doesn't exist."""
        from src.services.workspace import WorkspaceService

        nested = temp_workspace / "nested" / "path"
        ws = WorkspaceService(nested)
        meta = await ws.create_project("Deep")
        assert nested.is_dir()
        assert meta["name"] == "Deep"


class TestVersionRestoreEdge:
    """Cover version.py edge: restore when version dir has no index.html."""

    def test_restore_version_missing_html(self, api_client, temp_workspace):
        """Restore version whose index.html was deleted — FileNotFoundError caught → 404."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "RestoreEdge"})
        pid = create_resp.json()["id"]

        # Create version dir WITHOUT index.html under the project directory
        v_dir = temp_workspace / "projects" / pid / ".slidecraft" / "versions" / "v001"
        v_dir.mkdir(parents=True)
        (v_dir / "context.json").write_text(json.dumps({"title": "Empty"}))

        resp = api_client.post(f"/api/v1/projects/{pid}/versions/v001/restore")
        # get_version tries _read_file(index.html) → FileNotFoundError → caught → 404
        assert resp.status_code == 404


class TestExportEdge:
    """Cover export.py: version is None path."""

    def test_export_when_no_versions(self, api_client):
        """Export with uploaded HTML but no versions — version should be 'unknown'."""
        # Create project first
        create_resp = api_client.post("/api/v1/projects", json={"name": "NoVersion"})
        pid = create_resp.json()["id"]

        # Upload HTML
        html = b"<html><style>.c{}</style><div class='slide'>s1</div></html>"
        upload_resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )
        assert upload_resp.status_code == 200

        resp = api_client.get(f"/api/v1/projects/{pid}/export")
        assert resp.status_code == 200
        assert resp.json()["version"] == "unknown"

    def test_download_special_chars_name(self, api_client):
        """Download with project name containing special characters."""
        html = b"<html><div class='slide'>x</div></html>"
        upload_resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_name": "My Project: Final!"},
        )
        pid = upload_resp.json()["project_id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/download")
        assert resp.status_code == 200
        assert b"x" in resp.content


class TestAgentClientEdges:
    """Cover agent_client.py: connection errors, non-200 status."""

    @pytest.mark.asyncio
    async def test_chat_stream_404_status(self):
        """chat_stream when AgentGo returns 404."""
        from src.services.agent_client import AgentClient

        client = AgentClient(base_url="http://agentgo:8080", timeout=5.0)
        with respx.mock:
            respx.post("http://agentgo:8080/chat").mock(return_value=Response(404))
            with pytest.raises(Exception):
                async for _ in client.chat_stream("msg", "s1"):
                    pass
        await client.close()

    @pytest.mark.asyncio
    async def test_chat_stream_500_status(self):
        """chat_stream when AgentGo returns 500."""
        from src.services.agent_client import AgentClient

        client = AgentClient(base_url="http://agentgo:8080", timeout=5.0)
        with respx.mock:
            respx.post("http://agentgo:8080/chat").mock(return_value=Response(500))
            with pytest.raises(Exception):
                async for _ in client.chat_stream("msg", "s1"):
                    pass
        await client.close()

    @pytest.mark.asyncio
    async def test_chat_stream_empty_body(self):
        """chat_stream with empty response body."""
        from src.services.agent_client import AgentClient

        client = AgentClient(base_url="http://agentgo:8080", timeout=5.0)
        with respx.mock:
            respx.post("http://agentgo:8080/chat").mock(
                return_value=Response(200, content=b"")
            )
            lines = []
            async for line in client.chat_stream("msg", "s1"):
                lines.append(line)
            assert len(lines) == 0
        await client.close()


class TestConfigEdge:
    """Cover config.py: timeout defaults, env loading."""

    def test_agent_timeout_default(self):
        from src.config import settings
        assert settings.agent_timeout == 1000.0

    def test_sse_heartbeat_default(self):
        from src.config import settings
        assert settings.sse_heartbeat_interval == 90.0

    def test_log_level_default(self):
        from src.config import settings
        assert settings.log_level == "INFO"

    def test_cors_origins_with_extra_whitespace(self):
        """cors_origin_list should handle extra whitespace."""
        from src.config import settings
        original = settings.cors_origins
        try:
            settings.cors_origins = " http://a.com ,  http://b.com "
            assert settings.cors_origin_list == ["http://a.com", "http://b.com"]
        finally:
            settings.cors_origins = original


class TestFileUploadUppercase:
    """Cover file.py: .HTM/.HTML uppercase extension acceptance."""

    def test_upload_uppercase_htm(self, api_client):
        """.HTM (uppercase) should be accepted."""
        html = b"<html><div class='slide'>content</div></html>"
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.HTM", io.BytesIO(html), "text/html")},
            data={"project_name": "UpperHTM"},
        )
        assert resp.status_code == 200

    def test_upload_uppercase_html(self, api_client):
        """.HTML (uppercase) should be accepted."""
        html = b"<html><div class='slide'>content</div></html>"
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.HTML", io.BytesIO(html), "text/html")},
            data={"project_name": "UpperHTML"},
        )
        assert resp.status_code == 200


class TestNonUtf8Upload:
    """Cover file.py: non-UTF-8 file upload crash path."""

    def test_non_utf8_upload(self, api_client):
        """Upload Latin-1 encoded HTML — UnicodeDecodeError is ValueError subclass → 422."""
        content = b"<html><style>.s{}</style><div class='slide'>\xe9\xe8</div></html>"
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(content), "text/html")},
            data={"project_name": "Latin1Test"},
        )
        # UnicodeDecodeError subclasses ValueError → caught by value_error_handler → 422
        assert resp.status_code == 422
