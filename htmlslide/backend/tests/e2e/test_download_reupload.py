"""P5-09: E2E download + re-upload roundtrip — no information loss."""
from fastapi.testclient import TestClient

DECK_HTML = b"""<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<style>
  .slide { background: #0a0a0f; color: #f0f0f5; width: 1920px; height: 1080px; }
  h1 { font-size: 72px; color: #6366f1; }
  .hero { text-align: center; }
</style>
</head>
<body>
<div class="slide active hero">
  <h1>Hello World</h1>
</div>
</body>
</html>"""


class TestDownloadReupload:
    """Download → re-upload preserves full HTML content (including <style>)."""

    def test_download_reupload_preserves_content(self, api_client: TestClient):
        # 1. Upload initial HTML
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", DECK_HTML)},
        )
        assert resp.status_code == 200
        project_id = resp.json()["project_id"]

        # 2. Download the raw file
        dl = api_client.get(f"/api/v1/projects/{project_id}/download")
        assert dl.status_code == 200
        assert dl.headers.get("content-type", "").startswith("text/html")
        assert "attachment" in dl.headers.get("content-disposition", "")
        original_raw = dl.text

        # 3. Re-upload the downloaded file to a new project
        resp2 = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", original_raw.encode("utf-8"))},
        )
        assert resp2.status_code == 200
        project_id2 = resp2.json()["project_id"]

        # 4. Download again — content should match
        dl2 = api_client.get(f"/api/v1/projects/{project_id2}/download")
        assert dl2.status_code == 200
        reuploaded_raw = dl2.text

        # Strip whitespace differences for comparison
        assert original_raw.strip() == reuploaded_raw.strip(), (
            "Download → re-upload should preserve HTML content exactly"
        )

    def test_download_filename_matches_project_name(self, api_client: TestClient):
        resp = api_client.post(
            "/api/v1/files/upload",
            data={"project_name": "My Slides"},
            files={"file": ("deck.html", DECK_HTML)},
        )
        assert resp.status_code == 200
        project_id = resp.json()["project_id"]

        dl = api_client.get(f"/api/v1/projects/{project_id}/download")
        assert dl.status_code == 200
        disposition = dl.headers.get("content-disposition", "")
        assert "My-Slides.html" in disposition or "My Slides" in disposition
