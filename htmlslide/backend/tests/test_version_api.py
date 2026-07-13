import io
import json
import os


def _create_version_dir(workspace, name, mtime, context=None):
    """Helper: create a fake version directory."""
    v_dir = workspace / ".slidecraft" / "versions" / name
    v_dir.mkdir(parents=True)
    ctx = context or {"title": f"Version {name}", "slide_count": 3}
    (v_dir / "context.json").write_text(json.dumps(ctx))
    (v_dir / "index.html").write_text(
        "<html><style>body{color:red}</style><div class='slide'>a</div></html>"
    )
    os.utime(str(v_dir), (mtime, mtime))


class TestVersionList:
    def test_list_versions_empty(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "NoVersions"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/versions")
        assert resp.status_code == 200
        body = resp.json()
        assert body["project_id"] == pid
        assert body["current_version"] is None
        assert body["versions"] == []

    def test_list_versions_with_data(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "HasVersions"})
        pid = create_resp.json()["id"]

        _create_version_dir(temp_workspace / "projects" / pid, "v001", 1000)
        _create_version_dir(temp_workspace / "projects" / pid, "v002", 2000)

        resp = api_client.get(f"/api/v1/projects/{pid}/versions")
        assert resp.status_code == 200
        body = resp.json()
        assert body["current_version"] == "v002"
        assert len(body["versions"]) == 2
        assert body["versions"][0]["current"] is True
        assert body["versions"][0]["tag"] == "V2"


class TestVersionDetail:
    def test_get_version_detail(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "VDetail"})
        pid = create_resp.json()["id"]

        _create_version_dir(temp_workspace / "projects" / pid, "v001", 1000, {"title": "My Deck", "slide_count": 5})

        resp = api_client.get(f"/api/v1/projects/{pid}/versions/v001")
        assert resp.status_code == 200
        body = resp.json()
        assert body["id"] == "v001"
        assert body["tag"] == "V1"
        assert body["title"] == "My Deck"
        assert body["snapshot"]["slide_count"] == 5
        assert "body{color:red}" in body["css"]

    def test_version_not_found(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "VDetail"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/versions/v999")
        assert resp.status_code == 404


class TestSaveVersion:
    """Coverage: version.py lines 50-69 — save_version endpoint."""

    def test_save_version_creates_and_returns_detail(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "SaveVer"})
        pid = create_resp.json()["id"]
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        html = b"<html><style>.s{}</style><div class='slide'>x</div></html>"
        api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )

        resp = api_client.post(
            f"/api/v1/projects/{pid}/versions",
            json={"title": "My Version", "html": "<html><div>v1</div></html>"},
        )
        assert resp.status_code == 200
        body = resp.json()
        assert body["project_id"] == pid
        assert body["version_id"].startswith("v")
        assert body["title"] == "My Version"
        assert body["version_tag"].startswith("V")
        assert "v1" in body["html"]
        assert "created_at" in body

    def test_save_version_default_title(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "DefaultTitle"})
        pid = create_resp.json()["id"]
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        html = b"<html><div class='slide'>x</div></html>"
        api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )

        resp = api_client.post(
            f"/api/v1/projects/{pid}/versions",
            json={"html": "<html><div>no title</div></html>"},
        )
        assert resp.status_code == 200
        assert resp.json()["title"] == "用户手动修改"

    def test_save_version_no_html_fails(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "NoHTML"})
        pid = create_resp.json()["id"]

        resp = api_client.post(
            f"/api/v1/projects/{pid}/versions",
            json={"title": "test", "html": "<html></html>"},
        )
        assert resp.status_code == 400
        assert "无 HTML 内容" in resp.json()["message"]


class TestVersionRestore:
    def test_restore_version(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "Restore"})
        pid = create_resp.json()["id"]

        _create_version_dir(temp_workspace / "projects" / pid, "v001", 1000, {"title": "Original"})

        resp = api_client.post(f"/api/v1/projects/{pid}/versions/v001/restore")
        assert resp.status_code == 200
        body = resp.json()
        assert body["project_id"] == pid
        assert body["restored_to"] == "v001"
        assert "div" in body["html"]
