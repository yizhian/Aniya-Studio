class TestCreateProject:
    def test_create_project_201(self, api_client):
        resp = api_client.post("/api/v1/projects", json={"name": "My Project"})
        assert resp.status_code == 201
        body = resp.json()
        assert body["name"] == "My Project"
        assert body["id"].startswith("proj-")
        assert body["current_version"] is None
        assert body["has_html"] is False
        assert body["slide_count"] == 0

    def test_create_project_default_name(self, api_client):
        resp = api_client.post("/api/v1/projects", json={})
        assert resp.status_code == 201
        assert resp.json()["name"] == "未命名项目"


class TestGetProject:
    def test_get_existing_project(self, api_client):
        # Create first
        create_resp = api_client.post("/api/v1/projects", json={"name": "Test"})
        pid = create_resp.json()["id"]

        # Get
        resp = api_client.get(f"/api/v1/projects/{pid}")
        assert resp.status_code == 200
        body = resp.json()
        assert body["id"] == pid
        assert body["name"] == "Test"

    def test_get_nonexistent_project_returns_empty(self, api_client):
        # MVP: when project.json doesn't exist, read_meta returns {}
        # get_project_info still returns a response (with "any-id" as name)
        resp = api_client.get("/api/v1/projects/nonexistent")
        # The workspace has no project.json, so read_meta returns {}
        # But the endpoint checks for empty meta and returns 404
        assert resp.status_code == 404


class TestUpdateProject:
    def test_update_project_200(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "Old Name"})
        pid = create_resp.json()["id"]

        resp = api_client.patch(f"/api/v1/projects/{pid}", json={"name": "New Name"})
        assert resp.status_code == 200
        body = resp.json()
        assert body["name"] == "New Name"
        assert body["id"] == pid

    def test_update_project_404(self, api_client):
        resp = api_client.patch("/api/v1/projects/nonexistent", json={"name": "X"})
        assert resp.status_code == 404

    def test_update_project_422_empty_name(self, api_client):
        create_resp = api_client.post("/api/v1/projects", json={"name": "Test"})
        pid = create_resp.json()["id"]

        resp = api_client.patch(f"/api/v1/projects/{pid}", json={"name": ""})
        assert resp.status_code == 422
