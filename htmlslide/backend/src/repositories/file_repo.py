import json
from pathlib import Path

import aiofiles

from src.utils.atomic import atomic_write


class FileRepo:
    """File system repository — operates on a project's isolated directory.

    Active file tracking: active.json at the project root points to the
    currently active HTML file. resolve_active_file() is the single entry
    point for all active-file queries. sync_active_file() force-scans the
    filesystem and updates active.json.
    """

    def __init__(self, workspace_root: Path, project_id: str):
        self._root = (workspace_root / "projects" / project_id).resolve()
        self._active_json_path = self._root / "active.json"

    # ── Single resolution entry point ─────────────────────────────────

    async def resolve_active_file(self) -> str | None:
        """Return the active HTML filename, or None if no HTML exists.

        Resolution order:
        1. Read active.json. If the referenced file passes validation and
           exists on disk, use it.
        2. Scan project root for *.html, pick the most recently modified.
           If found, repair active.json and return it.
        3. Return None — the project has no HTML content.
        """
        # Step 1: Try active.json (cache)
        if self._active_json_path.is_file():
            async with aiofiles.open(self._active_json_path, "r") as f:
                info = json.loads(await f.read())
            active = info.get("active")
            if active and self._validate_filename(active):
                if (self._root / active).is_file():
                    return active

        # Step 2: Filesystem scan (ground truth)
        html_files = sorted(
            self._root.glob("*.html"),
            key=lambda f: f.stat().st_mtime,
            reverse=True,
        )
        if html_files:
            active = html_files[0].name
            # Repair stale cache
            async with aiofiles.open(self._active_json_path, "w") as f:
                await f.write(json.dumps({"active": active}, ensure_ascii=False))
            return active

        # Step 3: Empty project
        return None

    @staticmethod
    def _validate_filename(filename: str) -> bool:
        """Reject path traversal in filenames."""
        if not filename or ".." in filename or "/" in filename or "\\" in filename:
            return False
        return True

    # ── HTML operations ──────────────────────────────────────────────

    async def read_html(self) -> str:
        """Read the currently active HTML file."""
        active = await self.resolve_active_file()
        if active is None:
            raise FileNotFoundError("No HTML file in project")
        path = self._root / active
        async with aiofiles.open(path, "r", encoding="utf-8") as f:
            return await f.read()

    async def write_html(self, html: str) -> str:
        """Write to the currently active HTML file. Raises if no active file."""
        active = await self.resolve_active_file()
        if active is None:
            raise FileNotFoundError("No active HTML file in project")
        path = self._root / active
        path.parent.mkdir(parents=True, exist_ok=True)
        await atomic_write(str(path), html)
        return active

    async def html_exists(self) -> bool:
        active = await self.resolve_active_file()
        return active is not None

    async def html_size(self) -> int:
        active = await self.resolve_active_file()
        if active is None:
            return 0
        path = self._root / active
        return path.stat().st_size if path.is_file() else 0

    def file_path(self, filename: str) -> Path:
        """Return the absolute path for a filename in the project root."""
        if not self._validate_filename(filename):
            raise ValueError(f"Invalid filename: {filename}")
        return self._root / filename

    async def write_new_file(self, filename: str, content: str) -> str:
        """Write a new HTML file and sync active.json. For first-write scenarios."""
        path = self._root / filename
        path.parent.mkdir(parents=True, exist_ok=True)
        await atomic_write(str(path), content)
        await self.sync_active_file()
        return filename

    # ── Active file management ───────────────────────────────────────

    async def read_active_json(self) -> dict:
        """Read active.json. Returns empty dict if missing (no fallback)."""
        if not self._active_json_path.is_file():
            return {}
        async with aiofiles.open(self._active_json_path, "r") as f:
            return json.loads(await f.read())

    async def get_active_html(self) -> dict:
        """Return {filename, content} for the active file, or nulls if empty."""
        active = await self.resolve_active_file()
        if active is None:
            return {"filename": None, "content": None}
        path = self._root / active
        async with aiofiles.open(path, "r", encoding="utf-8") as f:
            return {"filename": active, "content": await f.read()}

    async def sync_active_file(self) -> str | None:
        """Force-scan project root for .html files, update active.json.

        Unlike resolve_active_file(), this always scans the filesystem
        rather than trusting the cache — used after Agent operations.
        """
        html_files = sorted(
            self._root.glob("*.html"),
            key=lambda f: f.stat().st_mtime,
            reverse=True,
        )
        if not html_files:
            return None
        active = html_files[0].name
        async with aiofiles.open(self._active_json_path, "w") as f:
            await f.write(json.dumps({"active": active}, ensure_ascii=False))
        return active

    async def list_html_files(self) -> list[dict]:
        """Return all .html files in the project root with names and mtimes."""
        files = []
        for p in sorted(self._root.glob("*.html"), key=lambda f: f.stat().st_mtime, reverse=True):
            files.append({
                "filename": p.name,
                "size_bytes": p.stat().st_size,
                "mtime": p.stat().st_mtime,
            })
        return files
