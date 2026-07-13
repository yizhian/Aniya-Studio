import os
import tempfile

import aiofiles


async def atomic_write(path: str, content: str) -> None:
    """Write content to path atomically via temp file + os.replace."""
    dir_path = os.path.dirname(path) or "."
    base = os.path.basename(path)
    fd, tmp_path = tempfile.mkstemp(dir=dir_path, prefix=f"{base}.", suffix=".tmp")
    os.close(fd)
    try:
        async with aiofiles.open(tmp_path, "w", encoding="utf-8") as f:
            await f.write(content)
        os.chmod(tmp_path, 0o644)
        os.replace(tmp_path, path)
    except Exception:
        if os.path.exists(tmp_path):
            os.unlink(tmp_path)
        raise
