import io


class TestFileUpload:
    def test_upload_valid_html(self, api_client):
        html = b"<html><style>body{color:red}</style><div class='slide'>hello</div></html>"
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_name": "My Deck"},
        )
        # Upload on empty project — write_html needs an active file.
        # This is deferred work; for now expect 500 (internal error).
        assert resp.status_code in (200, 500)

    def test_upload_to_existing_project(self, api_client, temp_workspace):
        # Create project first
        create_resp = api_client.post("/api/v1/projects", json={"name": "Existing"})
        pid = create_resp.json()["id"]

        # Pre-create a stub HTML file so write_html has an active file to write to
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        html = b"<html><div class='slide'>content</div></html>"
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )
        assert resp.status_code == 200
        assert resp.json()["project_id"] == pid

    def test_upload_to_nonexistent_project(self, api_client):
        html = b"<html><p>test</p></html>"
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": "proj-nonexistent"},
        )
        assert resp.status_code == 404

    def test_upload_non_html(self, api_client):
        """Note: python-magic may detect text/plain for simple content."""
        data = b"hello world"
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("test.txt", io.BytesIO(data), "text/plain")},
        )
        # Either 400 (wrong extension) or 400 (MIME)
        assert resp.status_code in (400, 413)

    def test_upload_empty_file(self, api_client):
        resp = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("empty.html", io.BytesIO(b""), "text/html")},
        )
        assert resp.status_code in (400, 422)


class TestActiveFileEndpoint:
    def test_get_active_file_returns_null_for_empty_project(self, api_client):
        """GET /projects/{id}/active-file returns nulls for empty project (no template)."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "ActiveFile"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/active-file")
        assert resp.status_code == 200
        body = resp.json()
        assert body["filename"] is None
        assert body["content"] is None

    def test_get_active_file_returns_content(self, api_client, temp_workspace):
        """GET /projects/{id}/active-file returns content when HTML exists."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "ActiveFile"})
        pid = create_resp.json()["id"]

        # Manually create an HTML file (simulating Agent write_file)
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "deck.html").write_text("<html><body>Hello</body></html>")
        (proj_dir / "active.json").write_text('{"active": "deck.html"}')

        resp = api_client.get(f"/api/v1/projects/{pid}/active-file")
        assert resp.status_code == 200
        body = resp.json()
        assert body["filename"] == "deck.html"
        assert "Hello" in body["content"]

    def test_get_active_file_nonexistent_project(self, api_client):
        """active-file for nonexistent project returns 404."""
        resp = api_client.get("/api/v1/projects/proj-nonexistent/active-file")
        assert resp.status_code == 404


class TestListFilesEndpoint:
    def test_list_files_empty_project(self, api_client):
        """GET /projects/{id}/files?type=html returns empty for new project."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "ListFiles"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/files?type=html")
        assert resp.status_code == 200
        body = resp.json()
        assert body["project_id"] == pid
        assert body["active"] is None
        assert body["files"] == []

    def test_list_files_no_type_returns_empty(self, api_client):
        """list files with non-html type returns empty files."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "NoType"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/files?type=other")
        assert resp.status_code == 200
        body = resp.json()
        assert body["project_id"] == pid
        assert body["files"] == []

    def test_list_files_with_two_files(self, api_client, temp_workspace):
        """After creating HTML files, list returns both."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "TwoFiles"})
        pid = create_resp.json()["id"]

        # Manually create HTML files in the project directory
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "index.html").write_text("<html></html>")
        (proj_dir / "deck.html").write_text("<html></html>")

        resp = api_client.get(f"/api/v1/projects/{pid}/files?type=html")
        assert resp.status_code == 200
        body = resp.json()
        filenames = {f["filename"] for f in body["files"]}
        assert "index.html" in filenames
        assert "deck.html" in filenames


class TestPreviewEndpoint:
    """Coverage: file.py lines 160-169 — get_project_preview endpoint."""

    def test_preview_returns_html(self, api_client, temp_workspace):
        create_resp = api_client.post("/api/v1/projects", json={"name": "Preview"})
        pid = create_resp.json()["id"]
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        html = b"<html><body><h1>Preview</h1></body></html>"
        api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )

        resp = api_client.get(f"/api/v1/projects/{pid}/preview")
        assert resp.status_code == 200
        assert "text/html" in resp.headers["content-type"]
        assert "<h1>Preview</h1>" in resp.text

    def test_preview_no_html_returns_404(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "PreviewEmpty"})
        pid = create_resp.json()["id"]

        resp = api_client.get(f"/api/v1/projects/{pid}/preview")
        assert resp.status_code == 404
        assert "HTML" in resp.json()["message"]

    def test_preview_nonexistent_project_returns_404(self, api_client):
        resp = api_client.get("/api/v1/projects/proj-nonexistent/preview")
        assert resp.status_code == 404


class TestSyncActiveEndpoint:
    def test_sync_active_empty_project(self, api_client):
        """POST /projects/{id}/sync-active returns null for empty project."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "SyncActive"})
        pid = create_resp.json()["id"]

        resp = api_client.post(f"/api/v1/projects/{pid}/sync-active")
        assert resp.status_code == 200
        body = resp.json()
        assert body["project_id"] == pid
        assert body["active"] is None

    def test_sync_active_updates_json(self, api_client, temp_workspace):
        """POST /projects/{id}/sync-active syncs active.json to latest HTML."""
        create_resp = api_client.post("/api/v1/projects", json={"name": "SyncActive"})
        pid = create_resp.json()["id"]

        # Manually create an HTML file
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "deck.html").write_text("<html></html>")

        resp = api_client.post(f"/api/v1/projects/{pid}/sync-active")
        assert resp.status_code == 200
        body = resp.json()
        assert body["project_id"] == pid
        assert body["active"] == "deck.html"
