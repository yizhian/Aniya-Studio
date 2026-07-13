import logging

import httpx
from fastapi import Request
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

from src.constants.error_codes import ERROR_STATUS_MAP, ErrorCode
from src.models.schemas import ErrorResponse

logger = logging.getLogger(__name__)


def _unpack_detail(exc: StarletteHTTPException) -> tuple[str, str]:
    """Return (code, message) from an HTTPException detail.

    Supports both structured dict details and plain strings.
    """
    if isinstance(exc.detail, dict) and "code" in exc.detail:
        return exc.detail["code"], exc.detail.get("message", str(exc.detail))
    return _code_for_status(exc.status_code), exc.detail


async def http_exception_handler(request: Request, exc: StarletteHTTPException) -> JSONResponse:
    code, message = _unpack_detail(exc)
    return JSONResponse(
        status_code=exc.status_code,
        content=ErrorResponse(code=code, message=message).model_dump(),
    )


async def httpx_status_error_handler(request: Request, exc: httpx.HTTPStatusError) -> JSONResponse:
    """Forward upstream AgentGo errors with their status code and detail."""
    detail = exc.response.text[:500] if exc.response.text else str(exc)
    return JSONResponse(
        status_code=exc.response.status_code,
        content=ErrorResponse(
            code=ErrorCode.AGENT_ERROR.value,
            message=detail,
        ).model_dump(),
    )


async def value_error_handler(request: Request, exc: ValueError) -> JSONResponse:
    return JSONResponse(
        status_code=422,
        content=ErrorResponse(
            code=ErrorCode.VALIDATION_ERROR.value,
            message=str(exc),
        ).model_dump(),
    )


async def generic_exception_handler(request: Request, exc: Exception) -> JSONResponse:
    logger.exception("Unhandled exception: %s", exc)
    return JSONResponse(
        status_code=500,
        content=ErrorResponse(
            code=ErrorCode.INTERNAL_ERROR.value,
            message=str(exc),
        ).model_dump(),
    )


def register_error_handlers(app):
    app.add_exception_handler(StarletteHTTPException, http_exception_handler)
    app.add_exception_handler(httpx.HTTPStatusError, httpx_status_error_handler)
    app.add_exception_handler(ValueError, value_error_handler)
    app.add_exception_handler(Exception, generic_exception_handler)


def _code_for_status(status_code: int) -> str:
    for code, status in ERROR_STATUS_MAP.items():
        if status == status_code:
            return code.value
    return ErrorCode.INTERNAL_ERROR.value
