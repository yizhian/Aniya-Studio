"""Unit tests for deps.py helper functions: require_html, require_project, clear_project_caches."""
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from fastapi import HTTPException

from src.api.deps import clear_project_caches, require_html
from src.constants.error_codes import ErrorCode


class TestRequireHtml:
    @pytest.mark.asyncio
    async def test_require_html_returns_content_when_exists(self):
        file_repo = MagicMock()
        file_repo.html_exists = AsyncMock(return_value=True)
        file_repo.read_html = AsyncMock(return_value="<html><body></body></html>")

        result = await require_html(file_repo)
        assert result == "<html><body></body></html>"

    @pytest.mark.asyncio
    async def test_require_html_raises_404_when_no_html(self):
        file_repo = MagicMock()
        file_repo.html_exists = AsyncMock(return_value=False)

        with pytest.raises(HTTPException) as exc:
            await require_html(file_repo)
        assert exc.value.status_code == 404
        assert exc.value.detail["code"] == ErrorCode.FILE_READ_ERROR.value
        assert "Project has no HTML content" in exc.value.detail["message"]

    @pytest.mark.asyncio
    async def test_require_html_custom_message(self):
        file_repo = MagicMock()
        file_repo.html_exists = AsyncMock(return_value=False)

        with pytest.raises(HTTPException) as exc:
            await require_html(file_repo, "自定义错误信息")
        assert exc.value.status_code == 404
        assert exc.value.detail["message"] == "自定义错误信息"

    @pytest.mark.asyncio
    async def test_require_html_default_message_in_english(self):
        file_repo = MagicMock()
        file_repo.html_exists = AsyncMock(return_value=False)

        with pytest.raises(HTTPException) as exc:
            await require_html(file_repo)
        assert exc.value.detail["message"] == "Project has no HTML content"


class TestClearProjectCaches:
    def test_clears_all_four_caches(self):
        # Populate caches by accessing getters via imports
        from src.api import deps

        pid = "test-pid-for-clear"

        # Force creation in all 4 caches
        with patch("src.api.deps.get_workspace") as mock_ws:
            mock_ws.return_value = MagicMock()
            deps.get_file_repo(pid)
            deps.get_version_repo(pid)
            deps.get_project_service(pid)
            deps.get_project_lock(pid)

        # Verify caches are populated
        assert pid in deps._file_repo_cache
        assert pid in deps._version_repo_cache
        assert pid in deps._project_service_cache
        assert pid in deps._project_locks

        # Clear and verify
        clear_project_caches(pid)
        assert pid not in deps._file_repo_cache
        assert pid not in deps._version_repo_cache
        assert pid not in deps._project_service_cache
        assert pid not in deps._project_locks

    def test_clear_nonexistent_id_no_error(self):
        """Clearing a project_id not in caches should not raise."""
        clear_project_caches("nonexistent-id")
