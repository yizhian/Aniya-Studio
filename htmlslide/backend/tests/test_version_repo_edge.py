"""VersionRepo edge cases for uncovered code paths."""
import json
import os

import pytest

from src.repositories.version_repo import VersionRepo


@pytest.fixture
def version_repo(temp_workspace):
    return VersionRepo(temp_workspace, "test-project")


def _create_version_dir(workspace, name, mtime, context=None, create_html=True):
    v_dir = workspace / "projects" / "test-project" / ".slidecraft" / "versions" / name
    v_dir.mkdir(parents=True)
    if context is not None:
        (v_dir / "context.json").write_text(json.dumps(context))
    if create_html:
        (v_dir / "index.html").write_text("<html></html>")
    os.utime(str(v_dir), (mtime, mtime))
    return v_dir


class TestVersionRepoEdges:
    @pytest.mark.asyncio
    async def test_version_dir_without_context_json(self, temp_workspace, version_repo):
        """Coverage: line 96-97 — v_dir exists but context.json doesn't."""
        v_dir = temp_workspace / "projects" / "test-project" / ".slidecraft" / "versions" / "v001"
        v_dir.mkdir(parents=True)
        (v_dir / "index.html").write_text("<html></html>")

        versions = await version_repo.list_versions()
        assert len(versions) == 1
        assert versions[0].slide_count == 0  # default from _read_context_json returning {}
        assert versions[0].title == "v001"  # fallback to dir name

    @pytest.mark.asyncio
    async def test_version_dir_without_index_html(self, temp_workspace, version_repo):
        """Version directory without index.html should show 0 file_size and not crash."""
        _create_version_dir(temp_workspace, "v001", 1000, create_html=False)
        versions = await version_repo.list_versions()
        assert len(versions) == 1
        assert versions[0].file_size_bytes == 0

    @pytest.mark.asyncio
    async def test_list_versions_skips_non_directories(self, temp_workspace, version_repo):
        """Coverage: line 58-59 — non-directory entries in versions dir should be skipped."""
        versions_dir = temp_workspace / "projects" / "test-project" / ".slidecraft" / "versions"
        versions_dir.mkdir(parents=True)
        (versions_dir / "README.txt").write_text("not a version")

        versions = await version_repo.list_versions()
        assert len(versions) == 0

    @pytest.mark.asyncio
    async def test_get_version_missing_context_json(self, temp_workspace, version_repo):
        """get_version when context.json is missing should use defaults."""
        # _create_version_dir with create_html=True but context=None (default)
        # does NOT create context.json — exactly what we want
        _create_version_dir(temp_workspace, "v001", 1000, create_html=True)

        detail = await version_repo.get_version("v001")
        assert detail.snapshot == {}
        assert detail.title == "v001"

    @pytest.mark.asyncio
    async def test_manifest_corrupted_json(self, temp_workspace, version_repo):
        """Corrupted manifest.json should raise, not silently return defaults."""
        manifest_dir = temp_workspace / "projects" / "test-project" / ".slidecraft"
        manifest_dir.mkdir(parents=True)
        (manifest_dir / "manifest.json").write_text("not valid json")

        with pytest.raises(json.JSONDecodeError):
            await version_repo.read_manifest()

    @pytest.mark.asyncio
    async def test_get_latest_version_empty_versions_dir(self, temp_workspace, version_repo):
        """Empty versions directory should return None."""
        versions_dir = temp_workspace / "projects" / "test-project" / ".slidecraft" / "versions"
        versions_dir.mkdir(parents=True)
        assert await version_repo.get_latest_version_id() is None

    @pytest.mark.asyncio
    async def testversion_tag_edge_cases(self):
        from src.repositories.version_repo import version_tag
        assert version_tag("v000") == "V0"
        assert version_tag("V005") == "V5"
        assert version_tag("v999") == "V999"
