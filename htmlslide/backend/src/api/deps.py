"""FastAPI dependency injection — factory functions keyed by project_id."""

import asyncio

from fastapi import HTTPException

from src.config import settings
from src.constants.error_codes import ErrorCode
from src.repositories.file_repo import FileRepo
from src.repositories.version_repo import VersionRepo
from src.services.agent_client import AgentClient
from src.services.project_service import ProjectService
from src.services.workspace import WorkspaceService

# ── Caches keyed by project_id ──────────────────────────────────────

_file_repo_cache: dict[str, FileRepo] = {}
_version_repo_cache: dict[str, VersionRepo] = {}
_project_service_cache: dict[str, ProjectService] = {}
_project_locks: dict[str, asyncio.Lock] = {}


def clear_project_caches(project_id: str) -> None:
    """Remove all cached objects for a deleted project."""
    _file_repo_cache.pop(project_id, None)
    _version_repo_cache.pop(project_id, None)
    _project_service_cache.pop(project_id, None)
    _project_locks.pop(project_id, None)


def get_project_lock(project_id: str) -> asyncio.Lock:
    """Return a per-project asyncio.Lock for serializing writes.

    Serializes requests within a single uvicorn process. Cross-process
    (multi-worker or agentgo) concurrent manifest writes are not fully
    protected — both sides use atomic_write so individual files won't
    corrupt, but read-modify-write on manifest.json can still lost-update.
    """
    if project_id not in _project_locks:
        _project_locks[project_id] = asyncio.Lock()
    return _project_locks[project_id]


def validate_project_id(project_id: str) -> str:
    """Reject path traversal characters in project_id. Returns the id on success."""
    if ".." in project_id or "/" in project_id or "\\" in project_id:
        raise HTTPException(
            status_code=422,
            detail={
                "code": ErrorCode.VALIDATION_ERROR.value,
                "message": f"Invalid project_id: {project_id}",
            },
        )
    if not project_id.strip():
        raise HTTPException(
            status_code=422,
            detail={
                "code": ErrorCode.VALIDATION_ERROR.value,
                "message": "project_id must not be empty",
            },
        )
    return project_id


async def require_project(project_id: str) -> dict:
    """Validate project_id and verify project exists. Returns meta dict or raises 404."""
    validate_project_id(project_id)
    workspace = get_workspace()
    meta = await workspace.read_meta(project_id)
    if not meta:
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.PROJECT_NOT_FOUND.value, "message": "Project not found"},
        )
    return meta


async def require_html(file_repo: FileRepo, message: str = "Project has no HTML content") -> str:
    """Verify project has HTML and return it. Raises 404 if not."""
    if not await file_repo.html_exists():
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.FILE_READ_ERROR.value, "message": message},
        )
    return await file_repo.read_html()


# ── Singleton (no project_id needed) ────────────────────────────────

_workspace_service: WorkspaceService | None = None
_agent_client: AgentClient | None = None


def get_workspace() -> WorkspaceService:
    global _workspace_service
    if _workspace_service is None:
        _workspace_service = WorkspaceService(
            settings.workspace_path,
            skills_source=settings.agentgo_skills_path,
        )
    return _workspace_service


def get_agent_client() -> AgentClient:
    global _agent_client
    if _agent_client is None:
        _agent_client = AgentClient()
    return _agent_client


# ── Per-project factories ───────────────────────────────────────────

def get_file_repo(project_id: str) -> FileRepo:
    if project_id not in _file_repo_cache:
        _file_repo_cache[project_id] = FileRepo(settings.workspace_path, project_id)
    return _file_repo_cache[project_id]


def get_version_repo(project_id: str) -> VersionRepo:
    if project_id not in _version_repo_cache:
        _version_repo_cache[project_id] = VersionRepo(settings.workspace_path, project_id)
    return _version_repo_cache[project_id]


def get_project_service(project_id: str) -> ProjectService:
    if project_id not in _project_service_cache:
        _project_service_cache[project_id] = ProjectService(
            file_repo=get_file_repo(project_id),
            version_repo=get_version_repo(project_id),
            workspace=get_workspace(),
        )
    return _project_service_cache[project_id]
