FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim

WORKDIR /app

COPY ./ ./
ENV UV_PROJECT_ENVIRONMENT="/usr/local/"
RUN uv sync --locked --no-editable && rm -rf /app/jaxes

RUN useradd -m -s /bin/bash jaxes
USER jaxes

EXPOSE 8080

CMD ["jaxes"]
