from fastapi import APIRouter
from fastapi.responses import StreamingResponse

from src.api.deps import (
    get_agent_client,
    get_file_repo,
    get_version_repo,
    get_workspace,
    require_project,
    validate_project_id,
)
from src.models.schemas import ChatHistoryResponse, ChatRequest, RecommendRequest
from src.utils.chat_utils import format_chat_message, messages_to_timeline


router = APIRouter()


@router.post("/chat")
async def chat(body: ChatRequest):
    project_id = validate_project_id(body.project_id)
    await require_project(project_id)

    # Build message and session ID.
    message = format_chat_message(body.prompt, body.selected_dom)
    session_id = project_id

    # Resolve active file.
    file_repo = get_file_repo(project_id)
    active_file = await file_repo.resolve_active_file() or ""

    # Determine workspace path.
    workspace_path = str(get_workspace().project_dir(project_id))

    # Get AgentGo SSE stream.
    agent_client = get_agent_client()
    attachments = [a.model_dump(exclude_none=True) for a in body.attachments]

    agentgo_stream = agent_client.chat_stream(
        message=message,
        session_id=session_id,
        workspace_path=workspace_path,
        active_file=active_file,
        dom_context=body.selected_dom.model_dump() if body.selected_dom else None,
        attachments=attachments,
    )

    # Bridge to frontend format.
    from src.services.sse_bridge import bridge_sse

    version_repo = get_version_repo(project_id)
    bridged = bridge_sse(
        agentgo_stream=agentgo_stream,
        project_id=project_id,
        version_repo=version_repo,
    )

    async def wrapped():
        async for event in bridged:
            yield event
        await file_repo.sync_active_file()

    return StreamingResponse(wrapped(), media_type="text/event-stream")


@router.post("/recommend-styles")
async def recommend_styles(body: RecommendRequest):
    agent_client = get_agent_client()
    return await agent_client.recommend_styles(body.brief, body.limit or 3)


@router.get("/projects/{project_id}/chat-history")
async def get_chat_history(project_id: str):
    project_id = validate_project_id(project_id)
    await require_project(project_id)

    agent_client = get_agent_client()
    session = await agent_client.get_session(project_id)
    if not session:
        return ChatHistoryResponse(project_id=project_id, entries=[])

    entries = session.get("timeline", [])
    if not entries:
        # Fallback: convert old-format messages for sessions saved before
        # the unified timeline was introduced.
        entries = messages_to_timeline(session)
    return ChatHistoryResponse(project_id=project_id, entries=entries)
