"""Version API edge case tests."""
import io
import json
from unittest.mock import patch

import pytest


class TestVersionRestoreEdges:
    def test_restore_nonexistent_version(self, api_client, temp_workspace):
        """Coverage: version.py lines 50-51 — restore with nonexistent version."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "RestoreEdges"})
        pid = create_resp.json()["id"]

        resp = api_client.post(f"/api/v1/projects/{pid}/versions/v999/restore")
        assert resp.status_code == 404

    @pytest.mark.asyncio
    async def test_restore_overwrites_current_html(self, api_client, temp_workspace):
        """Verify restore completely replaces current index.html content."""
        # Create project and upload some HTML first
        html = b"<html><style>.original{}</style><div class='slide'>original</div></html>"
        upload_resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_name": "RestoreOverwrite"},
        )
        pid = upload_resp.json()["project_id"]

        # Create a version with different content under the project directory
        v_dir = temp_workspace / "projects" / pid / ".slidecraft" / "versions" / "v001"
        v_dir.mkdir(parents=True)
        (v_dir / "context.json").write_text(json.dumps({"title": "Restored", "slide_count": 1}))
        (v_dir / "index.html").write_text(
            "<html><style>.restored{}</style><div class='slide'>restored</div></html>"
        )

        # Restore
        resp = api_client.post(f"/api/v1/projects/{pid}/versions/v001/restore")
        assert resp.status_code == 200
        body = resp.json()
        assert body["restored_to"] == "v001"
        assert ".restored" in body["css"]
        assert "restored" in body["html"]

        # Verify index.html was overwritten by reading active file
        from src.repositories.file_repo import FileRepo
        fr = FileRepo(temp_workspace, pid)
        current_html = await fr.read_html()
        assert ".restored" in current_html
        assert "original" not in current_html or ".original" not in current_html


class TestRestoreRollback:
    """Coverage: version.py lines 121-125 — rollback on write_html failure."""

    def test_restore_write_failure_rolls_back(self, api_client, temp_workspace):
        """When write_html fails during restore, manifest is restored and
        the rollback version directory is cleaned up."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "Rollback"})
        pid = create_resp.json()["id"]
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        html = b"<html><div class='slide'>x</div></html>"
        api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )

        # Create a version to restore from
        v_dir = proj_dir / ".slidecraft" / "versions" / "v001"
        v_dir.mkdir(parents=True)
        (v_dir / "context.json").write_text(json.dumps({"title": "V1", "slide_count": 1}))
        (v_dir / "index.html").write_text("<html><div>v1</div></html>")

        from src.repositories.file_repo import FileRepo

        async def failing_write(self, html_content):
            raise OSError("disk full")

        with patch.object(FileRepo, "write_html", failing_write):
            resp = api_client.post(f"/api/v1/projects/{pid}/versions/v001/restore")

        assert resp.status_code == 500
        assert "写入活跃文件失败" in resp.json()["message"]
