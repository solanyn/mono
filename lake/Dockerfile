FROM python:3.13-slim AS builder

WORKDIR /build

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

COPY libs/datalake/pyproject.toml libs/datalake/pyproject.toml
COPY libs/datalake/src/ libs/datalake/src/
COPY lake/pyproject.toml lake/pyproject.toml
COPY lake/src/ lake/src/

RUN uv pip install --system --no-cache \
    ./libs/datalake \
    ./lake

FROM python:3.13-slim

RUN groupadd -r lake && useradd -r -g lake -d /app lake

COPY --from=builder /usr/local/lib/python3.13/site-packages /usr/local/lib/python3.13/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY --from=builder /build/libs/datalake/src/datalake /usr/local/lib/python3.13/site-packages/datalake
COPY --from=builder /build/lake/src/lake /usr/local/lib/python3.13/site-packages/lake

RUN mkdir -p /data && chown lake:lake /data

USER lake
WORKDIR /app

ENV PYTHONUNBUFFERED=1
ENV LAKE_STATE_PATH=/data/lake-state.json

ENTRYPOINT ["python", "-m"]
CMD ["lake.ingest.cli", "all"]
