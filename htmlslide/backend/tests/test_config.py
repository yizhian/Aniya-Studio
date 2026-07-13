from pathlib import Path

from src.config import Settings


class TestConfigDefaults:
    def test_default_agent_url(self):
        s = Settings()
        assert s.agent_url == "http://agentgo:8080"

    def test_default_port(self):
        s = Settings()
        assert s.port == 8000

    def test_workspace_path_is_path(self):
        s = Settings()
        assert isinstance(s.workspace_path, Path)

    def test_cors_origin_list_single(self):
        s = Settings(cors_origins="http://localhost:5173")
        assert s.cors_origin_list == ["http://localhost:5173"]

    def test_cors_origin_list_multiple(self):
        s = Settings(cors_origins="http://a.com, http://b.com")
        assert s.cors_origin_list == ["http://a.com", "http://b.com"]

    def test_cors_origin_list_empty(self):
        s = Settings(cors_origins="")
        assert s.cors_origin_list == []

    def test_workspace_path_str_coercion(self):
        s = Settings(workspace_path="/tmp/test")
        assert s.workspace_path == Path("/tmp/test")

    def test_upload_max_size_default(self):
        s = Settings()
        assert s.upload_max_size == 10 * 1024 * 1024
