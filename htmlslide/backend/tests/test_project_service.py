import pytest
from src.repositories.file_repo import FileRepo
from src.repositories.version_repo import VersionRepo
from src.services.project_service import ProjectService
from src.services.workspace import WorkspaceService


@pytest.fixture
def project_service(temp_workspace):
    ws = WorkspaceService(temp_workspace)
    fr = FileRepo(temp_workspace, "test-project")
    vr = VersionRepo(temp_workspace, "test-project")
    return ProjectService(fr, vr, ws)


class TestProjectService:
    @pytest.mark.asyncio
    async def test_empty_project(self, temp_workspace):
        ws = WorkspaceService(temp_workspace)
        meta = await ws.create_project("Empty")
        pid = meta["id"]
        # Use repos for this specific project
        fr = FileRepo(temp_workspace, pid)
        vr = VersionRepo(temp_workspace, pid)
        ps = ProjectService(fr, vr, ws)
        info = await ps.get_project_info(pid)
        assert info.current_version is None
        assert info.has_html is False
        assert info.slide_count == 0

    @pytest.mark.asyncio
    async def test_project_with_html(self, temp_workspace):
        ws = WorkspaceService(temp_workspace)
        meta = await ws.create_project("With HTML")
        pid = meta["id"]

        # Write HTML using a FileRepo specific to this project
        fr = FileRepo(temp_workspace, pid)
        vr = VersionRepo(temp_workspace, pid)

        # Pre-create an HTML file (simulating Agent write_file)
        proj_dir = temp_workspace / "projects" / pid
        (proj_dir / "deck.html").write_text("")

        html = "<html><style>body{}</style><div class='slide'>1</div><div class='slide'>2</div></html>"
        await fr.write_html(html)

        # Use project-specific service
        ps = ProjectService(fr, vr, ws)
        info = await ps.get_project_info(pid)
        assert info.has_html is True
        assert info.slide_count == 2
        assert info.file_size_bytes > 0
        assert info.id == pid
        assert info.name == "With HTML"

    @pytest.mark.asyncio
    async def test_overwrite_project_name(self, temp_workspace, project_service):
        ws = WorkspaceService(temp_workspace)
        meta = await ws.create_project("MyProject")
        # Create project-specific service for the same project_id
        fr = FileRepo(temp_workspace, meta["id"])
        vr = VersionRepo(temp_workspace, meta["id"])
        ps = ProjectService(fr, vr, ws)
        info = await ps.get_project_info(meta["id"])
        assert info.name == "MyProject"
