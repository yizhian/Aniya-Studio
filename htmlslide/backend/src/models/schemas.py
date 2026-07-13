from datetime import datetime
from typing import Optional

from pydantic import BaseModel, Field, model_validator


# ─── Project ───

class CreateProjectRequest(BaseModel):
    name: str = Field(default="未命名项目", min_length=1, max_length=200)
    brief: Optional[str] = Field(default=None, max_length=2000)
    design_skill: Optional[str] = Field(default=None, max_length=200)


class UpdateProjectRequest(BaseModel):
    name: Optional[str] = Field(default=None, min_length=1, max_length=200)
    brief: Optional[str] = Field(default=None, max_length=2000)
    design_skill: Optional[str] = Field(default=None, max_length=200)

    @model_validator(mode="after")
    def at_least_one_field(self):
        if self.name is None and self.brief is None and self.design_skill is None:
            raise ValueError("at least one field must be provided")
        return self


class ProjectResponse(BaseModel):
    id: str
    name: str
    current_version: Optional[str] = None
    file_size_bytes: int = 0
    slide_count: int = 0
    has_html: bool = False
    brief: str = ""
    design_skill: str = ""
    created_at: str = ""


# ─── Chat ───

class DomContext(BaseModel):
    """User-selected DOM element info. BFF formats it as text prefix into the message."""
    css_path: str = ""
    tag: str = ""
    text: str = Field(default="", max_length=200)
    styles: dict[str, str] = Field(default_factory=dict)


class RecommendRequest(BaseModel):
    brief: str = Field(..., min_length=1, max_length=2000)
    limit: int = Field(default=3, ge=1, le=5)


class AttachmentMeta(BaseModel):
    """Snapshot of an uploaded file referenced in a user_message timeline entry."""
    original_name: str = Field(..., min_length=1, max_length=500)
    saved_path_rel: str = ""
    type: str = ""
    pages: Optional[int] = None
    char_count: Optional[int] = None
    width: Optional[int] = None
    height: Optional[int] = None
    format: Optional[str] = None
    error: Optional[str] = None


class ChatRequest(BaseModel):
    project_id: str = Field(..., min_length=1)
    prompt: str = Field(..., min_length=1, max_length=4000)
    selected_dom: Optional[DomContext] = None
    attachments: list[AttachmentMeta] = Field(default_factory=list)


# ─── File Upload ───

class FileUploadResponse(BaseModel):
    project_id: str
    file_name: str
    file_size_bytes: int
    html: str
    css: str = ""
    slide_count: int
    is_deck: bool


# ─── Export / Download ───

class ExportResponse(BaseModel):
    project_id: str
    version: str
    html: str
    css: str
    slide_count: int
    file_size_bytes: int


# ─── Version ───

class VersionItem(BaseModel):
    id: str
    tag: str
    title: str
    slide_count: int
    file_size_bytes: int
    created_at: datetime
    current: bool


class VersionListResponse(BaseModel):
    project_id: str
    current_version: Optional[str] = None
    versions: list[VersionItem]


class VersionDetailResponse(BaseModel):
    id: str
    tag: str
    title: str
    html: str
    css: str
    snapshot: dict
    created_at: datetime


class VersionRestoreResponse(BaseModel):
    project_id: str
    restored_to: str
    new_version_id: str
    new_version_tag: str
    html: str
    css: str


# ─── Save Version ───

class SaveVersionRequest(BaseModel):
    title: str = Field(default="用户手动修改", min_length=1, max_length=200)
    html: str = Field(..., min_length=1)


class SaveVersionResponse(BaseModel):
    project_id: str
    version_id: str
    version_tag: str
    title: str
    html: str
    css: str
    created_at: datetime


# ─── Chat History ───

class ChatHistoryResponse(BaseModel):
    project_id: str
    entries: list[dict] = Field(default_factory=list)


# ─── Settings ───

class ProviderConfigRequest(BaseModel):
    provider: str
    api_key: str
    base_url: str
    model_name: str


# ─── Error ───

class ErrorResponse(BaseModel):
    code: str
    message: str
    details: Optional[dict] = None


# ─── Settings ───

class ProviderSyncResponse(BaseModel):
    ok: bool
    message: str


class ProviderTestResponse(BaseModel):
    ok: bool
    message: str
    in_list: bool = False
    verified: bool = False


class ProviderModelsResponse(BaseModel):
    models: list[dict] = Field(default_factory=list)
    source: str = ""
    error: str = ""


# ─── Skills ───

class SkillsListResponse(BaseModel):
    skills: list[dict] = Field(default_factory=list)
    mode: str = ""
    error: str = ""


class SkillContentResponse(BaseModel):
    content: str


class SkillExampleResponse(BaseModel):
    html: str


# ─── Files ───

class ProjectFilesResponse(BaseModel):
    project_id: str
    active: Optional[str] = None
    files: list[dict] = Field(default_factory=list)


class SyncActiveResponse(BaseModel):
    project_id: str
    active: Optional[str] = None


# ─── Skill Precipitation ───

class PrecipitatePreviewRequest(BaseModel):
    project_id: str = Field(..., min_length=1)
    html_content: str = Field(..., min_length=1, max_length=2_000_000)


class PrecipitateConfirmRequest(BaseModel):
    project_id: str = Field(..., min_length=1)
    skill_name: str = Field(..., min_length=1, max_length=100)
    scenario: str = Field(default="marketing", max_length=50)
    skill_md: str = Field(..., min_length=1, max_length=200_000)
    example_html: str = Field(..., min_length=1, max_length=1_000_000)
