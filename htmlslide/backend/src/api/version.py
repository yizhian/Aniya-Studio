from fastapi import APIRouter, HTTPException

from src.api.deps import (
    get_file_repo,
    get_project_lock,
    get_project_service,
    get_version_repo,
    require_project,
    validate_project_id,
)
from src.constants.error_codes import ErrorCode
from src.models.schemas import (
    SaveVersionRequest,
    SaveVersionResponse,
    VersionDetailResponse,
    VersionListResponse,
    VersionRestoreResponse,
)

router = APIRouter()


@router.get("/projects/{project_id}/versions", response_model=VersionListResponse)
async def list_versions(project_id: str):
    validate_project_id(project_id)
    await require_project(project_id)
    version_repo = get_version_repo(project_id)
    versions = await version_repo.list_versions()
    current = next((v.id for v in versions if v.current), None)
    return VersionListResponse(
        project_id=project_id,
        current_version=current,
        versions=versions,
    )


@router.get("/projects/{project_id}/versions/{version_id}", response_model=VersionDetailResponse)
async def get_version_detail(project_id: str, version_id: str):
    validate_project_id(project_id)
    await require_project(project_id)
    version_repo = get_version_repo(project_id)
    try:
        return await version_repo.get_version(version_id)
    except FileNotFoundError:
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.VERSION_NOT_FOUND.value, "message": f"Version not found: {version_id}"},
        )


@router.post(
    "/projects/{project_id}/versions",
    response_model=SaveVersionResponse,
)
async def save_version(project_id: str, req: SaveVersionRequest):
    validate_project_id(project_id)
    lock = get_project_lock(project_id)
    version_repo = get_version_repo(project_id)
    file_repo = get_file_repo(project_id)

    if not await file_repo.html_exists():
        raise HTTPException(
            status_code=400,
            detail={"code": ErrorCode.FILE_READ_ERROR.value, "message": "项目无 HTML 内容，无法保存版本"},
        )

    async with lock:
        active = await file_repo.write_html(req.html)
        vid = await version_repo.create_version(
            title=req.title,
            html_content=req.html,
            session_id="manual",
            html_file=active,
        )

    detail = await version_repo.get_version(vid)
    return SaveVersionResponse(
        project_id=project_id,
        version_id=detail.id,
        version_tag=detail.tag,
        title=detail.title,
        html=detail.html,
        css=detail.css,
        created_at=detail.created_at,
    )


@router.post(
    "/projects/{project_id}/versions/{version_id}/restore",
    response_model=VersionRestoreResponse,
)
async def restore_version(project_id: str, version_id: str):
    validate_project_id(project_id)
    lock = get_project_lock(project_id)
    async with lock:
        project_service = get_project_service(project_id)
        return await project_service.restore_version(project_id, version_id)
