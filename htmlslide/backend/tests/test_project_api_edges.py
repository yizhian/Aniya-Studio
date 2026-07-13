"""Project API edge cases."""
import io


class TestProjectEdges:
    def test_create_with_long_name(self, api_client):
        """Create project with a long but valid name."""
        name = "A" * 200
        resp = api_client.post("/api/v1/projects", json={"name": name})
        assert resp.status_code == 201
        assert resp.json()["name"] == name

    def test_create_with_empty_name_validation(self, api_client):
        """Empty name (below min_length=1) should be rejected by Pydantic validation."""
        resp = api_client.post("/api/v1/projects", json={"name": ""})
        assert resp.status_code == 422  # Pydantic validation error

    def test_full_project_lifecycle(self, api_client, temp_workspace):
        """End-to-end: create → upload → export → versions."""
        # 1. Create
        c = api_client.post("/api/v1/projects", json={"name": "Lifecycle"})
        assert c.status_code == 201
        pid = c.json()["id"]

        # 2. Get (should have no HTML — templates removed)
        g = api_client.get(f"/api/v1/projects/{pid}")
        assert g.status_code == 200
        assert g.json()["has_html"] is False

        # Pre-create a stub HTML file (upload's write_html needs an active file)
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "stub.html").write_text("")

        # 3. Upload
        html = b"<html><style>.s{color:red}</style><div class='slide'>1</div><div class='slide'>2</div></html>"
        u = api_client.post(
            "/api/v1/files/upload",
            files={"file": ("deck.html", io.BytesIO(html), "text/html")},
            data={"project_id": pid},
        )
        assert u.status_code == 200

        # 4. Get again (should have HTML now)
        g2 = api_client.get(f"/api/v1/projects/{pid}")
        assert g2.status_code == 200
        assert g2.json()["has_html"] is True
        assert g2.json()["slide_count"] == 2

        # 5. Export
        e = api_client.get(f"/api/v1/projects/{pid}/export")
        assert e.status_code == 200
        assert ".s{color:red}" in e.json()["css"]

        # 6. Download
        d = api_client.get(f"/api/v1/projects/{pid}/download")
        assert d.status_code == 200
        assert b"<style>" in d.content  # Raw HTML with <style> tags
