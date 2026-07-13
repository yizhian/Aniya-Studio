import json
import logging
import shutil
import uuid
from datetime import datetime, timezone
from pathlib import Path

import aiofiles

logger = logging.getLogger(__name__)


class WorkspaceService:
    """Workspace lifecycle management.

    Each project gets an isolated directory under <workspace_root>/projects/{project_id}/.
    project_id doubles as AgentGo session_id.
    """

    def __init__(self, workspace_root: Path, skills_source: Path | None = None):
        self._root = workspace_root.resolve()
        self._skills_source = skills_source.resolve() if skills_source else None

    def generate_project_id(self) -> str:
        return f"proj-{uuid.uuid4().hex[:12]}"

    def _project_dir(self, project_id: str) -> Path:
        return self._root / "projects" / project_id

    def project_dir(self, project_id: str) -> Path:
        """Public accessor for the project directory."""
        return self._project_dir(project_id)

    async def create_project(
        self,
        name: str,
        brief: str | None = None,
        design_skill: str | None = None,
    ) -> dict:
        """Create a new project with directory tree only — no content files."""
        project_id = self.generate_project_id()
        project_dir = self._project_dir(project_id)

        # Create directory tree.
        project_dir.mkdir(parents=True, exist_ok=True)
        (project_dir / ".slidecraft" / "versions").mkdir(parents=True, exist_ok=True)

        # Mount .skills symlink (read-only reference to agentgo skill templates).
        # Target path exists in the agentgo container, not in this backend
        # container, so is_dir() is not meaningful. symlink_to() does not
        # require the target to exist at creation time.
        if self._skills_source:
            link_path = project_dir / ".skills"
            try:
                link_path.symlink_to(self._skills_source, target_is_directory=True)
                logger.info("skills mount: %s → %s", link_path, self._skills_source)
            except OSError as exc:
                logger.warning("skills mount failed (non-fatal): %s", exc)

        # Write project.json.
        meta = {
            "id": project_id,
            "name": name,
            "brief": brief or "",
            "created_at": datetime.now(timezone.utc).isoformat(),
        }
        if design_skill:
            meta["design_skill"] = design_skill
        async with aiofiles.open(project_dir / "project.json", "w") as f:
            await f.write(json.dumps(meta, ensure_ascii=False, indent=2))

        return meta

    async def read_meta(self, project_id: str) -> dict:
        """Read project.json for the given project_id."""
        path = self._project_dir(project_id) / "project.json"
        if not path.is_file():
            return {}
        async with aiofiles.open(path, "r") as f:
            return json.loads(await f.read())

    async def list_projects(self) -> list[dict]:
        """List all projects sorted by created_at descending."""
        projects_dir = self._root / "projects"
        if not projects_dir.is_dir():
            return []
        result = []
        for d in sorted(projects_dir.iterdir(), reverse=True):
            if not d.is_dir():
                continue
            meta = await self.read_meta(d.name)
            if meta:
                result.append(meta)
        result.sort(key=lambda m: m.get("created_at", ""), reverse=True)
        return result

    async def delete_project(self, project_id: str) -> bool:
        """Delete an entire project directory tree. Returns False if not found."""
        project_dir = self._project_dir(project_id)
        if not project_dir.is_dir():
            return False
        _async_rmtree(project_dir)
        return True

    async def update_project(
        self, project_id: str,
        name: str | None = None,
        brief: str | None = None,
        design_skill: str | None = None,
    ) -> dict | None:
        """Update project fields in project.json. Only non-None fields are updated."""
        meta = await self.read_meta(project_id)
        if not meta:
            return None
        if name is not None:
            meta["name"] = name
        if brief is not None:
            meta["brief"] = brief
        if design_skill is not None:
            meta["design_skill"] = design_skill
        meta["updated_at"] = datetime.now(timezone.utc).isoformat()
        path = self._project_dir(project_id) / "project.json"
        async with aiofiles.open(path, "w") as f:
            await f.write(json.dumps(meta, ensure_ascii=False, indent=2))
        return meta

    async def cleanup_drafts(self, max_age_hours: int = 24) -> int:
        """Delete projects older than max_age_hours that have no *.html files."""
        projects_dir = self._root / "projects"
        if not projects_dir.is_dir():
            return 0
        cutoff = datetime.now(timezone.utc).timestamp() - (max_age_hours * 3600)
        deleted = 0
        for d in projects_dir.iterdir():
            if not d.is_dir():
                continue
            try:
                if d.stat().st_mtime > cutoff:
                    continue
            except OSError:
                continue
            if list(d.glob("*.html")):
                continue
            if not (d / "project.json").is_file():
                continue
            logger.info("cleaning up draft project %s", d.name)
            _async_rmtree(d)
            deleted += 1
        return deleted


def _async_rmtree(path: Path) -> None:
    """Remove a directory tree synchronously (acceptably fast for FS deletes)."""
    if path.is_dir():
        shutil.rmtree(path)
