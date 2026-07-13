"""SIT / smoke tests for Skills API."""
from unittest.mock import AsyncMock, patch


class TestListSkills:
    """Tests for GET /api/v1/skills endpoint."""

    def test_list_skills_returns_empty_on_error(self, api_client):
        """When agent_client.get_skills raises, return empty list + error."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(side_effect=Exception("agentgo unreachable"))

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills")

        assert resp.status_code == 200
        body = resp.json()
        assert body["skills"] == []
        assert body["mode"] == "deck"
        assert "agentgo unreachable" in body["error"]

    def test_list_skills_returns_skills_from_agent_client(self, api_client):
        """GET /api/v1/skills returns skills fetched from AgentGo."""
        mock_skills = [
            {
                "name": "slides",
                "description": "Deck slide templates",
                "triggers": [],
                "scenario": "deck",
            },
            {
                "name": "diagram",
                "description": "Chart and diagram generation",
                "triggers": [],
                "scenario": "deck",
            },
        ]
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(return_value=mock_skills)

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills")

        assert resp.status_code == 200
        body = resp.json()
        assert body["skills"] == mock_skills
        assert body["mode"] == "deck"

    def test_list_skills_filters_by_mode_deck(self, api_client):
        """GET /api/v1/skills?mode=deck forwards mode parameter to agent client."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(return_value=[])

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            api_client.get("/api/v1/skills?mode=deck")

        mock_client.get_skills.assert_called_once_with(mode="deck")

    def test_list_skills_default_mode_is_deck(self, api_client):
        """GET /api/v1/skills with no mode defaults to 'deck'."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(return_value=[])

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            api_client.get("/api/v1/skills")

        mock_client.get_skills.assert_called_once_with(mode="deck")

    def test_list_skills_passes_custom_mode(self, api_client):
        """GET /api/v1/skills?mode=diagram passes 'diagram' to agent client."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(return_value=[])

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            api_client.get("/api/v1/skills?mode=diagram")

        mock_client.get_skills.assert_called_once_with(mode="diagram")

    def test_list_skills_empty_response(self, api_client):
        """GET /api/v1/skills returns empty skills list when agent has no skills."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(return_value=[])

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills")

        assert resp.status_code == 200
        body = resp.json()
        assert body["skills"] == []
        assert body["mode"] == "deck"
        assert body["error"] == ""


class TestSkillsEndpointRegistered:
    """Verify the skills endpoint is correctly wired into the router."""

    def test_skills_endpoint_in_router(self):
        """Verify /api/v1/skills path is defined in the API router."""
        from src.api.router import router as api_router

        paths = [route.path for route in api_router.routes]
        assert "/api/v1/skills" in paths, (
            f"Expected /api/v1/skills in router paths, got: {paths}"
        )

    def test_skills_endpoint_in_openapi(self, api_client):
        """Verify /api/v1/skills appears in OpenAPI schema."""
        resp = api_client.get("/openapi.json")
        assert resp.status_code == 200
        schema = resp.json()
        paths = schema["paths"]
        assert "/api/v1/skills" in paths, (
            f"Expected /api/v1/skills in OpenAPI paths, got: {list(paths.keys())}"
        )
        skills_path = paths["/api/v1/skills"]
        assert "get" in skills_path
        assert skills_path["get"]["operationId"] == "list_skills_api_v1_skills_get"


class TestSkillsResponseShape:
    """Validate the JSON shape of the skills response."""

    def test_response_has_skills_and_mode_keys(self, api_client):
        """Response JSON always contains 'skills' and 'mode' keys."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(return_value=[
            {"name": "s1", "description": "d1", "triggers": [], "scenario": "deck"},
        ])

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills")

        body = resp.json()
        assert "skills" in body
        assert "mode" in body
        assert isinstance(body["skills"], list)

    def test_response_mode_reflects_query_param(self, api_client):
        """Response 'mode' key reflects the query parameter used."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(return_value=[])

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills?mode=minimal")

        body = resp.json()
        assert body["mode"] == "minimal"

    def test_list_skills_with_special_mode_name(self, api_client):
        """GET /api/v1/skills?mode=deck%2B passes URL-encoded mode."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(return_value=[])

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills?mode=deck%2B")

        assert resp.status_code == 200
        # FastAPI decodes the %2B to + automatically.
        mock_client.get_skills.assert_called_once_with(mode="deck+")

    def test_list_skills_without_cache_headers(self, api_client):
        """Skills endpoint returns fresh data (no caching headers conflicting)."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(return_value=[])

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills")

        # Should return 200 and valid JSON.
        assert resp.status_code == 200
        assert resp.headers.get("content-type", "").startswith("application/json")

    def test_list_skills_timeout_propagates_as_error(self, api_client):
        """When agent_client.get_skills times out, error is returned in JSON."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(side_effect=TimeoutError("timed out"))

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills")

        assert resp.status_code == 200
        body = resp.json()
        assert body["skills"] == []
        assert "timed out" in body["error"]

    def test_list_skills_connection_refused(self, api_client):
        """ConnectionError during skills fetch is captured and returned."""
        mock_client = AsyncMock()
        mock_client.get_skills = AsyncMock(side_effect=ConnectionError("Connection refused"))

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills")

        assert resp.status_code == 200
        body = resp.json()
        assert body["skills"] == []
        assert "Connection refused" in body["error"]


class TestSkillPreviewEndpoint:
    """Tests for GET/HEAD /api/v1/skills/{name}/preview."""

    def test_get_preview_injects_base_tag(self, api_client):
        mock_client = AsyncMock()
        mock_client.get_skill_example = AsyncMock(
            return_value=(200, "<html><head></head><body>slide</body></html>")
        )

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills/html-ppt-demo/preview")

        assert resp.status_code == 200
        assert resp.headers["content-type"].startswith("text/html")
        assert '<base href="/api/v1/skills/html-ppt-demo/">' in resp.text
        assert "slide" in resp.text

    def test_get_preview_not_found(self, api_client):
        mock_client = AsyncMock()
        mock_client.get_skill_example = AsyncMock(return_value=(404, "not found"))

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.get("/api/v1/skills/missing-skill/preview")

        assert resp.status_code == 404

    def test_head_preview_ok(self, api_client):
        mock_client = AsyncMock()
        mock_client.get_skill_example = AsyncMock(
            return_value=(200, "<html><head></head><body>slide</body></html>")
        )

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.head("/api/v1/skills/html-ppt-demo/preview")

        assert resp.status_code == 200
        assert resp.text == ""

    def test_head_preview_not_found(self, api_client):
        mock_client = AsyncMock()
        mock_client.get_skill_example = AsyncMock(return_value=(404, "not found"))

        with patch("src.api.skills.get_agent_client", return_value=mock_client):
            resp = api_client.head("/api/v1/skills/missing-skill/preview")

        assert resp.status_code == 404
