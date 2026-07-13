import json

import pytest
from src.repositories.file_repo import FileRepo


@pytest.fixture
def file_repo(temp_workspace):
    return FileRepo(temp_workspace, "test-project")


def _ensure_project_dir(temp_workspace):
    proj_dir = temp_workspace / "projects" / "test-project"
    proj_dir.mkdir(parents=True, exist_ok=True)
    return proj_dir


class TestFileRepo:
    @pytest.mark.asyncio
    async def test_write_and_read_html(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "deck.html").write_text("")

        html = "<html><body>hello</body></html>"
        written = await file_repo.write_html(html)
        assert written == "deck.html"
        result = await file_repo.read_html()
        assert result == html

    @pytest.mark.asyncio
    async def test_read_nonexistent_raises(self, file_repo):
        with pytest.raises(FileNotFoundError, match="No HTML file in project"):
            await file_repo.read_html()

    @pytest.mark.asyncio
    async def test_html_exists_false_on_empty(self, file_repo):
        exists = await file_repo.html_exists()
        assert not exists

    @pytest.mark.asyncio
    async def test_html_exists_true_after_write(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "deck.html").write_text("")

        await file_repo.write_html("<html></html>")
        exists = await file_repo.html_exists()
        assert exists

    @pytest.mark.asyncio
    async def test_html_size_returns_correct_value(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "deck.html").write_text("")

        html = "<html></html>"
        await file_repo.write_html(html)
        size = await file_repo.html_size()
        assert size == len(html.encode("utf-8"))

    @pytest.mark.asyncio
    async def test_html_size_zero_when_no_file(self, file_repo):
        size = await file_repo.html_size()
        assert size == 0

    @pytest.mark.asyncio
    async def test_overwrite(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "deck.html").write_text("")

        await file_repo.write_html("first")
        await file_repo.write_html("second")
        result = await file_repo.read_html()
        assert result == "second"


class TestFileRepoActiveFile:
    @pytest.mark.asyncio
    async def test_read_active_json_default(self, file_repo):
        info = await file_repo.read_active_json()
        assert info == {}

    @pytest.mark.asyncio
    async def test_read_active_json_custom(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "active.json").write_text(json.dumps({"active": "deck.html"}))
        info = await file_repo.read_active_json()
        assert info == {"active": "deck.html"}

    @pytest.mark.asyncio
    async def test_get_active_html_returns_content(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "deck.html").write_text("<html></html>")

        result = await file_repo.get_active_html()
        assert result["filename"] == "deck.html"
        assert result["content"] == "<html></html>"

    @pytest.mark.asyncio
    async def test_get_active_html_nonexistent(self, file_repo):
        result = await file_repo.get_active_html()
        assert result == {"filename": None, "content": None}

    @pytest.mark.asyncio
    async def test_list_html_files_empty(self, file_repo):
        files = await file_repo.list_html_files()
        assert files == []

    @pytest.mark.asyncio
    async def test_list_html_files(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "index.html").write_text("<html></html>")
        (proj_dir / "deck.html").write_text("<html></html>")
        files = await file_repo.list_html_files()
        assert len(files) == 2
        filenames = {f["filename"] for f in files}
        assert filenames == {"index.html", "deck.html"}
        for f in files:
            assert f["size_bytes"] > 0
            assert f["mtime"] > 0

    @pytest.mark.asyncio
    async def test_sync_active_file_picks_most_recent(self, file_repo, temp_workspace):
        import time

        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "a.html").write_text("old")
        time.sleep(0.01)
        (proj_dir / "b.html").write_text("new")
        active = await file_repo.sync_active_file()
        assert active == "b.html"

    @pytest.mark.asyncio
    async def test_sync_active_file_no_html(self, file_repo):
        active = await file_repo.sync_active_file()
        assert active is None

    # ── resolve_active_file ──────────────────────────────────────────

    @pytest.mark.asyncio
    async def test_resolve_active_file_returns_none_for_empty_project(self, file_repo):
        result = await file_repo.resolve_active_file()
        assert result is None

    @pytest.mark.asyncio
    async def test_resolve_active_file_reads_active_json(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "active.json").write_text(json.dumps({"active": "deck.html"}))
        (proj_dir / "deck.html").write_text("<html></html>")
        result = await file_repo.resolve_active_file()
        assert result == "deck.html"

    @pytest.mark.asyncio
    async def test_resolve_active_file_falls_back_to_filesystem(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "active.json").write_text(json.dumps({"active": "gone.html"}))
        (proj_dir / "real.html").write_text("<html></html>")
        result = await file_repo.resolve_active_file()
        assert result == "real.html"

    @pytest.mark.asyncio
    async def test_resolve_active_file_repairs_stale_active_json(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "active.json").write_text(json.dumps({"active": "gone.html"}))
        (proj_dir / "real.html").write_text("<html></html>")
        await file_repo.resolve_active_file()
        repaired = json.loads((proj_dir / "active.json").read_text())
        assert repaired == {"active": "real.html"}

    @pytest.mark.asyncio
    async def test_resolve_active_file_rejects_path_traversal(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "active.json").write_text(json.dumps({"active": "../etc/passwd"}))
        (proj_dir / "safe.html").write_text("<html></html>")
        result = await file_repo.resolve_active_file()
        assert result == "safe.html"

    @pytest.mark.asyncio
    async def test_get_active_html_returns_null_for_empty_project(self, file_repo):
        result = await file_repo.get_active_html()
        assert result == {"filename": None, "content": None}

    @pytest.mark.asyncio
    async def test_html_exists_false_for_empty_project(self, file_repo):
        exists = await file_repo.html_exists()
        assert exists is False

    @pytest.mark.asyncio
    async def test_write_html_when_empty_raises(self, file_repo):
        with pytest.raises(FileNotFoundError, match="No active HTML file in project"):
            await file_repo.write_html("<html></html>")

    # ── Path traversal guard ─────────────────────────────────────────

    @pytest.mark.asyncio
    async def test_path_traversal_guard_in_filename(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "active.json").write_text(json.dumps({"active": "../etc/passwd"}))
        with pytest.raises(FileNotFoundError, match="No HTML file in project"):
            await file_repo.read_html()

    @pytest.mark.asyncio
    async def test_read_active_json_with_slash_rejected(self, file_repo, temp_workspace):
        proj_dir = _ensure_project_dir(temp_workspace)
        (proj_dir / "active.json").write_text(json.dumps({"active": "etc/passwd"}))
        with pytest.raises(FileNotFoundError, match="No HTML file in project"):
            await file_repo.read_html()
