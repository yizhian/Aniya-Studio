from pathlib import Path
from pydantic import field_validator
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    agent_url: str = "http://agentgo:8080"
    workspace_path: Path = Path("/workspace")
    agentgo_skills_path: Path = Path("/go/.skills")
    port: int = 8000
    log_level: str = "INFO"
    cors_origins: str = "http://localhost:5173,http://localhost:3000"
    agent_timeout: float = 1000.0
    sse_heartbeat_interval: float = 90.0
    upload_max_size: int = 10 * 1024 * 1024
    agent_retry_count: int = 3
    export_chunk_size: int = 65536

    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"

    @field_validator("workspace_path", mode="before")
    @classmethod
    def coerce_path(cls, v: object) -> Path:
        return Path(v) if not isinstance(v, Path) else v

    @property
    def cors_origin_list(self) -> list[str]:
        return [o.strip() for o in self.cors_origins.split(",") if o.strip()]


settings = Settings()
