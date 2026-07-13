import json
import pytest
from src.repositories.version_repo import VersionRepo, version_tag


@pytest.fixture
def version_repo(temp_workspace):
    return VersionRepo(temp_workspace, "test-project")


def _create_version_dir(workspace, name, mtime, context=None):
    """Helper: create a fake version directory with context.json + index.html."""
    v_dir = workspace / "projects" / "test-project" / ".slidecraft" / "versions" / name
    v_dir.mkdir(parents=True)
    ctx = context or {"title": f"Version {name}", "slide_count": 3}
    (v_dir / "context.json").write_text(json.dumps(ctx))
    (v_dir / "index.html").write_text("<html><style>body{}</style><div class='slide'>a</div></html>")
    # Set mtime
    (v_dir / "index.html").touch()
    import os
    os.utime(str(v_dir), (mtime, mtime))


class TestVersionTag:
    def test_v001(self):
        assert version_tag("v001") == "V1"

    def test_v010(self):
        assert version_tag("v010") == "V10"

    def test_v123(self):
        assert version_tag("v123") == "V123"

    def test_v1(self):
        assert version_tag("v1") == "V1"


class TestVersionRepo:
    @pytest.mark.asyncio
    async def test_empty_dir_returns_none(self, version_repo):
        assert await version_repo.get_latest_version_id() is None

    @pytest.mark.asyncio
    async def test_empty_dir_returns_empty_list(self, version_repo):
        assert await version_repo.list_versions() == []

    @pytest.mark.asyncio
    async def test_empty_dir_manifest_defaults(self, version_repo):
        manifest = await version_repo.read_manifest()
        assert manifest["current_version"] == 0
        assert manifest["versions"] == []

    @pytest.mark.asyncio
    async def test_list_versions(self, temp_workspace, version_repo):
        _create_version_dir(temp_workspace, "v001", 1000)
        _create_version_dir(temp_workspace, "v002", 2000)  # latest by mtime

        versions = await version_repo.list_versions()
        assert len(versions) == 2
        # Newest first (sorted reversely by dir name)
        assert versions[0].id == "v002"
        assert versions[0].tag == "V2"
        assert versions[0].current is True  # v002 is latest
        assert versions[0].slide_count == 3
        assert versions[1].id == "v001"
        assert versions[1].current is False

    @pytest.mark.asyncio
    async def test_get_version(self, temp_workspace, version_repo):
        _create_version_dir(temp_workspace, "v001", 1000, {"title": "My Deck", "slide_count": 5})

        detail = await version_repo.get_version("v001")
        assert detail.id == "v001"
        assert detail.tag == "V1"
        assert detail.title == "My Deck"
        assert detail.snapshot["slide_count"] == 5
        assert "div" in detail.html
        assert "body{}" in detail.css
        assert detail.snapshot["title"] == "My Deck"

    @pytest.mark.asyncio
    async def test_get_version_not_found(self, version_repo):
        with pytest.raises(FileNotFoundError):
            await version_repo.get_version("v999")

    @pytest.mark.asyncio
    async def test_get_latest_version_id(self, temp_workspace, version_repo):
        _create_version_dir(temp_workspace, "v001", 1000)
        _create_version_dir(temp_workspace, "v003", 3000)
        _create_version_dir(temp_workspace, "v002", 2000)

        assert await version_repo.get_latest_version_id() == "v003"

    @pytest.mark.asyncio
    async def test_read_manifest_existing(self, temp_workspace, version_repo):
        manifest_dir = temp_workspace / "projects" / "test-project" / ".slidecraft"
        manifest_dir.mkdir(parents=True)
        manifest = {
            "current_version": 3,
            "html_file": "index.html",
            "versions": [
                {"number": 1, "created_at": "2026-01-01T00:00:00Z", "html_file": "index.html"}
            ]
        }
        (manifest_dir / "manifest.json").write_text(json.dumps(manifest))

        result = await version_repo.read_manifest()
        assert result["current_version"] == 3
        assert len(result["versions"]) == 1

