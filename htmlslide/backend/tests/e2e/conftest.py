"""E2E test fixtures — conditionally skip if AgentGo is not reachable."""
import os
import tempfile
from pathlib import Path

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient


def _agentgo_reachable() -> bool:
    """Check whether a real AgentGo instance is available."""
    import httpx
    url = os.environ.get("AGENT_URL", "http://localhost:8080")
    try:
        resp = httpx.get(f"{url}/health", timeout=2.0)
        return resp.status_code == 200
    except Exception:
        return False


agentgo_required = pytest.mark.skipif(
    not _agentgo_reachable(),
    reason="AgentGo not reachable — start it with: docker compose up -d agentgo",
)


@pytest.fixture
def e2e_workspace():
    """Create a temporary workspace for E2E tests.

    Overrides settings.workspace_path so backend reads/writes from temp dir.
    """
    from src.api import deps
    from src.config import settings

    with tempfile.TemporaryDirectory() as tmpdir:
        original_workspace = settings.workspace_path
        original_agent = settings.agent_url
        settings.workspace_path = Path(tmpdir)
        settings.agent_url = os.environ.get("AGENT_URL", "http://localhost:8080")

        deps._workspace_service = None
        deps._agent_client = None
        deps._file_repo_cache.clear()
        deps._version_repo_cache.clear()
        deps._project_service_cache.clear()

        yield Path(tmpdir)

        settings.workspace_path = original_workspace
        settings.agent_url = original_agent
        deps._workspace_service = None
        deps._agent_client = None
        deps._file_repo_cache.clear()
        deps._version_repo_cache.clear()
        deps._project_service_cache.clear()


@pytest.fixture
def e2e_client(e2e_workspace):
    """TestClient wired to temp workspace and real AgentGo."""
    from src.api.router import router as api_router
    from src.middleware.error_handler import register_error_handlers

    app = FastAPI()
    register_error_handlers(app)
    app.include_router(api_router)

    with TestClient(app, raise_server_exceptions=False) as client:
        yield client
