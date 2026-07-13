import pytest
import respx
from httpx import Response
from src.services.agent_client import AgentClient


@pytest.fixture
def agent_client():
    return AgentClient(base_url="http://agentgo:8080", timeout=30.0)


class TestAgentClient:
    @pytest.mark.asyncio
    async def test_chat_stream(self, agent_client):
        with respx.mock:
            sse_lines = [
                'data: {"type":"thinking","data":{"text":"Let me think..."},"round":1}',
                'data: {"type":"text","data":{"text":"Here is your slide"},"round":1}',
            ]
            body = "\n".join(sse_lines)
            respx.post("http://agentgo:8080/chat").mock(
                return_value=Response(200, content=body.encode())
            )

            lines = []
            async for line in agent_client.chat_stream("make a slide", "proj-123"):
                lines.append(line)

            assert len(lines) > 0
            # Should have parsed lines
            content = [l.decode("utf-8") if isinstance(l, bytes) else l for l in lines]
            assert any("thinking" in c for c in content)
            assert any("Here is your slide" in c for c in content)

    @pytest.mark.asyncio
    async def test_chat_stream_http_error(self, agent_client):
        with respx.mock:
            respx.post("http://agentgo:8080/chat").mock(return_value=Response(502))
            with pytest.raises(Exception):
                async for _ in agent_client.chat_stream("msg", "proj-123"):
                    pass

    @pytest.mark.asyncio
    async def test_close(self, agent_client):
        await agent_client.close()
        assert agent_client._client.is_closed

    @pytest.mark.asyncio
    async def test_get_session_found(self, agent_client):
        with respx.mock:
            respx.get("http://agentgo:8080/sessions/proj-123").mock(
                return_value=Response(200, json={"session_id": "proj-123", "messages": []})
            )
            session = await agent_client.get_session("proj-123")
            assert session is not None
            assert session["session_id"] == "proj-123"

    @pytest.mark.asyncio
    async def test_get_session_not_found(self, agent_client):
        with respx.mock:
            respx.get("http://agentgo:8080/sessions/proj-nonexistent").mock(
                return_value=Response(404)
            )
            session = await agent_client.get_session("proj-nonexistent")
            assert session is None

    @pytest.mark.asyncio
    async def test_get_skills(self, agent_client):
        with respx.mock:
            respx.get("http://agentgo:8080/skills", params={"mode": "deck"}).mock(
                return_value=Response(200, json={"skills": [
                    {"name": "slides", "description": "Slide deck skill"},
                ]})
            )
            skills = await agent_client.get_skills("deck")
            assert len(skills) == 1
            assert skills[0]["name"] == "slides"

    @pytest.mark.asyncio
    async def test_get_skills_default_mode(self, agent_client):
        with respx.mock:
            respx.get("http://agentgo:8080/skills", params={"mode": "deck"}).mock(
                return_value=Response(200, json={"skills": []})
            )
            skills = await agent_client.get_skills()
            assert skills == []

    @pytest.mark.asyncio
    async def test_recommend_styles(self, agent_client):
        with respx.mock:
            respx.post("http://agentgo:8080/recommend-styles").mock(
                return_value=Response(200, json={"styles": [
                    {"name": "Minimal Dark", "colors": ["#111"]},
                ]})
            )
            result = await agent_client.recommend_styles("dark theme", 2)
            assert len(result["styles"]) == 1


class TestBuildProviderPayload:
    """Tests for AgentClient.build_provider_payload() — maps ProviderConfigRequest to AgentGo dict."""

    def test_maps_all_fields(self, agent_client):
        from src.models.schemas import ProviderConfigRequest

        req = ProviderConfigRequest(
            provider="openai",
            api_key="sk-test-key",
            base_url="https://api.openai.com/v1",
            model_name="gpt-4o",
        )
        payload = agent_client.build_provider_payload(req)
        assert payload == {
            "provider_type": "openai",
            "api_key": "sk-test-key",
            "base_url": "https://api.openai.com/v1",
            "model": "gpt-4o",
        }

    def test_empty_strings_pass_through(self, agent_client):
        from src.models.schemas import ProviderConfigRequest

        req = ProviderConfigRequest(
            provider="",
            api_key="",
            base_url="",
            model_name="",
        )
        payload = agent_client.build_provider_payload(req)
        assert payload["provider_type"] == ""
        assert payload["api_key"] == ""
        assert payload["base_url"] == ""
        assert payload["model"] == ""

    def test_custom_provider(self, agent_client):
        from src.models.schemas import ProviderConfigRequest

        req = ProviderConfigRequest(
            provider="deepseek",
            api_key="sk-deepseek",
            base_url="https://api.deepseek.com",
            model_name="deepseek-chat",
        )
        payload = agent_client.build_provider_payload(req)
        assert payload["provider_type"] == "deepseek"
        assert payload["model"] == "deepseek-chat"
