import copy

from fastapi import HTTPException

from src.constants.error_codes import ErrorCode
from src.models.schemas import ProjectResponse, VersionRestoreResponse
from src.repositories.file_repo import FileRepo
from src.repositories.version_repo import VersionRepo, version_tag
from src.services.workspace import WorkspaceService
from src.utils.html_parser import count_slides


class ProjectService:
    """Aggregate dynamic project info from repos."""

    def __init__(
        self,
        file_repo: FileRepo,
        version_repo: VersionRepo,
        workspace: WorkspaceService,
    ):
        self._file_repo = file_repo
        self._version_repo = version_repo
        self._workspace = workspace

    async def has_project(self, project_id: str) -> bool:
        """Check whether a project exists (project.json is present)."""
        meta = await self._workspace.read_meta(project_id)
        return bool(meta)

    async def get_project_info(self, project_id: str) -> ProjectResponse:
        """Aggregate project dynamic fields."""
        meta = await self._workspace.read_meta(project_id)
        current_version = await self._version_repo.get_latest_version_id()

        if await self._file_repo.html_exists():
            html = await self._file_repo.read_html()
            file_size_bytes = await self._file_repo.html_size()
            slide_count = count_slides(html)
        else:
            file_size_bytes = 0
            slide_count = 0

        return ProjectResponse(
            id=meta.get("id", project_id),
            name=meta.get("name", project_id),
            current_version=current_version,
            file_size_bytes=file_size_bytes,
            slide_count=slide_count,
            has_html=await self._file_repo.html_exists(),
            brief=meta.get("brief", ""),
            design_skill=meta.get("design_skill", ""),
            created_at=meta.get("created_at", ""),
        )

    async def restore_version(
        self, project_id: str, version_id: str
    ) -> VersionRestoreResponse:
        """Restore a version: create rollback snapshot and write HTML back.

        Must be called inside a project-level lock to prevent concurrent writes.
        """
        try:
            target = await self._version_repo.get_version(version_id)
        except FileNotFoundError:
            raise HTTPException(
                status_code=404,
                detail={"code": ErrorCode.VERSION_NOT_FOUND.value, "message": f"Version not found: {version_id}"},
            )

        manifest_backup = copy.deepcopy(await self._version_repo.read_manifest())

        target_html_file = (
            target.snapshot.get("design_snapshot", {}).get("active_file")
        )
        if not target_html_file:
            vdir = self._version_repo.version_dir(version_id)
            html_files = list(vdir.glob("*.html"))
            target_html_file = html_files[0].name if html_files else "index.html"

        new_vid = await self._version_repo.create_version(
            title=f"回退到 {target.tag}",
            html_content=target.html,
            session_id="manual-rollback",
            html_file=target_html_file,
        )
        try:
            active = await self._file_repo.resolve_active_file()
            if active is None:
                await self._file_repo.write_new_file(target_html_file, target.html)
            else:
                await self._file_repo.write_html(target.html)
        except Exception:
            await self._version_repo.write_manifest(manifest_backup)
            self._version_repo.delete_version(new_vid)
            raise HTTPException(
                status_code=500,
                detail={"code": ErrorCode.FILE_WRITE_ERROR.value, "message": "写入活跃文件失败，回退操作已取消"},
            )

        new_tag = version_tag(new_vid)
        return VersionRestoreResponse(
            project_id=project_id,
            restored_to=version_id,
            new_version_id=new_vid,
            new_version_tag=new_tag,
            html=target.html,
            css=target.css,
        )
