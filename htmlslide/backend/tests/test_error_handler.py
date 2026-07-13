import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from src.constants.error_codes import ErrorCode
from src.middleware.error_handler import register_error_handlers


@pytest.fixture
def client():
    app = FastAPI()
    register_error_handlers(app)

    @app.get("/raise-404")
    async def raise_404():
        from fastapi import HTTPException
        raise HTTPException(status_code=404, detail="Not found")

    @app.get("/raise-422")
    async def raise_422():
        raise ValueError("Invalid input")

    @app.get("/raise-500")
    async def raise_500():
        raise RuntimeError("Boom")

    return TestClient(app, raise_server_exceptions=False)


class TestErrorHandler:
    def test_404_returns_standard_error(self, client):
        resp = client.get("/raise-404")
        assert resp.status_code == 404
        body = resp.json()
        assert body["code"] == "project_not_found"
        assert body["message"] == "Not found"

    def test_422_returns_validation_error(self, client):
        resp = client.get("/raise-422")
        assert resp.status_code == 422
        body = resp.json()
        assert body["code"] == ErrorCode.VALIDATION_ERROR.value
        assert "Invalid input" in body["message"]

    def test_500_returns_internal_error(self, client):
        resp = client.get("/raise-500")
        assert resp.status_code == 500
        body = resp.json()
        assert body["code"] == ErrorCode.INTERNAL_ERROR.value
        assert body["details"] is None
