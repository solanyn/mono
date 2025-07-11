FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim AS builder

ARG VERSION=3.1.1

ENV UV_COMPILE_BYTECODE=1
ENV UV_LINK_MODE=copy

WORKDIR /app
RUN uv venv && \
  uv pip install \
  "mlflow==${VERSION}" \
  psycopg2-binary \
  boto3

FROM python:3.13-slim-bookworm

ARG TARGETARCH
ARG VERSION=3.1.1

RUN groupadd -g 1001 mlflow && \
  useradd -u 1001 -g mlflow -s /bin/bash -m mlflow

RUN apt-get update && apt-get install -y \
  curl \
  postgresql-client \
  && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/.venv /app/.venv

RUN mkdir -p /config && chown mlflow:mlflow /config

USER mlflow

WORKDIR /config

EXPOSE 5000

ENV PATH="/app/.venv/bin:$PATH"
ENV MLFLOW_BACKEND_STORE_URI=sqlite:///config/mlflow.db
ENV MLFLOW_DEFAULT_ARTIFACT_ROOT=file:///config/artifacts
ENV MLFLOW_HOST=0.0.0.0
ENV MLFLOW_PORT=5000

CMD ["mlflow", "server", \
  "--backend-store-uri", "${MLFLOW_BACKEND_STORE_URI}", \
  "--default-artifact-root", "${MLFLOW_DEFAULT_ARTIFACT_ROOT}", \
  "--host", "${MLFLOW_HOST}", \
  "--port", "${MLFLOW_PORT}"]
