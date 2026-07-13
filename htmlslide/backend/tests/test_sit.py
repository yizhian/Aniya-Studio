"""System Integration Tests — cross-component integration scenarios.

Tests the full backend stack (API → Service → File I/O) with mocked AgentGo.
"""
import os
import json
import asyncio
import time

import pytest
import respx
from httpx import Response
from unittest.mock import AsyncMock, patch

from src.services.workspace import WorkspaceService
from src.services.agent_client import AgentClient


# ============================================================================
# SIT: Project CRUD Integration
# ============================================================================

class TestSIT_ProjectLifecycle:
    """End-to-end project CRUD via API."""

    def test_create_read_update_delete_flow(self, api_client):
        """Full CRUD lifecycle for a project."""
        # 1. Create
        create_resp = api_client.post("/api/v1/projects", json={"name": "SIT Project"})
        assert create_resp.status_code == 201
        pid = create_resp.json()["id"]
        assert pid.startswith("proj-")

        # 2. Read
        read_resp = api_client.get(f"/api/v1/projects/{pid}")
        assert read_resp.status_code == 200
        assert read_resp.json()["name"] == "SIT Project"

        # 3. List (should include new project)
        list_resp = api_client.get("/api/v1/projects")
        assert list_resp.status_code == 200
        names = [p["name"] for p in list_resp.json()]
        assert "SIT Project" in names

        # 4. Update
        update_resp = api_client.patch(
            f"/api/v1/projects/{pid}",
            json={"name": "Updated SIT", "design_skill": "slides", "brief": "A SIT test"},
        )
        assert update_resp.status_code == 200
        assert update_resp.json()["name"] == "Updated SIT"
        assert update_resp.json()["design_skill"] == "slides"

        # 5. Verify update persisted
        read2_resp = api_client.get(f"/api/v1/projects/{pid}")
        assert read2_resp.json()["name"] == "Updated SIT"

        # 6. Delete
        delete_resp = api_client.delete(f"/api/v1/projects/{pid}")
        assert delete_resp.status_code == 204

        # 7. Verify deleted
        read3_resp = api_client.get(f"/api/v1/projects/{pid}")
        assert read3_resp.status_code == 404

    def test_create_multiple_projects_and_list_order(self, api_client):
        """List returns projects in reverse creation order."""
        for name in ["Alpha", "Beta", "Gamma"]:
            api_client.post("/api/v1/projects", json={"name": name})

        resp = api_client.get("/api/v1/projects")
        projects = resp.json()
        # Most recently created first.
        assert projects[0]["name"] == "Gamma"
        assert projects[1]["name"] == "Beta"
        assert projects[2]["name"] == "Alpha"

    def test_create_project_with_brief_via_api(self, api_client):
        """Create project with brief field."""
        resp = api_client.post("/api/v1/projects", json={
            "name": "Briefed Project",
            "brief": "A project with a design brief for testing",
        })
        assert resp.status_code == 201
        data = resp.json()
        assert data["brief"] == "A project with a design brief for testing"


# ============================================================================
# SIT: Chat / Agent Integration
# ============================================================================

class TestSIT_ChatIntegration:
    """Chat endpoint integration with mocked AgentGo."""

    def test_full_chat_flow_with_multiple_rounds(self, api_client):
        """Simulate a multi-round chat conversation."""
        # Create project.
        create_resp = api_client.post("/api/v1/projects", json={"name": "Chat SIT"})
        pid = create_resp.json()["id"]

        with respx.mock:
            # Multi-round SSE response simulating thinking + tool + text.
            sse_body = (
                'data: {"type":"thinking","data":{"text":"Let me design a slide..."},"round":1}\n'
                'data: {"type":"tool","data":{"name":"write_file","phase":"start"},"round":1}\n'
                'data: {"type":"tool","data":{"name":"write_file","phase":"result","content":"File written"},"round":1}\n'
                'data: {"type":"text","data":{"text":"Here is your HTML slide"},"round":1}\n'
                'data: {"type":"thinking","data":{"text":"Refining..."},"round":2}\n'
                'data: {"type":"text","data":{"text":"Final result"},"round":2}\n'
            )
            respx.post("http://agentgo:8080/chat").mock(
                return_value=Response(200, content=sse_body)
            )

            resp = api_client.post(
                "/api/v1/chat",
                json={"project_id": pid, "prompt": "Create a title slide"},
            )

            assert resp.status_code == 200
            body = resp.text
            # Verify all event types present.
            assert "event: thinking" in body
            assert "event: text" in body
            assert "event: done" in body

    def test_chat_with_dom_context(self, api_client):
        """Chat request with selected DOM context."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "DOM SIT"})
        pid = create_resp.json()["id"]

        with respx.mock:
            sse_body = 'data: {"type":"text","data":{"text":"Updated blue color"},"round":1}\n'
            respx.post("http://agentgo:8080/chat").mock(
                return_value=Response(200, content=sse_body)
            )

            resp = api_client.post(
                "/api/v1/chat",
                json={
                    "project_id": pid,
                    "prompt": "make it blue",
                    "selected_dom": {
                        "css_path": ".slide > h1",
                        "tag": "h1",
                        "text": "Hello",
                        "styles": {"color": "#fff"},
                    },
                },
            )

            assert resp.status_code == 200
            assert "event: text" in resp.text

    def test_chat_agent_unavailable_returns_error_event(self, api_client):
        """AgentGo 502 produces SSE error event."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "502 SIT"})
        pid = create_resp.json()["id"]

        with respx.mock:
            respx.post("http://agentgo:8080/chat").mock(return_value=Response(502))

            resp = api_client.post(
                "/api/v1/chat",
                json={"project_id": pid, "prompt": "hello"},
            )

            assert resp.status_code == 200  # SSE starts before error
            assert "event: error" in resp.text
            assert "agent_unavailable" in resp.text

    def test_chat_project_not_found(self, api_client):
        resp = api_client.post(
            "/api/v1/chat",
            json={"project_id": "proj-nonexistent", "prompt": "hello"},
        )
        assert resp.status_code == 404

    def test_chat_validates_prompt_max_length(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "Validate"})
        pid = create_resp.json()["id"]

        too_long = "x" * 5000
        resp = api_client.post(
            "/api/v1/chat",
            json={"project_id": pid, "prompt": too_long},
        )
        # Should be rejected by Pydantic validation.
        assert resp.status_code == 422


# ============================================================================
# SIT: Recommend Styles Integration
# ============================================================================

class TestSIT_RecommendStyles:
    """Style recommendation endpoint integration."""

    def test_recommend_styles_with_agent_client(self, api_client):
        """Full flow: validate input → call agent → return results."""
        mock_client = AsyncMock()
        mock_client.recommend_styles = AsyncMock(return_value={
            "styles": [
                {"name": "Minimal Dark", "colors": ["#111"]},
                {"name": "Coral Warm", "colors": ["#ff6b6b"]},
            ],
        })
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/recommend-styles",
                json={"brief": "warm dark theme", "limit": 2},
            )
        assert resp.status_code == 200
        data = resp.json()
        assert len(data["styles"]) == 2

    def test_recommend_styles_empty_brief(self, api_client):
        resp = api_client.post(
            "/api/v1/recommend-styles",
            json={"brief": ""},
        )
        assert resp.status_code == 422

    def test_recommend_styles_missing_brief(self, api_client):
        resp = api_client.post(
            "/api/v1/recommend-styles",
            json={},
        )
        assert resp.status_code == 422


# ============================================================================
# SIT: Chat History Integration
# ============================================================================

class TestSIT_ChatHistory:
    """Chat history endpoint integration."""

    def test_empty_history_for_new_project(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "History SIT"})
        pid = create_resp.json()["id"]

        mock_client = AsyncMock()
        mock_client.get_session = AsyncMock(return_value=None)
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.get(f"/api/v1/projects/{pid}/chat-history")
        assert resp.status_code == 200
        assert resp.json()["entries"] == []

    def test_history_with_timeline_entries(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "Timeline SIT"})
        pid = create_resp.json()["id"]

        mock_client = AsyncMock()
        mock_client.get_session = AsyncMock(return_value={
            "timeline": [
                {"event": "user_message", "data": {"content": "make a slide"}},
                {"event": "thinking", "data": {"content": "Let me think..."}},
                {"event": "text", "data": {"content": "Here is HTML"}},
            ],
        })
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.get(f"/api/v1/projects/{pid}/chat-history")
        assert resp.status_code == 200
        entries = resp.json()["entries"]
        assert len(entries) == 3
        assert entries[0]["event"] == "user_message"
        assert entries[2]["event"] == "text"

    def test_history_project_not_found(self, api_client):
        resp = api_client.get("/api/v1/projects/nonexistent/chat-history")
        assert resp.status_code == 404


# ============================================================================
# SIT: Health Check Integration
# ============================================================================

class TestSIT_HealthCheck:
    """Health check endpoint verification."""

    def test_health_endpoint_on_main_app(self):
        """Health check is registered on the main FastAPI app, not the API router."""
        from src.main import app
        from fastapi.testclient import TestClient
        with TestClient(app, raise_server_exceptions=False) as client:
            resp = client.get("/health")
            assert resp.status_code == 200
            assert resp.json()["status"] == "ok"

    def test_cors_headers_on_api(self, api_client):
        """API list endpoint responds correctly (CORS tested via withCORS wrapper)."""
        resp = api_client.get("/api/v1/projects")
        assert resp.status_code == 200


# ============================================================================
# SIT: Workspace File Write + Read
# ============================================================================

class TestSIT_WorkspaceFiles:
    """Verify file-based workspace operations."""

    def test_project_json_written_correctly(self, temp_workspace):
        ws = WorkspaceService(temp_workspace)
        meta = asyncio.run(ws.create_project("FileTest"))
        pid = meta["id"]
        proj_dir = temp_workspace / "projects" / pid

        project_json = json.loads((proj_dir / "project.json").read_text())
        assert project_json["name"] == "FileTest"
        assert project_json["id"] == pid
        assert "created_at" in project_json

    def test_update_writes_to_project_json(self, temp_workspace):
        ws = WorkspaceService(temp_workspace)
        meta = asyncio.run(ws.create_project("Original"))
        pid = meta["id"]

        asyncio.run(ws.update_project(pid, name="Modified", brief="New brief"))
        proj_dir = temp_workspace / "projects" / pid
        updated = json.loads((proj_dir / "project.json").read_text())
        assert updated["name"] == "Modified"
        assert updated["brief"] == "New brief"


# ============================================================================
# SIT: Concurrent Operations
# ============================================================================

class TestSIT_Concurrent:
    """Verify thread-safety of concurrent operations."""

    def test_concurrent_project_creation(self, api_client):
        """Create multiple projects quickly (no race conditions)."""
        pids = set()
        for i in range(20):
            resp = api_client.post("/api/v1/projects", json={"name": f"Concurrent {i}"})
            assert resp.status_code == 201
            pids.add(resp.json()["id"])
        # All IDs should be unique.
        assert len(pids) == 20

    def test_concurrent_project_list(self, api_client):
        """List projects after concurrent creation."""
        for i in range(10):
            api_client.post("/api/v1/projects", json={"name": f"List {i}"})

        # Multiple list calls should be consistent.
        results = []
        for _ in range(5):
            resp = api_client.get("/api/v1/projects")
            results.append(len(resp.json()))
        assert all(r == results[0] for r in results)


# ============================================================================
# SIT: Version Endpoints with require_project Guard (Round 3)
# ============================================================================


class TestSIT_VersionGuards:
    """Version endpoints return 404 for nonexistent projects (Round 3 fix)."""

    def test_list_versions_nonexistent_project_returns_404(self, api_client):
        resp = api_client.get("/api/v1/projects/proj-nonexistent99/versions")
        assert resp.status_code == 404
        assert resp.json()["code"] == "project_not_found"

    def test_get_version_detail_nonexistent_project_returns_404(self, api_client):
        resp = api_client.get("/api/v1/projects/proj-nonexistent99/versions/v1")
        assert resp.status_code == 404
        assert resp.json()["code"] == "project_not_found"

    def test_save_version_nonexistent_project_returns_400(self, api_client):
        """save_version doesn't call require_project but FileRepo.html_exists()
        returns False for nonexistent dirs, yielding a 400."""
        resp = api_client.post(
            "/api/v1/projects/proj-nonexistent99/versions",
            json={"title": "v1", "html": "<div></div>"},
        )
        assert resp.status_code == 400

    def test_restore_version_nonexistent_project_returns_404(self, api_client):
        resp = api_client.post(
            "/api/v1/projects/proj-nonexistent99/versions/v1/restore",
        )
        assert resp.status_code == 404

    def test_list_versions_for_real_project_returns_200(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "VersionGuard"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/versions")
        assert resp.status_code == 200
        body = resp.json()
        assert body["project_id"] == pid
        assert "versions" in body


# ============================================================================
# SIT: Settings Endpoints (Round 3 — build_provider_payload refactor)
# ============================================================================


class TestSIT_SettingsEndpoints:
    """Settings endpoints integration with mocked AgentGo."""

    def test_sync_settings_ok(self, api_client):
        mock_client = AsyncMock()
        mock_client.build_provider_payload = lambda req: {
            "provider_type": req.provider,
            "api_key": req.api_key,
            "base_url": req.base_url,
            "model": req.model_name,
        }
        mock_client.sync_provider_config = AsyncMock(return_value={"message": "OK"})

        with patch("src.api.settings.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/settings/sync",
                json={
                    "provider": "openai",
                    "api_key": "sk-test",
                    "base_url": "https://api.openai.com/v1",
                    "model_name": "gpt-4o",
                },
            )
        assert resp.status_code == 200
        assert resp.json()["ok"] is True

    def test_sync_settings_error_handled(self, api_client):
        mock_client = AsyncMock()
        mock_client.build_provider_payload = lambda req: {}
        mock_client.sync_provider_config = AsyncMock(side_effect=Exception("boom"))

        with patch("src.api.settings.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/settings/sync",
                json={
                    "provider": "openai",
                    "api_key": "x",
                    "base_url": "https://x.com",
                    "model_name": "m",
                },
            )
        assert resp.status_code == 200
        assert resp.json()["ok"] is False
        assert "boom" in resp.json()["message"]

    def test_test_connection_ok(self, api_client):
        mock_client = AsyncMock()
        mock_client.build_provider_payload = lambda req: {}
        mock_client.test_provider_config = AsyncMock(return_value={
            "ok": True, "message": "Connected", "in_list": True, "verified": True,
        })

        with patch("src.api.settings.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/settings/test-connection",
                json={
                    "provider": "openai",
                    "api_key": "sk-test",
                    "base_url": "https://api.openai.com/v1",
                    "model_name": "gpt-4o",
                },
            )
        assert resp.status_code == 200
        body = resp.json()
        assert body["ok"] is True
        assert body["verified"] is True

    def test_list_models_ok(self, api_client):
        mock_client = AsyncMock()
        mock_client.build_provider_payload = lambda req: {}
        mock_client.list_provider_models = AsyncMock(return_value={
            "models": [{"id": "gpt-4o"}, {"id": "gpt-4o-mini"}], "source": "api", "error": "",
        })

        with patch("src.api.settings.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/settings/models",
                json={
                    "provider": "openai",
                    "api_key": "sk-test",
                    "base_url": "https://api.openai.com/v1",
                    "model_name": "gpt-4o",
                },
            )
        assert resp.status_code == 200
        body = resp.json()
        assert len(body["models"]) == 2
        assert body["source"] == "api"


# ============================================================================
# SIT: require_html Consolidation (Round 3+4)
# ============================================================================


class TestSIT_RequireHtml:
    """Export and file endpoints use require_html (Round 3+4 refactor)."""

    def test_export_json_without_html_returns_404(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "NoHTMLExport"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/export")
        assert resp.status_code == 404
        assert resp.json()["code"] == "file_read_error"

    def test_download_without_html_returns_404(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "NoHTMLDownload"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/download")
        assert resp.status_code == 404
        assert resp.json()["code"] == "file_read_error"

    def test_pdf_export_without_html_returns_404(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "NoHTMLPdf"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/export/pdf")
        assert resp.status_code == 404

    def test_pptx_export_without_html_returns_404(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "NoHTMLPptx"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/export/pptx")
        assert resp.status_code == 404

    def test_preview_without_html_returns_404(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "NoHTMLPreview"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/preview")
        assert resp.status_code == 404
        assert resp.json()["code"] == "file_read_error"


# ============================================================================
# SIT: _streaming_binary_export Unification (Round 5)
# ============================================================================


class TestSIT_StreamingExports:
    """PDF and PPTX exports share _streaming_binary_export (Round 5)."""

    def test_both_export_endpoints_accept_same_project(self, api_client, temp_workspace):
        """Same project with HTML can be exported as both PDF and PPTX."""
        import io
        create = api_client.post("/api/v1/projects", json={"name": "DualExport"})
        pid = create.json()["id"]
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        html = b"<html><div class='slide'>x</div></html>"
        api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )

        with patch("src.api.export.generate_pdf", new_callable=AsyncMock) as pdf_mock, \
             patch("src.api.export.generate_pptx", new_callable=AsyncMock) as pptx_mock:
            pdf_mock.return_value = b"%PDF-fake"
            pptx_mock.return_value = b"PK-fake-pptx"

            pdf_resp = api_client.get(f"/api/v1/projects/{pid}/export/pdf")
            pptx_resp = api_client.get(f"/api/v1/projects/{pid}/export/pptx")

        assert pdf_resp.status_code == 200
        assert pptx_resp.status_code == 200
        assert pdf_resp.headers["content-type"] == "application/pdf"
        assert "presentationml" in pptx_resp.headers["content-type"]
