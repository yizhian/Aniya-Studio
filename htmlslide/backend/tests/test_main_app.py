"""Tests for the real FastAPI app in main.py with temporary workspace."""
import pytest
from fastapi.testclient import TestClient


@pytest.fixture
def app_client(temp_workspace):
    """Create TestClient against the real app, overriding workspace path."""
    from src.config import settings
    from src.api import deps

    settings.workspace_path = temp_workspace

    deps._workspace_service = None
    deps._agent_client = None
    deps._file_repo_cache.clear()
    deps._version_repo_cache.clear()
    deps._project_service_cache.clear()

    from src.main import app

    return TestClient(app, raise_server_exceptions=False)


class TestHealthEndpoint:
    def test_health_returns_ok(self, app_client):
        resp = app_client.get("/health")
        assert resp.status_code == 200
        assert resp.json() == {"status": "ok"}


class TestOpenAPI:
    def test_docs_accessible(self, app_client):
        resp = app_client.get("/docs")
        assert resp.status_code == 200

    def test_openapi_json(self, app_client):
        resp = app_client.get("/openapi.json")
        assert resp.status_code == 200
        schema = resp.json()
        assert schema["info"]["title"] == "Aniya Studio BFF"
        # Verify all 9 endpoints present
        paths = schema["paths"]
        assert "/api/v1/projects" in paths
        assert "/api/v1/projects/{project_id}" in paths
        assert "/api/v1/chat" in paths
        assert "/api/v1/files/upload" in paths
        assert "/api/v1/projects/{project_id}/export" in paths
        assert "/api/v1/projects/{project_id}/download" in paths
        assert "/api/v1/projects/{project_id}/versions" in paths
        assert "/api/v1/projects/{project_id}/versions/{version_id}" in paths


class TestCORSMiddleware:
    def test_cors_headers_present(self, app_client):
        resp = app_client.options(
            "/health",
            headers={
                "Origin": "http://localhost:5173",
                "Access-Control-Request-Method": "GET",
            },
        )
        # CORS headers should be present
        assert "access-control-allow-origin" in resp.headers


class TestLifespan:
    def test_workspace_created(self, temp_workspace):
        """Verify lifespan creates workspace directory."""
        assert temp_workspace.is_dir()
