"""Error handler edge cases and uncovered code paths."""
import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from src.constants.error_codes import ERROR_STATUS_MAP, ErrorCode
from src.middleware.error_handler import _code_for_status, register_error_handlers


class TestCodeForStatus:
    def test_known_status_404(self):
        assert _code_for_status(404) == ErrorCode.PROJECT_NOT_FOUND.value

    def test_known_status_500(self):
        # 500 maps to multiple codes; first in insertion order is FILE_READ_ERROR
        assert _code_for_status(500) in (
            ErrorCode.FILE_READ_ERROR.value,
            ErrorCode.INTERNAL_ERROR.value,
        )

    def test_known_status_502(self):
        assert _code_for_status(502) == ErrorCode.AGENT_ERROR.value

    def test_known_status_504(self):
        # 504 has no mapping → INTERNAL_ERROR
        assert _code_for_status(504) == ErrorCode.INTERNAL_ERROR.value

    def test_known_status_400(self):
        assert _code_for_status(400) == ErrorCode.UNSUPPORTED_FORMAT.value

    def test_known_status_413(self):
        assert _code_for_status(413) == ErrorCode.FILE_TOO_LARGE.value

    def test_known_status_422(self):
        # 422 maps to multiple codes; first in insertion order is EMPTY_FILE
        assert _code_for_status(422) in (
            ErrorCode.EMPTY_FILE.value,
            ErrorCode.VALIDATION_ERROR.value,
        )

    def test_unknown_status_returns_internal_error(self):
        """Coverage: line 54 — status code with no mapping returns INTERNAL_ERROR."""
        assert _code_for_status(418) == ErrorCode.INTERNAL_ERROR.value  # I'm a teapot
        assert _code_for_status(503) == ErrorCode.INTERNAL_ERROR.value


class TestErrorHandlerEdges:
    @pytest.fixture
    def client(self):
        app = FastAPI()
        register_error_handlers(app)

        @app.get("/raise-401")
        async def raise_401():
            from fastapi import HTTPException
            raise HTTPException(status_code=401, detail="Need auth")

        @app.get("/raise-418")
        async def raise_418():
            from fastapi import HTTPException
            raise HTTPException(status_code=418, detail="I'm a teapot")

        @app.get("/raise-422")
        async def raise_422():
            raise ValueError("Invalid input")

        @app.get("/raise-type-error")
        async def raise_type_error():
            raise TypeError("Bad type")

        return TestClient(app, raise_server_exceptions=False)

    def test_401_unmapped_status(self, client):
        """401 has no mapping in ERROR_STATUS_MAP → INTERNAL_ERROR code."""
        resp = client.get("/raise-401")
        assert resp.status_code == 401
        body = resp.json()
        # code comes from _code_for_status(401) which returns INTERNAL_ERROR
        assert body["code"] == ErrorCode.INTERNAL_ERROR.value

    def test_418_unmapped_status(self, client):
        """418 (I'm a teapot) → INTERNAL_ERROR code."""
        resp = client.get("/raise-418")
        assert resp.status_code == 418
        body = resp.json()
        assert body["code"] == ErrorCode.INTERNAL_ERROR.value

    def test_type_error_returns_500(self, client):
        """TypeError → generic handler → 500 INTERNAL_ERROR (not ValueError)."""
        resp = client.get("/raise-type-error")
        assert resp.status_code == 500
        body = resp.json()
        assert body["code"] == ErrorCode.INTERNAL_ERROR.value
        assert body["details"] is None

    def test_value_error_with_nested_details(self, client):
        """ValueError message should be preserved directly."""
        resp = client.get("/raise-422")
        assert resp.status_code == 422
        body = resp.json()
        assert "Invalid input" in body["message"]
