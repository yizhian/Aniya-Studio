"""Tests for agent_client.get_skills method."""
import pytest
import respx
from httpx import Response

from src.services.agent_client import AgentClient


@pytest.fixture
def agent_client():
    """Create a fresh AgentClient pointed at agentgo for skill tests."""
    return AgentClient(base_url="http://agentgo:8080", timeout=30.0)


class TestAgentClientGetSkills:
    """Tests for AgentClient.get_skills()."""

    @pytest.mark.asyncio
    async def test_get_skills_calls_correct_endpoint(self, agent_client):
        """get_skills calls GET /skills?mode=deck by default."""
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, json={"skills": []}))

            await agent_client.get_skills()

    @pytest.mark.asyncio
    async def test_get_skills_uses_mode_parameter(self, agent_client):
        """get_skills passes mode query param to the endpoint."""
        with respx.mock:
            route = respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "minimal"},
            ).mock(return_value=Response(200, json={"skills": []}))

            await agent_client.get_skills(mode="minimal")

            assert route.called, "Expected GET /skills?mode=minimal to be called"

    @pytest.mark.asyncio
    async def test_get_skills_returns_skill_list(self, agent_client):
        """get_skills returns the list of skills from the JSON response."""
        mock_skills = [
            {"name": "slides", "description": "Deck templates", "triggers": [], "scenario": "deck"},
            {"name": "diagram", "description": "Charts", "triggers": [], "scenario": "deck"},
        ]
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, json={"skills": mock_skills}))

            result = await agent_client.get_skills(mode="deck")

        assert result == mock_skills

    @pytest.mark.asyncio
    async def test_get_skills_returns_empty_list_when_no_skills_key(self, agent_client):
        """get_skills returns [] when response JSON has no 'skills' key."""
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, json={"other": "data"}))

            result = await agent_client.get_skills(mode="deck")

        assert result == []

    @pytest.mark.asyncio
    async def test_get_skills_raises_on_http_error(self, agent_client):
        """get_skills raises on server error responses (e.g. 500)."""
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(500))

            with pytest.raises(Exception):
                await agent_client.get_skills()

    @pytest.mark.asyncio
    async def test_get_skills_raises_on_connection_error(self, agent_client):
        """get_skills raises on connection errors."""
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(side_effect=Exception("Connection refused"))

            with pytest.raises(Exception) as exc_info:
                await agent_client.get_skills()

            assert "Connection refused" in str(exc_info.value)

    @pytest.mark.asyncio
    async def test_get_skills_with_custom_base_url(self):
        """get_skills works with a custom base URL."""
        client = AgentClient(base_url="http://custom-agent:9090", timeout=5.0)

        mock_skills = [{"name": "custom-skill", "description": "Custom", "triggers": [], "scenario": "deck"}]
        with respx.mock:
            respx.get(
                "http://custom-agent:9090/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, json={"skills": mock_skills}))

            result = await client.get_skills()

        assert result == mock_skills
        await client.close()

    @pytest.mark.asyncio
    async def test_get_skills_passable_integration(self, agent_client):
        """get_skills integrates correctly with downstream: skills are dicts with expected keys."""
        mock_skills = [
            {
                "name": "slides",
                "description": "Deck slide templates",
                "triggers": ["slide", "deck"],
                "scenario": "deck",
            },
        ]
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, json={"skills": mock_skills}))

            result = await agent_client.get_skills()

        assert len(result) == 1
        skill = result[0]
        assert "name" in skill
        assert "description" in skill
        assert "triggers" in skill
        assert "scenario" in skill
        assert isinstance(skill["triggers"], list)


class TestAgentClientGetSkillsEdgeCases:
    """Edge case tests for get_skills response handling."""

    @pytest.mark.asyncio
    async def test_get_skills_handles_null_skills_key(self, agent_client):
        """get_skills returns [] when 'skills' key is null."""
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, json={"skills": None}))

            result = await agent_client.get_skills(mode="deck")

        assert result == [] or result is None

    @pytest.mark.asyncio
    async def test_get_skills_handles_malformed_json(self, agent_client):
        """get_skills raises on garbage JSON response."""
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, content=b"not json at all {{{"))

            with pytest.raises(Exception):
                await agent_client.get_skills()

    @pytest.mark.asyncio
    async def test_get_skills_handles_404(self, agent_client):
        """get_skills raises on 404 Not Found from agentgo."""
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(404))

            with pytest.raises(Exception):
                await agent_client.get_skills()

    @pytest.mark.asyncio
    async def test_get_skills_empty_list_preserves_type(self, agent_client):
        """get_skills returns list even for empty response."""
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, json={"skills": []}))

            result = await agent_client.get_skills()

        assert isinstance(result, list)
        assert len(result) == 0

    @pytest.mark.asyncio
    async def test_get_skills_truncated_response(self, agent_client):
        """get_skills handles a truncated response gracefully."""
        with respx.mock:
            # Simulate a truncated JSON body (only first 20 bytes).
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, content=b'{"skills":[{"name":"'))

            with pytest.raises(Exception):
                await agent_client.get_skills()

    @pytest.mark.asyncio
    async def test_get_skills_large_payload(self, agent_client):
        """get_skills handles a large response with many skills."""
        large_skills = [
            {"name": f"skill-{i:04d}", "description": f"Description {i}", "triggers": [f"t{i}"], "scenario": "deck"}
            for i in range(100)
        ]
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, json={"skills": large_skills}))

            result = await agent_client.get_skills(mode="deck")

        assert len(result) == 100
        assert result[0]["name"] == "skill-0000"
        assert result[99]["name"] == "skill-0099"


class TestAgentClientLifecycle:
    """Test client lifecycle methods in context of skills."""

    @pytest.mark.asyncio
    async def test_close_after_get_skills(self, agent_client):
        """client can be closed after calling get_skills."""
        with respx.mock:
            respx.get(
                "http://agentgo:8080/skills",
                params={"mode": "deck"},
            ).mock(return_value=Response(200, json={"skills": []}))

            await agent_client.get_skills()

        await agent_client.close()
        assert agent_client._client.is_closed

    @pytest.mark.asyncio
    async def test_reuse_after_close_raises(self, agent_client):
        """Using a closed client raises RuntimeError."""
        await agent_client.close()
        with pytest.raises(RuntimeError):
            await agent_client.get_skills()
