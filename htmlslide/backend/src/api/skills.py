from fastapi import APIRouter, HTTPException, Query
from fastapi.responses import Response, StreamingResponse

from src.api.deps import get_agent_client, require_project, validate_project_id
from src.constants.error_codes import ErrorCode
from src.models.schemas import (
    PrecipitateConfirmRequest,
    PrecipitatePreviewRequest,
    SkillContentResponse,
    SkillExampleResponse,
    SkillsListResponse,
)
from src.utils.html_parser import inject_base_tag

router = APIRouter()


@router.get("/skills", response_model=SkillsListResponse)
async def list_skills(mode: str = Query(default="deck", description="Filter by skill mode")):
    """List available design skills, optionally filtered by mode.

    Delegates to AgentGo's /skills endpoint which returns the full SkillIndex
    filtered by mode, with name, description, triggers, and scenario.
    """
    agent_client = get_agent_client()
    try:
        skills = await agent_client.get_skills(mode=mode)
        return {"skills": skills, "mode": mode}
    except Exception as exc:
        return {"skills": [], "mode": mode, "error": str(exc)}


@router.post("/skills/precipitate/stream")
async def precipitate_stream(req: PrecipitatePreviewRequest):
    """Generate skill from HTML via agent-driven SSE stream."""
    project_id = validate_project_id(req.project_id)
    await require_project(project_id)
    agent_client = get_agent_client()

    async def event_generator():
        async for chunk in agent_client.precipitate_stream({"html_content": req.html_content}):
            yield chunk

    return StreamingResponse(event_generator(), media_type="text/event-stream")


@router.post("/skills/precipitate/confirm")
async def precipitate_confirm(req: PrecipitateConfirmRequest):
    """Persist a precipitated skill to disk."""
    project_id = validate_project_id(req.project_id)
    await require_project(project_id)
    agent_client = get_agent_client()
    return await agent_client.precipitate_confirm(req.model_dump(exclude={"project_id"}))


@router.get("/skills/{name}/content", response_model=SkillContentResponse)
async def get_skill_content(name: str):
    """Return the raw SKILL.md content for a design skill."""
    agent_client = get_agent_client()
    status, content = await agent_client.get_skill_content(name)
    if status == 404:
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.FILE_READ_ERROR.value, "message": content},
        )
    return {"content": content}


@router.get("/skills/{name}/example", response_model=SkillExampleResponse)
async def get_skill_example(name: str):
    """Return the example.html content for a design skill."""
    agent_client = get_agent_client()
    status, content = await agent_client.get_skill_example(name)
    if status == 404:
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.FILE_READ_ERROR.value, "message": content},
        )
    return {"html": content}


async def _load_skill_preview_html(name: str) -> tuple[int, str]:
    agent_client = get_agent_client()
    status, content = await agent_client.get_skill_example(name)
    if status == 404:
        return status, content
    return 200, inject_base_tag(content, f"/api/v1/skills/{name}/")


@router.get("/skills/{name}/preview")
async def get_skill_preview(name: str):
    """Serve example.html directly as text/html with a <base> tag for asset resolution.

    This endpoint lets the browser load the preview HTML natively via iframe src,
    avoiding blob URL lifecycle issues and sandbox security warnings.
    """
    status, html = await _load_skill_preview_html(name)
    if status == 404:
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.FILE_READ_ERROR.value, "message": html},
        )
    return Response(content=html, media_type="text/html; charset=utf-8")


@router.head("/skills/{name}/preview")
async def head_skill_preview(name: str):
    """Existence check used by SkillPreviewPanel before loading the iframe."""
    status, _ = await _load_skill_preview_html(name)
    if status == 404:
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.FILE_READ_ERROR.value, "message": "preview not found"},
        )
    return Response(status_code=200)


@router.get("/skills/{name}/assets/{path:path}")
async def get_skill_asset(name: str, path: str):
    """Proxy a skill asset file from AgentGo."""
    agent_client = get_agent_client()
    status, content, content_type = await agent_client.get_skill_asset(name, path)
    if status == 404:
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.FILE_READ_ERROR.value, "message": f"Asset not found: {path}"},
        )
    return Response(content=content, media_type=content_type)
