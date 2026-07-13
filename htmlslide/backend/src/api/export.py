import io
import logging
from typing import Awaitable, Callable
from urllib.parse import quote

from fastapi import APIRouter, HTTPException
from fastapi.responses import FileResponse, StreamingResponse

from src.api.deps import get_file_repo, get_version_repo, get_workspace, require_html, validate_project_id
from src.constants.error_codes import ErrorCode
from src.models.schemas import ExportResponse
from src.services.pdf_exporter import generate_pdf
from src.services.pptx_exporter import generate_pptx
from src.utils.html_parser import count_slides, extract_style_content

logger = logging.getLogger(__name__)
router = APIRouter()


@router.get("/projects/{project_id}/export", response_model=ExportResponse)
async def export_project(project_id: str):
    validate_project_id(project_id)
    file_repo = get_file_repo(project_id)
    version_repo = get_version_repo(project_id)

    html = await require_html(file_repo)
    latest_id = await version_repo.get_latest_version_id()

    return ExportResponse(
        project_id=project_id,
        version=latest_id or "unknown",
        html=html,
        css=extract_style_content(html),
        slide_count=count_slides(html),
        file_size_bytes=await file_repo.html_size(),
    )


@router.get("/projects/{project_id}/download")
async def download_project(project_id: str):
    validate_project_id(project_id)
    workspace = get_workspace()
    file_repo = get_file_repo(project_id)

    await require_html(file_repo)

    meta = await workspace.read_meta(project_id)
    safe_name = meta.get("name", project_id).replace(" ", "-")
    filename = f"{safe_name}.html"

    active_file = await file_repo.resolve_active_file()
    if active_file is None:
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.FILE_READ_ERROR.value, "message": "Project has no HTML content"},
        )
    html_path = file_repo.file_path(active_file)

    return FileResponse(
        path=str(html_path),
        filename=filename,
        media_type="text/html; charset=utf-8",
    )


async def _streaming_binary_export(
    project_id: str,
    exporter: Callable[[str], Awaitable[bytes]],
    ext: str,
    media_type: str,
) -> StreamingResponse:
    """Shared implementation for PDF/PPTX streaming export endpoints."""
    validate_project_id(project_id)
    workspace = get_workspace()
    file_repo = get_file_repo(project_id)

    html = await require_html(file_repo)
    meta = await workspace.read_meta(project_id)
    safe_name = meta.get("name", project_id).replace(" ", "-")
    filename = f"{safe_name}.{ext}"

    result_bytes = await exporter(html)

    encoded = quote(filename, safe="")
    return StreamingResponse(
        io.BytesIO(result_bytes),
        media_type=media_type,
        headers={"Content-Disposition": f"attachment; filename*=UTF-8''{encoded}"},
    )


@router.get("/projects/{project_id}/export/pdf")
async def export_pdf(project_id: str):
    return await _streaming_binary_export(project_id, generate_pdf, "pdf", "application/pdf")


@router.get("/projects/{project_id}/export/pptx")
async def export_pptx(project_id: str):
    logger.info("PPTX 导出请求: project_id=%s", project_id)
    return await _streaming_binary_export(
        project_id, generate_pptx, "pptx",
        "application/vnd.openxmlformats-officedocument.presentationml.presentation",
    )
