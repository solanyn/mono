FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim

COPY --from=requirements requirements_gpu.txt /app/requirements.txt

RUN uv pip --no-cache-dir install --system -r /app/requirements.txt

COPY --from=torcheslib . /app

WORKDIR /app

ENTRYPOINT ["uv", "run", "python"]

CMD ["main.py"]

