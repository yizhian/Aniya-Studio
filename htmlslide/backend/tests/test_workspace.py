import json
import pytest
from src.services.workspace import WorkspaceService


@pytest.fixture
def workspace(temp_workspace):
    return WorkspaceService(temp_workspace)


class TestWorkspaceService:
    def test_generate_project_id_format(self, workspace):
        pid = workspace.generate_project_id()
        assert pid.startswith("proj-")
        assert len(pid) == 17  # "proj-" + 12 hex chars

    def test_generate_project_id_unique(self, workspace):
        ids = {workspace.generate_project_id() for _ in range(10)}
        assert len(ids) == 10

    @pytest.mark.asyncio
    async def test_create_project(self, workspace):
        meta = await workspace.create_project("Test Project")
        assert meta["name"] == "Test Project"
        assert meta["id"].startswith("proj-")
        assert "created_at" in meta

        # Verify project.json was written under the project directory
        proj_dir = workspace._root / "projects" / meta["id"]
        project_json = json.loads((proj_dir / "project.json").read_text())
        assert project_json["name"] == "Test Project"

        # Verify no template files
        html_files = list(proj_dir.glob("*.html"))
        assert len(html_files) == 0
        assert not (proj_dir / "active.json").exists()

    @pytest.mark.asyncio
    async def test_read_meta_roundtrip(self, workspace):
        created = await workspace.create_project("Roundtrip")
        read = await workspace.read_meta(created["id"])
        assert read["id"] == created["id"]
        assert read["name"] == created["name"]
        assert read["created_at"] == created["created_at"]

    @pytest.mark.asyncio
    async def test_read_meta_empty(self, workspace):
        meta = await workspace.read_meta("proj-nonexistent")
        assert meta == {}

    @pytest.mark.asyncio
    async def test_update_project(self, workspace):
        created = await workspace.create_project("Old Name")
        pid = created["id"]

        meta = await workspace.update_project(pid, "New Name")
        assert meta is not None
        assert meta["name"] == "New Name"
        assert "updated_at" in meta

        # Verify persistence.
        read = await workspace.read_meta(pid)
        assert read["name"] == "New Name"

    @pytest.mark.asyncio
    async def test_update_project_nonexistent(self, workspace):
        meta = await workspace.update_project("proj-nonexistent", "X")
        assert meta is None

    @pytest.mark.asyncio
    async def test_cleanup_drafts_removes_empty_project(self, workspace):
        # Create an empty project (no HTML).
        meta = await workspace.create_project("Empty Draft")
        pid = meta["id"]

        deleted = await workspace.cleanup_drafts(max_age_hours=0)  # 0 = immediately
        assert deleted >= 1

        # Verify project is gone.
        read = await workspace.read_meta(pid)
        assert read == {}

    @pytest.mark.asyncio
    async def test_cleanup_drafts_keeps_active_project(self, workspace):
        # Create a project and add a .html file.
        meta = await workspace.create_project("Active Project")
        pid = meta["id"]
        proj_dir = workspace._root / "projects" / pid
        (proj_dir / "slides.html").write_text("<h1>Hello</h1>")

        await workspace.cleanup_drafts(max_age_hours=0)

        # Active project must still exist.
        read = await workspace.read_meta(pid)
        assert read["name"] == "Active Project"

    @pytest.mark.asyncio
    async def test_update_project_with_design_skill(self, workspace):
        created = await workspace.create_project("Design")
        pid = created["id"]

        meta = await workspace.update_project(pid, design_skill="slides")
        assert meta is not None
        assert meta["design_skill"] == "slides"

        read = await workspace.read_meta(pid)
        assert read["design_skill"] == "slides"

    @pytest.mark.asyncio
    async def test_update_project_with_brief(self, workspace):
        created = await workspace.create_project("With Brief")
        pid = created["id"]

        meta = await workspace.update_project(pid, brief="A presentation project")
        assert meta is not None
        assert meta["brief"] == "A presentation project"

    @pytest.mark.asyncio
    async def test_update_project_multiple_fields(self, workspace):
        created = await workspace.create_project("Multi")
        pid = created["id"]

        meta = await workspace.update_project(
            pid, name="Renamed", brief="Updated brief", design_skill="slides"
        )
        assert meta["name"] == "Renamed"
        assert meta["brief"] == "Updated brief"
        assert meta["design_skill"] == "slides"

    @pytest.mark.asyncio
    async def test_create_project_with_brief(self, workspace):
        meta = await workspace.create_project("Brief Project", brief="A test brief")
        assert meta["brief"] == "A test brief"

        proj_dir = workspace._root / "projects" / meta["id"]
        project_json = json.loads((proj_dir / "project.json").read_text())
        assert project_json["brief"] == "A test brief"

    @pytest.mark.asyncio
    async def test_create_project_with_design_skill(self, workspace):
        meta = await workspace.create_project(
            "Styled Project", brief="deck", design_skill="html-ppt-pitch-deck",
        )
        assert meta["design_skill"] == "html-ppt-pitch-deck"

        proj_dir = workspace._root / "projects" / meta["id"]
        project_json = json.loads((proj_dir / "project.json").read_text())
        assert project_json["design_skill"] == "html-ppt-pitch-deck"

    def test_project_dir(self, workspace):
        path = workspace.project_dir("proj-test123")
        assert str(path).endswith("proj-test123")

    @pytest.mark.asyncio
    async def test_delete_project_success(self, workspace):
        meta = await workspace.create_project("To Delete")
        pid = meta["id"]

        deleted = await workspace.delete_project(pid)
        assert deleted is True

        # Project should no longer exist.
        read = await workspace.read_meta(pid)
        assert read == {}

    @pytest.mark.asyncio
    async def test_delete_project_not_found(self, workspace):
        deleted = await workspace.delete_project("proj-nonexistent")
        assert deleted is False

    @pytest.mark.asyncio
    async def test_list_projects_empty(self, workspace):
        projects = await workspace.list_projects()
        assert projects == []

    @pytest.mark.asyncio
    async def test_list_projects_with_data(self, workspace):
        a = await workspace.create_project("Project A")
        b = await workspace.create_project("Project B")

        projects = await workspace.list_projects()
        assert len(projects) >= 2
        names = [p["name"] for p in projects]
        assert "Project A" in names
        assert "Project B" in names

        # Most recently created should be first.
        assert projects[0]["name"] == "Project B"

    @pytest.mark.asyncio
    async def test_list_projects_skips_invalid_dirs(self, workspace):
        await workspace.create_project("Valid")
        # Create a directory without project.json.
        bad_dir = workspace._root / "projects" / "not-a-project"
        bad_dir.mkdir(parents=True, exist_ok=True)

        projects = await workspace.list_projects()
        names = [p["name"] for p in projects]
        assert "Valid" in names

    @pytest.mark.asyncio
    async def test_cleanup_drafts_keeps_recent_project(self, workspace):
        meta = await workspace.create_project("Recent Draft")
        pid = meta["id"]

        # With max_age_hours=24, a just-created project is kept.
        deleted = await workspace.cleanup_drafts(max_age_hours=24)
        assert deleted == 0

        read = await workspace.read_meta(pid)
        assert read["name"] == "Recent Draft"

    @pytest.mark.asyncio
    async def test_cleanup_drafts_skips_missing_project_json(self, workspace):
        # Create a directory under projects without project.json.
        draft_dir = workspace._root / "projects" / "orphan"
        draft_dir.mkdir(parents=True, exist_ok=True)

        # This should not raise, and should skip the orphan dir.
        deleted = await workspace.cleanup_drafts(max_age_hours=0)
        # No project.json → not counted as a project to delete.
        assert deleted >= 0

    @pytest.mark.asyncio
    async def test_cleanup_drafts_no_projects_dir(self, workspace):
        """When projects dir doesn't exist, returns 0."""
        # workspace fixture has no projects dir initially
        # _root / "projects" doesn't exist
        deleted = await workspace.cleanup_drafts(max_age_hours=0)
        assert deleted == 0

    @pytest.mark.asyncio
    async def test_list_projects_no_projects_dir(self, workspace):
        """When projects dir doesn't exist, returns empty list."""
        projects = await workspace.list_projects()
        assert projects == []

    @pytest.mark.asyncio
    async def test_cleanup_drafts_skips_non_dirs(self, workspace):
        """Non-directory entries in projects/ are skipped."""
        projects_dir = workspace._root / "projects"
        projects_dir.mkdir(parents=True, exist_ok=True)
        (projects_dir / "some_file.txt").write_text("not a dir")

        deleted = await workspace.cleanup_drafts(max_age_hours=0)
        assert deleted >= 0

    @pytest.mark.asyncio
    async def test_list_projects_skips_non_dir_entries(self, workspace):
        """Coverage: workspace.py line 83 — non-directory entries in projects/."""
        await workspace.create_project("Real")
        projects_dir = workspace._root / "projects"
        (projects_dir / "not_a_dir.txt").write_text("file")

        projects = await workspace.list_projects()
        names = [p["name"] for p in projects]
        assert "Real" in names
        assert len(projects) == 1


class TestSymlinkError:
    """Coverage: workspace.py lines 52-53 — symlink OSError is non-fatal."""

    @pytest.mark.asyncio
    async def test_symlink_oserror_does_not_block_creation(self, temp_workspace):
        from unittest.mock import patch

        ws = WorkspaceService(temp_workspace, skills_source=temp_workspace / "skills")

        with patch("pathlib.Path.symlink_to", side_effect=OSError("Permission denied")):
            meta = await ws.create_project("SymlinkFail")
        assert meta["name"] == "SymlinkFail"
        assert meta["id"].startswith("proj-")
        # Project directory and project.json should still exist
        proj_dir = temp_workspace / "projects" / meta["id"]
        assert proj_dir.is_dir()
        assert (proj_dir / "project.json").is_file()
