FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y \
  gcc \
  libcups2-dev \
  && rm -rf /var/lib/apt/lists/*

COPY ./cronprint/pyproject.toml ./cronprint/uv.lock ./
COPY ./cronprint/cronprint ./cronprint
ENV UV_PROJECT_ENVIRONMENT="/usr/local/"
RUN uv sync --locked --no-editable && rm -rf /app/cronprint

FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y \
  cups \
  libcups2 \
  && rm -rf /var/lib/apt/lists/*

COPY --from=builder /usr/local /usr/local

RUN useradd -m -s /bin/bash cronprint
USER cronprint

EXPOSE 8080

CMD ["cronprint"]
