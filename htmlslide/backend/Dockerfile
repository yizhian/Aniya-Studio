FROM python:3.12-slim AS builder
WORKDIR /app
RUN pip install uv
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project || uv sync --no-dev --no-install-project

FROM python:3.12-slim AS runtime
WORKDIR /app
RUN apt-get update -o Acquire::Retries=3 && apt-get install -y --no-install-recommends \
    libmagic1 curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/.venv /app/.venv
ENV PATH="/app/.venv/bin:$PATH"
RUN playwright install-deps chromium
RUN playwright install chromium
COPY src/ ./src/
EXPOSE 8000
CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]
