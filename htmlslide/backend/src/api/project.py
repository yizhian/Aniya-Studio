import logging

from fastapi import APIRouter, HTTPException
from fastapi.responses import Response

from src.api.deps import clear_project_caches, get_project_service, get_workspace, validate_project_id
from src.constants.error_codes import ErrorCode
from src.models.schemas import CreateProjectRequest, ProjectResponse, UpdateProjectRequest

router = APIRouter()
logger = logging.getLogger(__name__)


@router.post("/projects", status_code=201, response_model=ProjectResponse)
async def create_project(body: CreateProjectRequest):
    workspace = get_workspace()
    meta = await workspace.create_project(body.name, body.brief, body.design_skill)
    project_id = meta["id"]
    project_service = get_project_service(project_id)
    return await project_service.get_project_info(project_id)


@router.patch("/projects/{project_id}", response_model=ProjectResponse)
async def update_project(project_id: str, body: UpdateProjectRequest):
    validate_project_id(project_id)
    workspace = get_workspace()
    meta = await workspace.update_project(
        project_id,
        name=body.name,
        brief=body.brief,
        design_skill=body.design_skill,
    )
    if not meta:
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.PROJECT_NOT_FOUND.value, "message": "Project not found"},
        )
    project_service = get_project_service(project_id)
    return await project_service.get_project_info(project_id)


@router.get("/projects", response_model=list[ProjectResponse])
async def list_projects():
    workspace = get_workspace()
    metas = await workspace.list_projects()
    result = []
    for meta in metas:
        pid = meta["id"]
        ps = get_project_service(pid)
        result.append(await ps.get_project_info(pid))
    return result


@router.delete("/projects/{project_id}", status_code=204)
async def delete_project(project_id: str):
    validate_project_id(project_id)
    workspace = get_workspace()
    deleted = await workspace.delete_project(project_id)
    if not deleted:
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.PROJECT_NOT_FOUND.value, "message": "Project not found"},
        )
    clear_project_caches(project_id)
    return Response(status_code=204)


@router.get("/projects/{project_id}", response_model=ProjectResponse)
async def get_project(project_id: str):
    validate_project_id(project_id)
    project_service = get_project_service(project_id)
    if not await project_service.has_project(project_id):
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.PROJECT_NOT_FOUND.value, "message": "Project not found"},
        )
    return await project_service.get_project_info(project_id)
