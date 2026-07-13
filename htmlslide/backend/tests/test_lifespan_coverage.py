"""Cover main.py lifespan yield line that unit tests miss."""
import pytest


class TestLifespan:
    def test_lifespan_startup_and_shutdown(self, temp_workspace):
        """Directly exercise the lifespan async context manager to cover yield."""
        import asyncio

        from src.config import settings
        from src.main import lifespan

        settings.workspace_path = temp_workspace

        async def run():
            async with lifespan(None):
                assert temp_workspace.is_dir()

        asyncio.run(run())
