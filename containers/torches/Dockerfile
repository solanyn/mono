FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim

WORKDIR /app

COPY ../../torches/pyproject.toml ../../torches/requirements-linux.lock ./
ENV UV_PROJECT_ENVIRONMENT="/usr/local/"
RUN uv sync --locked --no-dev

COPY ../../torches/*.py ./

RUN useradd -m -s /bin/bash torches
USER torches

EXPOSE 8080

CMD ["python", "main.py"]
