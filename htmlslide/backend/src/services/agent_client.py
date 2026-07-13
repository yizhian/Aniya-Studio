from typing import AsyncIterator

import httpx

from src.config import settings


class AgentClient:
    """AgentGo HTTP client.

    POST /chat (SSE stream) — initiate agent conversation.
    """

    def __init__(self, base_url: str | None = None, timeout: float | None = None):
        self._client = httpx.AsyncClient(
            base_url=base_url or settings.agent_url,
            timeout=httpx.Timeout(timeout or settings.agent_timeout, connect=10.0),
        )

    async def chat_stream(
        self,
        message: str,
        session_id: str,
        workspace_path: str = "",
        active_file: str = "",
        dom_context: dict | None = None,
        attachments: list[dict] | None = None,
    ) -> AsyncIterator[bytes]:
        """Start agent conversation, returns AgentGo raw SSE byte stream."""
        body: dict = {
            "message": message,
            "session_id": session_id,
            "workspace_path": workspace_path,
            "active_file": active_file,
        }
        if dom_context:
            body["dom_context"] = dom_context
        if attachments:
            body["attachments"] = attachments

        async with self._client.stream("POST", "/chat", json=body) as response:
            response.raise_for_status()
            async for line in response.aiter_lines():
                yield line.encode("utf-8") if isinstance(line, str) else line

    async def get_session(self, session_id: str) -> dict | None:
        """Fetch session state from AgentGo."""
        resp = await self._client.get(f"/sessions/{session_id}")
        if resp.status_code == 404:
            return None
        resp.raise_for_status()
        return resp.json()

    async def recommend_styles(self, brief: str, limit: int = 3) -> dict:
        """Get style recommendations from AgentGo Ranker."""
        resp = await self._client.post("/recommend-styles", json={
            "brief": brief, "limit": limit,
        })
        resp.raise_for_status()
        return resp.json()

    async def get_skills(self, mode: str = "deck") -> list[dict]:
        """Fetch available skills from AgentGo, optionally filtered by mode."""
        resp = await self._client.get("/skills", params={"mode": mode})
        resp.raise_for_status()
        data = resp.json()
        return data.get("skills", [])

    async def precipitate_stream(self, payload: dict) -> AsyncIterator[bytes]:
        """Generate skill from HTML via agent-driven SSE stream."""
        async with self._client.stream("POST", "/skills/precipitate/stream", json=payload) as resp:
            resp.raise_for_status()
            async for chunk in resp.aiter_bytes():
                yield chunk

    async def precipitate_confirm(self, payload: dict) -> dict:
        """Persist a precipitated skill to disk."""
        resp = await self._client.post("/skills/precipitate/confirm", json=payload)
        resp.raise_for_status()
        return resp.json()

    async def get_skill_example(self, name: str) -> tuple[int, str]:
        """Fetch example.html for a skill. Returns (status_code, html_or_error)."""
        resp = await self._client.get(f"/skills/{name}/example")
        if resp.status_code == 404:
            return (404, f"No example for skill: {name}")
        resp.raise_for_status()
        return (200, resp.text)

    async def get_skill_content(self, name: str) -> tuple[int, str]:
        """Fetch SKILL.md for a skill. Returns (status_code, content_or_error)."""
        resp = await self._client.get(f"/skills/{name}/content")
        if resp.status_code == 404:
            return (404, f"No content for skill: {name}")
        resp.raise_for_status()
        data = resp.json()
        return (200, data.get("content", ""))

    async def get_skill_asset(self, name: str, path: str) -> tuple[int, bytes, str]:
        """Fetch an asset file for a skill. Returns (status_code, content_bytes, content_type)."""
        resp = await self._client.get(f"/skills/{name}/assets/{path}")
        if resp.status_code == 404:
            return (404, b"", "")
        resp.raise_for_status()
        return (200, resp.content, resp.headers.get("content-type", "application/octet-stream"))

    async def sync_provider_config(self, payload: dict) -> dict:
        """Push runtime provider config to AgentGo."""
        resp = await self._client.post("/provider/config", json=payload)
        resp.raise_for_status()
        return resp.json()

    async def test_provider_config(self, payload: dict) -> dict:
        """Probe provider credentials and model name via AgentGo."""
        resp = await self._client.post("/provider/test", json=payload)
        resp.raise_for_status()
        return resp.json()

    async def _parse_json_response(self, resp: httpx.Response) -> dict:
        """Parse JSON body; fall back to plain-text error for non-JSON responses."""
        try:
            data = resp.json()
            if isinstance(data, dict):
                return data
        except Exception:
            pass
        text = (resp.text or "").strip()
        if resp.status_code == 404 and "not found" in text.lower():
            return {
                "models": [],
                "source": "none",
                "error": (
                    "AgentGo 未提供 /provider/models 接口，请重启或重新构建 "
                    "agentgo 服务后再试（docker compose up --build）"
                ),
            }
        return {
            "models": [],
            "source": "none",
            "error": text or f"HTTP {resp.status_code}",
        }

    async def list_provider_models(self, payload: dict) -> dict:
        """List models from provider via AgentGo."""
        resp = await self._client.post("/provider/models", json=payload)
        if resp.status_code >= 400:
            return await self._parse_json_response(resp)
        resp.raise_for_status()
        return await self._parse_json_response(resp)

    async def close(self):
        await self._client.aclose()

    def build_provider_payload(self, req) -> dict:
        """Map ProviderConfigRequest to AgentGo provider config payload."""
        return {
            "provider_type": req.provider,
            "api_key": req.api_key,
            "base_url": req.base_url,
            "model": req.model_name,
        }
