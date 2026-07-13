import json
import shutil
from datetime import datetime, timezone
from pathlib import Path

import aiofiles

from src.models.schemas import VersionDetailResponse, VersionItem
from src.utils.atomic import atomic_write
from src.utils.html_parser import count_slides, extract_style_content, extract_title_from_html


def version_tag(version_id: str) -> str:
    """Convert "v005" to "V5" (strip leading zeros)."""
    num_part = version_id.lstrip("vV")
    return f"V{int(num_part)}"


class VersionRepo:
    """Version snapshot repository — read-only access to <project>/.slidecraft/versions/."""

    def __init__(self, workspace_root: Path, project_id: str):
        self._root = (workspace_root / "projects" / project_id).resolve()

    def _versions_dir(self) -> Path:
        return self._root / ".slidecraft" / "versions"

    def version_dir(self, version_id: str) -> Path:
        """Return the directory path for a specific version."""
        return self._versions_dir() / version_id

    def delete_version(self, version_id: str) -> None:
        """Remove a version directory. No-op if it doesn't exist."""
        shutil.rmtree(self.version_dir(version_id), ignore_errors=True)

    async def read_manifest(self) -> dict:
        path = self._root / ".slidecraft" / "manifest.json"
        if not path.is_file():
            return {"current_version": 0, "versions": []}
        async with aiofiles.open(path, "r") as f:
            return json.loads(await f.read())

    async def get_latest_version_id(self) -> str | None:
        versions_dir = self._versions_dir()
        if not versions_dir.is_dir():
            return None
        entries = sorted(
            [d for d in versions_dir.iterdir() if d.is_dir()],
            key=lambda d: d.stat().st_mtime,
            reverse=True,
        )
        return entries[0].name if entries else None

    async def list_versions(self) -> list[VersionItem]:
        versions_dir = self._versions_dir()
        if not versions_dir.is_dir():
            return []

        latest_id = await self.get_latest_version_id()
        entries = sorted(versions_dir.iterdir(), reverse=True)

        result = []
        for v_dir in entries:
            if not v_dir.is_dir():
                continue
            ctx = await self._read_context_json(v_dir)

            # HTML file name is recorded in manifest or context.json.
            html_files = list(v_dir.glob("*.html"))
            html_path = html_files[0] if html_files else (v_dir / "index.html")

            result.append(VersionItem(
                id=v_dir.name,
                tag=version_tag(v_dir.name),
                title=ctx.get("title", v_dir.name),
                slide_count=ctx.get("slide_count", 0),
                file_size_bytes=html_path.stat().st_size if html_path.is_file() else 0,
                created_at=datetime.fromtimestamp(v_dir.stat().st_mtime),
                current=(v_dir.name == latest_id),
            ))

        return result

    async def get_version(self, version_id: str) -> VersionDetailResponse:
        v_dir = self._versions_dir() / version_id
        if not v_dir.is_dir():
            raise FileNotFoundError(f"Version not found: {version_id}")

        ctx = await self._read_context_json(v_dir)

        html_files = list(v_dir.glob("*.html"))
        html_path = html_files[0] if html_files else (v_dir / "index.html")
        html = await self._read_file(html_path)
        css = extract_style_content(html)

        return VersionDetailResponse(
            id=version_id,
            tag=version_tag(version_id),
            title=ctx.get("title", version_id),
            html=html,
            css=css,
            snapshot=ctx,
            created_at=datetime.fromtimestamp(v_dir.stat().st_mtime),
        )

    async def _read_context_json(self, v_dir: Path) -> dict:
        path = v_dir / "context.json"
        if not path.is_file():
            return {}
        async with aiofiles.open(path, "r") as f:
            return json.loads(await f.read())

    async def _read_file(self, path: Path) -> str:
        async with aiofiles.open(path, "r", encoding="utf-8") as f:
            return await f.read()

    # ── Write operations ────────────────────────────────────────────

    async def _next_version_id(self) -> str:
        """Scan versions/ dirs + manifest current_version, return next ID."""
        max_dir = 0
        vdir = self._versions_dir()
        if vdir.is_dir():
            for d in vdir.iterdir():
                if d.is_dir() and d.name.startswith("v"):
                    try:
                        n = int(d.name.lstrip("v"))
                        if n > max_dir:
                            max_dir = n
                    except ValueError:
                        continue
        manifest = await self.read_manifest()
        manifest_cv = manifest.get("current_version", 0)
        return f"v{max(max_dir, manifest_cv) + 1:03d}"

    async def create_version(
        self,
        title: str,
        html_content: str,
        session_id: str = "manual",
        html_file: str = "index.html",
    ) -> str:
        vid = await self._next_version_id()
        vdir = self._versions_dir() / vid

        try:
            vdir.mkdir(parents=True, exist_ok=False)

            html_path = vdir / html_file
            await atomic_write(str(html_path), html_content)

            slide_count = count_slides(html_content)
            title_text = extract_title_from_html(html_content) or title
            now_iso = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
            ctx = {
                "version": int(vid.lstrip("v")),
                "created_at": now_iso,
                "session_id": session_id,
                "html_path": str(self._root / html_file),
                "title": title,
                "design_snapshot": {
                    "title": title_text,
                    "slide_count": slide_count,
                    "active_file": html_file,
                    "total_size_bytes": len(html_content.encode("utf-8")),
                    "theme": "",
                    "slide_headings": [],
                    "css_classes_used": [],
                    "fonts": [],
                    "color_palette": {},
                    "html_sections": [],
                },
                "todos_at_version": [],
            }
            await atomic_write(
                str(vdir / "context.json"),
                json.dumps(ctx, ensure_ascii=False, indent=2),
            )

            manifest = await self.read_manifest()
            manifest["current_version"] = int(vid.lstrip("v"))
            manifest["html_file"] = manifest.get("html_file", html_file)
            versions = manifest.setdefault("versions", [])
            versions.append({
                "number": int(vid.lstrip("v")),
                "created_at": now_iso,
                "html_file": html_file,
            })
            await self.write_manifest(manifest)

            return vid
        except Exception:
            if vdir.exists():
                shutil.rmtree(vdir, ignore_errors=True)
            raise

    async def write_manifest(self, manifest: dict) -> None:
        path = self._root / ".slidecraft" / "manifest.json"
        path.parent.mkdir(parents=True, exist_ok=True)
        await atomic_write(
            str(path),
            json.dumps(manifest, ensure_ascii=False, indent=2),
        )
