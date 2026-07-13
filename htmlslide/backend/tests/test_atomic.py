"""Unit tests for atomic write utility."""
import os
import tempfile

import pytest

from src.utils.atomic import atomic_write


@pytest.mark.asyncio
async def test_atomic_write_basic(tmp_path):
    path = str(tmp_path / "test.txt")
    await atomic_write(path, "hello world")

    assert os.path.exists(path)
    with open(path) as f:
        assert f.read() == "hello world"

    # No temp file should remain.
    for fname in os.listdir(tmp_path):
        assert not fname.endswith(".tmp")


@pytest.mark.asyncio
async def test_atomic_write_overwrites_existing(tmp_path):
    path = str(tmp_path / "existing.txt")
    path_obj = tmp_path / "existing.txt"
    path_obj.write_text("old content")

    await atomic_write(path, "new content")

    with open(path) as f:
        assert f.read() == "new content"


@pytest.mark.asyncio
async def test_atomic_write_empty_content(tmp_path):
    path = str(tmp_path / "empty.txt")
    await atomic_write(path, "")

    assert os.path.exists(path)
    with open(path) as f:
        assert f.read() == ""


@pytest.mark.asyncio
async def test_atomic_write_unicode_content(tmp_path):
    path = str(tmp_path / "unicode.txt")
    content = "你好世界\n🎉\nEmoji test"
    await atomic_write(path, content)

    with open(path, encoding="utf-8") as f:
        assert f.read() == content


@pytest.mark.asyncio
async def test_atomic_write_nested_directory(tmp_path):
    nested_dir = tmp_path / "sub" / "deep"
    nested_dir.mkdir(parents=True)
    path = str(nested_dir / "nested.txt")

    await atomic_write(path, "nested content")

    assert os.path.exists(path)


@pytest.mark.asyncio
async def test_atomic_write_cleanup_on_error(tmp_path, monkeypatch):
    """When write fails, the temp file should be cleaned up."""
    path = str(tmp_path / "should_not_exist.txt")

    # Cause an error during the write by patching aiofiles.open
    # to raise after the temp file is created.
    original_mkstemp = tempfile.mkstemp
    created_tmp_files = []

    def tracking_mkstemp(*args, **kwargs):
        fd, tmp = original_mkstemp(*args, **kwargs)
        created_tmp_files.append(tmp)
        return fd, tmp

    monkeypatch.setattr(tempfile, "mkstemp", tracking_mkstemp)

    # Simulate a write failure by monkeypatching os.replace to raise
    def failing_replace(src, dst):
        raise OSError("Simulated disk error")

    monkeypatch.setattr(os, "replace", failing_replace)

    with pytest.raises(OSError, match="Simulated disk error"):
        await atomic_write(path, "content")

    # Target file should NOT exist.
    assert not os.path.exists(path)

    # Temp file should have been cleaned up.
    for tmp_file in created_tmp_files:
        assert not os.path.exists(tmp_file), f"Temp file {tmp_file} not cleaned up"


@pytest.mark.asyncio
async def test_atomic_write_current_directory_fallback(tmp_path, monkeypatch):
    """When path has no dirname, fall back to '.'."""
    monkeypatch.chdir(tmp_path)
    await atomic_write("current_dir_file.txt", "content")

    assert os.path.exists(tmp_path / "current_dir_file.txt")
