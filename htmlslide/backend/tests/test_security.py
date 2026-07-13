"""Security tests — project_id injection, MIME forgery, oversized inputs."""
import pytest
from fastapi.testclient import TestClient


class TestProjectIdValidation:
    """P5-11: project_id path traversal injection is rejected.

    Starlette normalizes URL paths before routing, so `../etc` in a URL path
    becomes `/etc` which doesn't match the route pattern — producing a 404
    from Starlette itself. validate_project_id() adds defense-in-depth for
    body/form parameters where no routing normalization occurs.

    The key assertion: no malicious project_id ever returns 200.
    """

    # IDs with special chars that survive URL encoding — will hit our validator
    BODY_IDS = [
        "proj-../etc",
        "proj-..\\windows",
    ]

    # These get normalized by Starlette into non-matching routes → 404 from router
    URL_IDS = [
        "../etc",
        "/etc/passwd",
        "\\windows\\system32",
    ]

    @pytest.mark.parametrize("bad_id", URL_IDS)
    def test_get_project_rejects_normalized_paths(self, api_client: TestClient, bad_id: str):
        """Starlette normalizes ../ in paths → route mismatch → 404."""
        resp = api_client.get(f"/api/v1/projects/{bad_id}")
        # Starlette normalization: ../etc → /etc → doesn't match /api/v1/projects/{id}
        # Result is 404 from Starlette routing (not 422 from our validator)
        assert resp.status_code in (404, 422), f"expected 404/422 for {bad_id}, got {resp.status_code}"

    @pytest.mark.parametrize("bad_id", URL_IDS)
    def test_export_rejects_normalized_paths(self, api_client: TestClient, bad_id: str):
        resp = api_client.get(f"/api/v1/projects/{bad_id}/export")
        assert resp.status_code in (404, 422), f"expected 404/422 for {bad_id}, got {resp.status_code}"

    @pytest.mark.parametrize("bad_id", URL_IDS)
    def test_download_rejects_normalized_paths(self, api_client: TestClient, bad_id: str):
        resp = api_client.get(f"/api/v1/projects/{bad_id}/download")
        assert resp.status_code in (404, 422), f"expected 404/422 for {bad_id}, got {resp.status_code}"

    @pytest.mark.parametrize("bad_id", URL_IDS)
    def test_versions_rejects_normalized_paths(self, api_client: TestClient, bad_id: str):
        resp = api_client.get(f"/api/v1/projects/{bad_id}/versions")
        assert resp.status_code in (404, 422), f"expected 404/422 for {bad_id}, got {resp.status_code}"

    @pytest.mark.parametrize("bad_id", BODY_IDS)
    def test_chat_rejects_path_traversal_in_body(self, api_client: TestClient, bad_id: str):
        """project_id in JSON body hits validate_project_id → 422."""
        resp = api_client.post(
            "/api/v1/chat",
            json={"project_id": bad_id, "prompt": "hello"},
        )
        assert resp.status_code == 422, f"expected 422 for {bad_id}, got {resp.status_code}"

    @pytest.mark.parametrize("bad_id", BODY_IDS)
    def test_upload_rejects_path_traversal_in_form(self, api_client: TestClient, bad_id: str):
        """project_id in form data hits validate_project_id → 422."""
        resp = api_client.post(
            "/api/v1/files/upload",
            data={"project_id": bad_id},
            files={"file": ("deck.html", b"<!doctype html><html></html>")},
        )
        assert resp.status_code == 422, f"expected 422 for {bad_id}, got {resp.status_code}"

    def test_empty_project_id_rejected(self, api_client: TestClient):
        resp = api_client.get("/api/v1/projects/%20")
        assert resp.status_code in (404, 422)


class TestUploadMimeForgery:
    """P5-12: Forged MIME type is caught by python-magic.

    Note: python-magic may classify simple strings as text/plain, which is
    allowed alongside text/html. True forgery requires binary content that
    libmagic can't classify as text/*.
    """

    def test_png_disguised_as_html_rejected(self, api_client: TestClient):
        """Upload a .html file containing PNG binary data."""
        # Minimal PNG header bytes
        png_bytes = (
            b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01"
            b"\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde"
        )
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("fake.html", png_bytes)},
        )
        # python-magic detects image/png → 400 unsupported_format
        assert resp.status_code == 400

    def test_pdf_disguised_as_html_rejected(self, api_client: TestClient):
        """Upload a .html file containing PDF header bytes."""
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("fake.html", b"%PDF-1.4\n%\x80\x80\x80\x80")},
        )
        assert resp.status_code == 400

    def test_valid_html_accepted(self, api_client: TestClient):
        """Genuine HTML files should pass MIME check."""
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", b"<!doctype html><html><body><p>Hi</p></body></html>")},
        )
        assert resp.status_code == 200

    def test_empty_file_rejected(self, api_client: TestClient):
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("empty.html", b"")},
        )
        assert resp.status_code == 422


class TestPromptSizeLimit:
    """P5-13: Pydantic max_length on ChatRequest.prompt."""

    def test_oversized_prompt_rejected(self, api_client: TestClient):
        """Pydantic rejects prompts over 4000 characters with 422."""
        long_prompt = "x" * 4001
        resp = api_client.post(
            "/api/v1/chat",
            json={"project_id": "proj-test", "prompt": long_prompt},
        )
        assert resp.status_code == 422

    def test_max_length_prompt_accepted_by_validation(self, api_client: TestClient):
        """4000-char prompts should pass Pydantic validation."""
        resp = api_client.post(
            "/api/v1/chat",
            json={"project_id": "proj-test", "prompt": "x" * 4000},
        )
        assert resp.status_code != 422, "4000-char prompt should not be rejected by validation"
