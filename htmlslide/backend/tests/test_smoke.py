"""Smoke Tests — critical path verification.

These tests verify that all major API endpoints respond correctly. Designed to
run quickly and catch regressions in critical user flows.
"""
import io
import json

import pytest
import respx
from httpx import Response
from unittest.mock import AsyncMock, patch

FAKE_PPTX = b"PK\x03\x04 fake pptx content"


class TestSmoke_HealthCheck:
    """Verify the service is alive (health check on main app)."""

    def test_health_returns_200(self):
        from src.main import app
        from fastapi.testclient import TestClient
        with TestClient(app, raise_server_exceptions=False) as client:
            resp = client.get("/health")
            assert resp.status_code == 200
            body = resp.json()
            assert body["status"] == "ok"

    def test_health_response_is_json(self):
        from src.main import app
        from fastapi.testclient import TestClient
        with TestClient(app, raise_server_exceptions=False) as client:
            resp = client.get("/health")
            assert resp.headers["content-type"].startswith("application/json")


class TestSmoke_ProjectCRUD:
    """Verify project CRUD is operational."""

    def test_create_project_minimal(self, api_client):
        resp = api_client.post("/api/v1/projects", json={"name": "SmokeTest"})
        assert resp.status_code == 201
        data = resp.json()
        assert data["name"] == "SmokeTest"
        assert data["id"].startswith("proj-")

    def test_create_project_with_brief(self, api_client):
        resp = api_client.post("/api/v1/projects", json={
            "name": "Brief Smoke",
            "brief": "Smoke test brief",
        })
        assert resp.status_code == 201
        assert resp.json()["brief"] == "Smoke test brief"

    def test_list_projects(self, api_client):
        resp = api_client.get("/api/v1/projects")
        assert resp.status_code == 200
        assert isinstance(resp.json(), list)

    def test_get_project_by_id(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "GetMe"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}")
        assert resp.status_code == 200
        assert resp.json()["name"] == "GetMe"

    def test_get_nonexistent_project_returns_404(self, api_client):
        resp = api_client.get("/api/v1/projects/proj-000000000000")
        assert resp.status_code == 404

    def test_delete_project(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "DeleteMe"})
        pid = create.json()["id"]
        resp = api_client.delete(f"/api/v1/projects/{pid}")
        assert resp.status_code == 204

    def test_delete_nonexistent_project_returns_404(self, api_client):
        resp = api_client.delete("/api/v1/projects/proj-000000000000")
        assert resp.status_code == 404

    def test_update_project_name(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "Old"})
        pid = create.json()["id"]
        resp = api_client.patch(f"/api/v1/projects/{pid}", json={"name": "New"})
        assert resp.status_code == 200
        assert resp.json()["name"] == "New"


class TestSmoke_Chat:
    """Verify chat endpoint is operational."""

    def test_chat_returns_sse_stream(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "ChatSmoke"})
        pid = create.json()["id"]

        with respx.mock:
            respx.post("http://agentgo:8080/chat").mock(
                return_value=Response(
                    200,
                    content='data: {"type":"text","data":{"text":"Hello"},"round":1}\n',
                )
            )
            resp = api_client.post(
                "/api/v1/chat",
                json={"project_id": pid, "prompt": "test"},
            )
            assert resp.status_code == 200
            assert "text/event-stream" in resp.headers["content-type"]
            assert "event: text" in resp.text
            assert "event: done" in resp.text

    def test_chat_rejects_missing_project_id(self, api_client):
        resp = api_client.post("/api/v1/chat", json={"prompt": "hello"})
        assert resp.status_code == 422

    def test_chat_rejects_missing_prompt(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "MissingPrompt"})
        pid = create.json()["id"]
        resp = api_client.post("/api/v1/chat", json={"project_id": pid})
        assert resp.status_code == 422


class TestSmoke_ChatHistory:
    """Verify chat history endpoint."""

    def test_history_returns_200(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "HistSmoke"})
        pid = create.json()["id"]

        mock_client = AsyncMock()
        mock_client.get_session = AsyncMock(return_value=None)
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.get(f"/api/v1/projects/{pid}/chat-history")
        assert resp.status_code == 200
        body = resp.json()
        assert "entries" in body


class TestSmoke_RecommendStyles:
    """Verify style recommendation endpoint."""

    def test_recommend_returns_200(self, api_client):
        mock_client = AsyncMock()
        mock_client.recommend_styles = AsyncMock(return_value={"styles": []})
        with patch("src.api.chat.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/recommend-styles",
                json={"brief": "minimal design"},
            )
        assert resp.status_code == 200
        assert "styles" in resp.json()


class TestSmoke_EndpointsRespond:
    """Verify endpoints respond appropriately."""

    def test_get_on_post_only_returns_405(self, api_client):
        """GET on POST-only endpoints should return 405."""
        resp = api_client.get("/api/v1/chat")
        assert resp.status_code == 405


class TestSmoke_SchemaValidation:
    """Verify request validation catches malformed input."""

    def test_create_project_with_default_name(self, api_client):
        """Empty request body creates project with default name (not rejected)."""
        resp = api_client.post("/api/v1/projects", json={})
        assert resp.status_code == 201
        assert "name" in resp.json()

    def test_chat_rejects_invalid_json(self, api_client):
        resp = api_client.post("/api/v1/chat", data="bad json")
        assert resp.status_code == 422


class TestSmoke_PptxExport:
    """Verify PPTX export endpoint is wired correctly."""

    def test_pptx_export_endpoint_returns_correct_content_type(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "PptxSmoke"})
        pid = create_resp.json()["id"]
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        html = b"<html><div class='slide'>x</div></html>"
        api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )

        with patch("src.api.export.generate_pptx", new_callable=AsyncMock) as mock_gen:
            mock_gen.return_value = FAKE_PPTX
            resp = api_client.get(f"/api/v1/projects/{pid}/export/pptx")

        assert resp.status_code == 200
        assert "presentationml" in resp.headers["content-type"]

    def test_pptx_export_requires_html_content(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "NoContent"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/export/pptx")
        assert resp.status_code == 404

    def test_pptx_export_returns_streaming_response(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "StreamSmoke"})
        pid = create_resp.json()["id"]
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        html = b"<html><div class='slide'>x</div></html>"
        api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )

        with patch("src.api.export.generate_pptx", new_callable=AsyncMock) as mock_gen:
            mock_gen.return_value = FAKE_PPTX
            resp = api_client.get(f"/api/v1/projects/{pid}/export/pptx")

        assert resp.status_code == 200
        # Verify it's a non-empty binary response
        assert len(resp.content) > 0
        assert "attachment" in resp.headers.get("content-disposition", "")


class TestSmoke_ConcurrentAccess:
    """Verify basic concurrency safety."""

    def test_rapid_project_creation(self, api_client):
        """Create 10 projects rapidly without failures."""
        for i in range(10):
            resp = api_client.post("/api/v1/projects", json={"name": f"Smoke{i}"})
            assert resp.status_code == 201

    def test_rapid_list_and_create(self, api_client):
        """Interleave lists and creates."""
        for i in range(5):
            resp = api_client.post("/api/v1/projects", json={"name": f"Interleave{i}"})
            assert resp.status_code == 201
            list_resp = api_client.get("/api/v1/projects")
            assert list_resp.status_code == 200


# ============================================================================
# Smoke: Version Endpoints (Round 3 guards)
# ============================================================================


class TestSmoke_Versions:
    """Smoke tests for version endpoints with require_project guards."""

    def test_list_versions_empty_project(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "VersionSmoke"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/versions")
        assert resp.status_code == 200
        body = resp.json()
        assert body["project_id"] == pid
        assert body["versions"] == []
        assert body["current_version"] is None

    def test_list_versions_nonexistent_project(self, api_client):
        resp = api_client.get("/api/v1/projects/proj-deadbeef0000/versions")
        assert resp.status_code == 404

    def test_get_version_detail_nonexistent_project(self, api_client):
        resp = api_client.get("/api/v1/projects/proj-deadbeef0000/versions/v1")
        assert resp.status_code == 404

    def test_restore_version_nonexistent_project(self, api_client):
        resp = api_client.post("/api/v1/projects/proj-deadbeef0000/versions/v1/restore")
        assert resp.status_code == 404


# ============================================================================
# Smoke: Settings Endpoints (Round 3 build_provider_payload refactor)
# ============================================================================


class TestSmoke_Settings:
    """Smoke tests for settings endpoints (sync, test-connection, models)."""

    def test_sync_returns_200(self, api_client):
        mock_client = AsyncMock()
        mock_client.build_provider_payload = lambda req: {}
        mock_client.sync_provider_config = AsyncMock(return_value={"message": "OK"})

        with patch("src.api.settings.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/settings/sync",
                json={
                    "provider": "test",
                    "api_key": "k",
                    "base_url": "https://x.com",
                    "model_name": "m",
                },
            )
        assert resp.status_code == 200
        assert "ok" in resp.json()

    def test_test_connection_returns_200(self, api_client):
        mock_client = AsyncMock()
        mock_client.build_provider_payload = lambda req: {}
        mock_client.test_provider_config = AsyncMock(
            return_value={"ok": True, "message": "OK", "in_list": True, "verified": True}
        )

        with patch("src.api.settings.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/settings/test-connection",
                json={
                    "provider": "test",
                    "api_key": "k",
                    "base_url": "https://x.com",
                    "model_name": "m",
                },
            )
        assert resp.status_code == 200

    def test_list_models_returns_200(self, api_client):
        mock_client = AsyncMock()
        mock_client.build_provider_payload = lambda req: {}
        mock_client.list_provider_models = AsyncMock(
            return_value={"models": [], "source": "none", "error": ""}
        )

        with patch("src.api.settings.get_agent_client", return_value=mock_client):
            resp = api_client.post(
                "/api/v1/settings/models",
                json={
                    "provider": "test",
                    "api_key": "k",
                    "base_url": "https://x.com",
                    "model_name": "m",
                },
            )
        assert resp.status_code == 200
        assert "models" in resp.json()

    def test_settings_endpoints_validate_input(self, api_client):
        resp = api_client.post("/api/v1/settings/sync", json={})
        assert resp.status_code == 422


# ============================================================================
# Smoke: require_html in Export and File Endpoints (Round 4)
# ============================================================================


class TestSmoke_RequireHtml:
    """Export and download endpoints use require_html (Round 4 refactor)."""

    def test_json_export_without_html_404(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "SmokeNoHTML"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/export")
        assert resp.status_code == 404

    def test_download_without_html_404(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "SmokeNoDL"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/download")
        assert resp.status_code == 404

    def test_pdf_export_without_html_404(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "SmokeNoPDF"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/export/pdf")
        assert resp.status_code == 404

    def test_pptx_export_without_html_404(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "SmokeNoPPTX"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/export/pptx")
        assert resp.status_code == 404

    def test_preview_without_html_404(self, api_client):
        create = api_client.post("/api/v1/projects", json={"name": "SmokeNoPreview"})
        pid = create.json()["id"]
        resp = api_client.get(f"/api/v1/projects/{pid}/preview")
        assert resp.status_code == 404


# ============================================================================
# Smoke: inject_base_tag in Skills Preview (Round 6b)
# ============================================================================


class TestSmoke_SkillsPreview:
    """Skills preview endpoint smoke tests (Round 6b refactor)."""

    def test_preview_responds_200(self, api_client):
        mock_client = AsyncMock()
        mock_client.get_skill_example = AsyncMock(
            return_value=(200, "<html><head></head><body>slide</body></html>")
        )

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills/html-ppt-demo/preview")

        assert resp.status_code == 200
        assert "text/html" in resp.headers["content-type"]
        assert '<base href="/api/v1/skills/html-ppt-demo/">' in resp.text

    def test_preview_not_found(self, api_client):
        mock_client = AsyncMock()
        mock_client.get_skill_example = AsyncMock(return_value=(404, "not found"))

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills/nonexistent/preview")

        assert resp.status_code == 404
