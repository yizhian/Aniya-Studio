import tempfile
from pathlib import Path

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient


@pytest.fixture
def temp_workspace():
    """Create a temporary workspace directory for isolated repository tests."""
    with tempfile.TemporaryDirectory() as tmpdir:
        yield Path(tmpdir)


@pytest.fixture
def api_client(temp_workspace):
    """Create a TestClient with isolated temp workspace for integration tests."""
    from src.api import deps
    from src.config import settings

    # Override workspace path
    settings.workspace_path = temp_workspace

    # Clear caches so new singletons use the temp workspace
    deps._workspace_service = None
    deps._agent_client = None
    deps._file_repo_cache.clear()
    deps._version_repo_cache.clear()
    deps._project_service_cache.clear()

    from src.api.router import router as api_router
    from src.middleware.error_handler import register_error_handlers

    app = FastAPI()
    register_error_handlers(app)
    app.include_router(api_router)

    with TestClient(app, raise_server_exceptions=False) as client:
        yield client
