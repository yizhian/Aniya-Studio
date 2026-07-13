from fastapi import APIRouter

from src.api import chat, export, file, project, settings, skills, version

router = APIRouter(prefix="/api/v1")

router.include_router(project.router, tags=["Project"])
router.include_router(chat.router, tags=["Chat"])
router.include_router(file.router, tags=["File"])
router.include_router(export.router, tags=["Export"])
router.include_router(version.router, tags=["Version"])
router.include_router(skills.router, tags=["Skills"])
router.include_router(settings.router, tags=["Settings"])
