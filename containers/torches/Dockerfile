FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim

WORKDIR /app

COPY ./torches/pyproject.toml ./torches/uv.lock ./
COPY ./torches/torches ./torches
ENV UV_PROJECT_ENVIRONMENT="/usr/local/"
RUN uv sync --locked --no-editable && rm -rf /app/torches

RUN useradd -m -s /bin/bash torches
USER torches

EXPOSE 8080

CMD ["torches"]
