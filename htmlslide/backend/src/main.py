import asyncio
import logging
from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from src.api.router import router as api_router
from src.config import settings
from src.middleware.error_handler import register_error_handlers

logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings.workspace_path.mkdir(parents=True, exist_ok=True)

    async def draft_cleanup_loop():
        from src.api.deps import get_workspace
        while True:
            try:
                ws = get_workspace()
                deleted = await ws.cleanup_drafts(max_age_hours=24)
                if deleted > 0:
                    logger.info("draft cleanup: removed %d abandoned projects", deleted)
            except Exception:
                logger.exception("draft cleanup failed")
            await asyncio.sleep(3600)

    cleanup_task = asyncio.create_task(draft_cleanup_loop())
    try:
        yield
    finally:
        cleanup_task.cancel()


app = FastAPI(
    title="Aniya Studio BFF",
    version="0.1.0",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.cors_origin_list,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

register_error_handlers(app)

app.include_router(api_router)


@app.get("/health")
async def health():
    return {"status": "ok"}
