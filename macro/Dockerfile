FROM python:3.13-slim AS builder

WORKDIR /build

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

COPY libs/datalake/pyproject.toml libs/datalake/pyproject.toml
COPY libs/datalake/src/ libs/datalake/src/
COPY macro/pyproject.toml macro/pyproject.toml
COPY macro/src/ macro/src/

RUN uv pip install --system --no-cache \
    ./libs/datalake \
    ./macro

FROM python:3.13-slim

RUN groupadd -r macro && useradd -r -g macro -d /app macro

COPY --from=builder /usr/local/lib/python3.13/site-packages /usr/local/lib/python3.13/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY --from=builder /build/libs/datalake/src/datalake /usr/local/lib/python3.13/site-packages/datalake
COPY --from=builder /build/macro/src/macro /usr/local/lib/python3.13/site-packages/macro

RUN mkdir -p /data && chown macro:macro /data

USER macro
WORKDIR /app

ENV PYTHONUNBUFFERED=1
ENV MACRO_STATE_PATH=/data/macro-state.json

ENTRYPOINT ["python", "-m"]
CMD ["macro.ingest.cli", "all"]
