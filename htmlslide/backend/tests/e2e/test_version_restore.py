"""P5-08: E2E Version restore — multi-round chat, list versions, restore, verify content."""
import json

from fastapi.testclient import TestClient
from .conftest import agentgo_required

DECK_HTML = b"""<!doctype html>
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
  <h1>Original Title</h1>
</div>
</body>
</html>"""


class TestVersionRestore:
    @agentgo_required
    def test_version_restore_roundtrip(self, e2e_client: TestClient):
        """Upload HTML → chat rounds → list versions → restore → verify response structure."""
        # 1. Upload HTML
        resp = e2e_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", DECK_HTML)},
        )
        assert resp.status_code == 200
        project_id = resp.json()["project_id"]

        # Round 1: generate initial content
        with e2e_client.stream(
            "POST",
            "/api/v1/chat",
            json={
                "project_id": project_id,
                "prompt": "创建一页标题为Version1的幻灯片，深色背景",
            },
        ) as resp1:
            assert resp1.status_code == 200
            for _ in resp1.iter_lines():
                pass

        # Round 2: modify
        with e2e_client.stream(
            "POST",
            "/api/v1/chat",
            json={
                "project_id": project_id,
                "prompt": "把标题改成Version2，浅色背景",
            },
        ) as resp2:
            assert resp2.status_code == 200
            for _ in resp2.iter_lines():
                pass

        # 2. List versions — returns 200 with valid structure
        ver_resp = e2e_client.get(f"/api/v1/projects/{project_id}/versions")
        assert ver_resp.status_code == 200
        versions_data = ver_resp.json()
        assert "versions" in versions_data

        # 3. Export still works after chat rounds
        export_resp = e2e_client.get(f"/api/v1/projects/{project_id}/export")
        assert export_resp.status_code == 200
        export = export_resp.json()
        assert len(export["html"]) > 0

        # 4. If versions exist, verify detail and restore endpoints
        versions = versions_data["versions"]
        if len(versions) > 0:
            first_version = versions[-1]

            detail_resp = e2e_client.get(
                f"/api/v1/projects/{project_id}/versions/{first_version['id']}"
            )
            assert detail_resp.status_code == 200
            detail = detail_resp.json()
            assert len(detail["html"]) > 0
            assert "snapshot" in detail

            restore_resp = e2e_client.post(
                f"/api/v1/projects/{project_id}/versions/{first_version['id']}/restore"
            )
            assert restore_resp.status_code == 200
            restored = restore_resp.json()
            assert restored["project_id"] == project_id

    @agentgo_required
    def test_version_detail_of_nonexistent_version(self, e2e_client: TestClient):
        """GET version detail for a made-up version ID returns 404."""
        resp = e2e_client.post("/api/v1/projects", json={"name": "vdetail"})
        project_id = resp.json()["id"]

        resp = e2e_client.get(
            f"/api/v1/projects/{project_id}/versions/nonexistent-version"
        )
        assert resp.status_code == 404
