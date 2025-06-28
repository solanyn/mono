FROM python:3.12-slim AS builder

COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /usr/local/bin/

# Install system dependencies needed for CUPS
RUN apt-get update && apt-get install -y \
  gcc \
  libcups2-dev \
  pkg-config \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

ENV UV_COMPILE_BYTECODE=1

ENV UV_LINK_MODE=copy

RUN uv venv /app/.venv

COPY requirements_lock.txt ./requirements.txt

RUN --mount=type=cache,target=/root/.cache/uv \
  uv pip install --python /app/.venv -r requirements.txt

COPY . .

ENV CRONPRINT_HOST=0.0.0.0
ENV CRONPRINT_PORT=8080
ENV CRONPRINT_TIMEZONE=UTC

WORKDIR /app

ENTRYPOINT ["/app/.venv/bin/uvicorn", "main:create_app", "--factory", "--host", "0.0.0.0", "--port", "8080"]
