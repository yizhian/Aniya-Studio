import io
from unittest.mock import patch


class TestExport:
    def test_export_returns_json(self, api_client, temp_workspace):
        # Create project and pre-create a stub HTML file
        create_resp = api_client.post("/api/v1/projects", json={"name": "Deck"})
        pid = create_resp.json()["id"]
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        # Upload
        html = b"<html><style>.slide{}</style><div class='slide'>x</div></html>"
        upload_resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )
        assert upload_resp.status_code == 200

        resp = api_client.get(f"/api/v1/projects/{pid}/export")
        assert resp.status_code == 200
        body = resp.json()
        assert body["project_id"] == pid
        assert ".slide{}" in body["css"]
        assert body["slide_count"] == 1
        assert body["version"] is not None

    def test_export_empty_project_returns_404(self, api_client):
        """Empty project (no HTML) → export returns 404."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "Empty"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/export")
        assert resp.status_code == 404


class TestDownload:
    def test_download_returns_html_file(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "My Deck"})
        pid = create_resp.json()["id"]
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        html = b"<html><style>.slide{}</style><div class='slide'>x</div></html>"
        upload_resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )
        assert upload_resp.status_code == 200

        resp = api_client.get(f"/api/v1/projects/{pid}/download")
        assert resp.status_code == 200
        assert "text/html" in resp.headers["content-type"]
        assert "attachment" in resp.headers["content-disposition"]
        assert b"<style>" in resp.content

    def test_download_empty_project_returns_404(self, api_client):
        """Empty project (no HTML) → download returns 404."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "Empty"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/download")
        assert resp.status_code == 404

    def test_download_null_active_file_edge(self, api_client, temp_workspace):
        """Download returns 404 when project exists but has no HTML content.

        The previous race-condition scenario (html_exists passes then
        resolve_active_file returns None) is no longer reachable since
        require_html consolidates both checks atomically. This test now
        verifies the clean 404 path for a project without HTML."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "EdgeDL"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/download")
        assert resp.status_code == 404
        assert resp.json()["code"] == "file_read_error"
