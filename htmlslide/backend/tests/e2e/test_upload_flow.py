"""P5-07: E2E upload flow — upload HTML, verify response contains html+css+slide_count."""
import pytest
from fastapi.testclient import TestClient

MINIMAL_HTML = b"""<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<style>
  .slide { background: #111; color: #fff; width: 1920px; height: 1080px; }
  h1 { font-size: 64px; }
</style>
</head>
<body>
<div class="slide active">
  <h1>Welcome to Aniya</h1>
  <p>SaaS Landing Page</p>
</div>
<div class="slide">
  <h1>Features</h1>
  <p>Fast, reliable, secure</p>
</div>
<div class="slide">
  <h1>Pricing</h1>
  <p>Start free, scale up</p>
</div>
</body>
</html>"""


class TestUploadFlow:
    """Upload HTML file → get project + parsed HTML back in one response."""

    def test_upload_html_returns_all_fields(self, api_client: TestClient):
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", MINIMAL_HTML)},
        )
        assert resp.status_code == 200
        data = resp.json()

        # Required fields per FileUploadResponse schema
        assert "project_id" in data
        assert data["project_id"].startswith("proj-")
        assert data["file_name"] == "deck.html"
        assert data["file_size_bytes"] > 0

        # html field: body content without <style> tags
        assert "html" in data
        assert "<h1>" in data["html"]
        assert "<style>" not in data["html"]

        # css field: extracted <style> content
        assert "css" in data
        assert ".slide" in data["css"]
        assert "background" in data["css"]

        # slide_count: 3 slides in the test HTML
        assert data["slide_count"] == 3

        # is_deck: true when .slide elements exist
        assert data["is_deck"] is True

    def test_upload_with_project_name(self, api_client: TestClient):
        resp = api_client.post(
            "/api/v1/files/upload",
            data={"project_name": "My Custom Deck"},
            files={"file": ("slides.html", MINIMAL_HTML)},
        )
        assert resp.status_code == 200

        # Verify project was named correctly
        project_id = resp.json()["project_id"]
        proj_resp = api_client.get(f"/api/v1/projects/{project_id}")
        assert proj_resp.json()["name"] == "My Custom Deck"

    def test_upload_to_existing_project(self, api_client: TestClient):
        # Create project first
        resp = api_client.post("/api/v1/projects", json={"name": "existing"})
        project_id = resp.json()["id"]

        # Upload to that project
        resp = api_client.post(
            "/api/v1/files/upload",
            data={"project_id": project_id},
            files={"file": ("updated.html", MINIMAL_HTML)},
        )
        assert resp.status_code == 200
        assert resp.json()["project_id"] == project_id

    def test_upload_html_without_slides(self, api_client: TestClient):
        html = b"<!doctype html><html><body><p>Just a paragraph</p></body></html>"
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("simple.html", html)},
        )
        assert resp.status_code == 200
        assert resp.json()["is_deck"] is False
        assert resp.json()["slide_count"] == 0

    def test_upload_preserves_content_for_agentgo(self, api_client: TestClient):
        """After upload, the file on disk should contain the full HTML for AgentGo."""
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", MINIMAL_HTML)},
        )
        assert resp.status_code == 200
        project_id = resp.json()["project_id"]

        # Download should return the full HTML with <style> intact
        dl = api_client.get(f"/api/v1/projects/{project_id}/download")
        assert dl.status_code == 200
        raw = dl.text
        assert "<style>" in raw
        assert ".slide" in raw
        assert '<div class="slide active">' in raw
