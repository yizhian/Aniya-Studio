import mimetypes

import magic
from fastapi import APIRouter, File, Form, HTTPException, UploadFile
from fastapi.responses import FileResponse, HTMLResponse

from src.api.deps import get_file_repo, get_workspace, require_html, require_project, validate_project_id
from src.config import settings
from src.constants.error_codes import ErrorCode
from src.models.schemas import FileUploadResponse, ProjectFilesResponse, SyncActiveResponse
from src.utils.html_parser import (
    count_slides,
    extract_style_content,
    has_slide_elements,
    strip_style_tags,
)

# Ensure .svg files are served with correct MIME (mimetypes may not register them on all platforms)
mimetypes.add_type("image/svg+xml", ".svg")

router = APIRouter()


def _name_from_file(filename: str) -> str:
    """Extract project name from filename (strip extension). Defaults on empty."""
    if not filename:
        return "未命名项目"
    name = filename.rsplit(".", 1)[0]
    return name if name else "未命名项目"


def _validate_upload(file: UploadFile, content: bytes) -> None:
    """Validate MIME type, extension, size, and non-empty."""
    if not file.filename:
        raise HTTPException(
            status_code=422,
            detail={"code": ErrorCode.EMPTY_FILE.value, "message": "Empty file"},
        )

    # Extension check
    if not file.filename.lower().endswith((".html", ".htm")):
        raise HTTPException(
            status_code=400,
            detail={"code": ErrorCode.UNSUPPORTED_FORMAT.value, "message": "仅支持 .html/.htm 文件"},
        )

    # Size check
    if len(content) > settings.upload_max_size:
        raise HTTPException(
            status_code=413,
            detail={"code": ErrorCode.FILE_TOO_LARGE.value, "message": f"文件过大，最大 {settings.upload_max_size // (1024 * 1024)}MB"},
        )

    # Empty check
    if len(content.strip()) == 0:
        raise HTTPException(
            status_code=422,
            detail={"code": ErrorCode.EMPTY_FILE.value, "message": "Empty file"},
        )

    # MIME check
    mime = magic.from_buffer(content[:1024], mime=True)
    if mime not in ("text/html", "text/plain"):
        raise HTTPException(
            status_code=400,
            detail={"code": ErrorCode.UNSUPPORTED_FORMAT.value, "message": "文件内容不是 HTML 格式"},
        )


@router.post("/files/upload", response_model=FileUploadResponse)
async def upload_file(
    file: UploadFile = File(...),
    project_id: str = Form(""),
    project_name: str = Form(""),
):
    content = await file.read()

    # 1. Validate
    _validate_upload(file, content)

    workspace = get_workspace()

    # 2. Create or verify project
    if not project_id:
        proj = await workspace.create_project(
            name=project_name or _name_from_file(file.filename)
        )
        project_id = proj["id"]
    else:
        validate_project_id(project_id)
        await require_project(project_id)

    # 3. Save to active file
    content_str = content.decode("utf-8")
    file_repo = get_file_repo(project_id)

    active = await file_repo.resolve_active_file()
    if active is None:
        filename = file.filename or "untitled.html"
        await file_repo.write_new_file(filename, content_str)
    else:
        await file_repo.write_html(content_str)

    # 4. Parse in-memory (no disk read-back)
    css = extract_style_content(content_str)
    body_html = strip_style_tags(content_str)
    slide_count = count_slides(content_str)

    return FileUploadResponse(
        project_id=project_id,
        file_name=file.filename or "untitled.html",
        file_size_bytes=len(content),
        html=body_html,
        css=css,
        slide_count=slide_count,
        is_deck=has_slide_elements(content_str),
    )


@router.get("/projects/{project_id}/active-file")
async def get_active_file(project_id: str):
    """Return the currently active HTML file content, or nulls if empty."""
    validate_project_id(project_id)
    await require_project(project_id)
    file_repo = get_file_repo(project_id)
    return await file_repo.get_active_html()


@router.get("/projects/{project_id}/files", response_model=ProjectFilesResponse)
async def list_project_files(project_id: str, type: str = "html"):
    """List files in the project directory. type=html returns only .html files."""
    validate_project_id(project_id)
    file_repo = get_file_repo(project_id)
    if type == "html":
        active_info = await file_repo.read_active_json()
        files = await file_repo.list_html_files()
        return {
            "project_id": project_id,
            "active": active_info.get("active"),
            "files": files,
        }
    return {"project_id": project_id, "files": []}


@router.post("/projects/{project_id}/sync-active", response_model=SyncActiveResponse)
async def sync_active_file(project_id: str):
    """Sync active.json to the most recently modified .html file."""
    validate_project_id(project_id)
    file_repo = get_file_repo(project_id)
    active = await file_repo.sync_active_file()
    return {"project_id": project_id, "active": active}


@router.get("/projects/{project_id}/preview", response_class=HTMLResponse)
async def get_project_preview(project_id: str):
    """Return project HTML for iframe thumbnail preview."""
    validate_project_id(project_id)
    await require_project(project_id)
    file_repo = get_file_repo(project_id)
    html = await require_html(file_repo, "项目无 HTML 内容")
    return HTMLResponse(content=html, media_type="text/html")


@router.get("/projects/{project_id}/uploads/{file_path:path}")
async def serve_uploaded_asset(project_id: str, file_path: str):
    """Serve an uploaded asset (image, etc.) from a project's uploads directory."""
    validate_project_id(project_id)
    await require_project(project_id)
    project_dir = get_workspace().project_dir(project_id)
    uploads_root = project_dir / "uploads"
    resolved = (uploads_root / file_path).resolve()

    if not resolved.is_relative_to(uploads_root.resolve()):
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.FILE_READ_ERROR.value, "message": "Asset not found"},
        )

    if not resolved.is_file():
        raise HTTPException(
            status_code=404,
            detail={"code": ErrorCode.FILE_READ_ERROR.value, "message": "Asset not found"},
        )

    mime, _ = mimetypes.guess_type(resolved.as_posix())
    if not mime:
        # Whitelist known image types; reject everything else
        ext = resolved.suffix.lower()
        mime_map = {
            ".svg": "image/svg+xml",
            ".png": "image/png",
            ".jpg": "image/jpeg",
            ".jpeg": "image/jpeg",
            ".gif": "image/gif",
            ".webp": "image/webp",
        }
        mime = mime_map.get(ext)
        if not mime:
            raise HTTPException(
                status_code=415,
                detail={"code": ErrorCode.UNSUPPORTED_FORMAT.value, "message": "不支持的资源类型"},
            )

    return FileResponse(resolved, media_type=mime)
