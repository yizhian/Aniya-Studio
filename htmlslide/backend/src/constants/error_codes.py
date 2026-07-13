from enum import Enum


class ErrorCode(str, Enum):
    # Project
    PROJECT_NOT_FOUND = "project_not_found"

    # File
    UNSUPPORTED_FORMAT = "unsupported_format"
    FILE_TOO_LARGE = "file_too_large"
    EMPTY_FILE = "empty_file"
    FILE_READ_ERROR = "file_read_error"
    FILE_WRITE_ERROR = "file_write_error"

    # Agent
    AGENT_ERROR = "agent_error"

    # Version
    VERSION_NOT_FOUND = "version_not_found"

    # General
    VALIDATION_ERROR = "validation_error"
    INTERNAL_ERROR = "internal_error"


ERROR_STATUS_MAP: dict[ErrorCode, int] = {
    ErrorCode.PROJECT_NOT_FOUND: 404,
    ErrorCode.UNSUPPORTED_FORMAT: 400,
    ErrorCode.FILE_TOO_LARGE: 413,
    ErrorCode.EMPTY_FILE: 422,
    ErrorCode.FILE_READ_ERROR: 500,
    ErrorCode.FILE_WRITE_ERROR: 500,
    ErrorCode.AGENT_ERROR: 502,
    ErrorCode.VERSION_NOT_FOUND: 404,
    ErrorCode.VALIDATION_ERROR: 422,
    ErrorCode.INTERNAL_ERROR: 500,
}
