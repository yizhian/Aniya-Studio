import httpx
from fastapi import APIRouter

from src.api.deps import get_agent_client
from src.models.schemas import (
    ProviderConfigRequest,
    ProviderModelsResponse,
    ProviderSyncResponse,
    ProviderTestResponse,
)

router = APIRouter()


@router.post("/settings/sync", response_model=ProviderSyncResponse)
async def sync_settings(req: ProviderConfigRequest):
    """Push provider config to AgentGo runtime."""
    agent_client = get_agent_client()
    try:
        data = await agent_client.sync_provider_config(agent_client.build_provider_payload(req))
        return {"ok": True, "message": data.get("message", "Synced to AgentGo")}
    except httpx.HTTPStatusError as exc:
        detail = exc.response.text
        return {"ok": False, "message": detail or f"AgentGo error: HTTP {exc.response.status_code}"}
    except Exception as exc:
        return {"ok": False, "message": str(exc)}


@router.post("/settings/test-connection", response_model=ProviderTestResponse)
async def test_connection(req: ProviderConfigRequest):
    """Probe API key, base URL, and model name via AgentGo."""
    agent_client = get_agent_client()
    try:
        data = await agent_client.test_provider_config(agent_client.build_provider_payload(req))
        return {
            "ok": data.get("ok", False),
            "message": data.get("message", "Unknown response"),
            "in_list": data.get("in_list", False),
            "verified": data.get("verified", False),
        }
    except httpx.HTTPStatusError as exc:
        return {"ok": False, "message": exc.response.text or f"HTTP {exc.response.status_code}"}
    except Exception as exc:
        return {"ok": False, "message": str(exc)}


@router.post("/settings/models", response_model=ProviderModelsResponse)
async def list_models(req: ProviderConfigRequest):
    """Fetch available model IDs from the provider's /v1/models endpoint."""
    agent_client = get_agent_client()
    try:
        data = await agent_client.list_provider_models(agent_client.build_provider_payload(req))
        return {
            "models": data.get("models", []),
            "source": data.get("source", "none"),
            "error": data.get("error"),
        }
    except httpx.HTTPStatusError as exc:
        return {"models": [], "source": "none", "error": exc.response.text}
    except Exception as exc:
        return {"models": [], "source": "none", "error": str(exc)}
