FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

COPY --from=requirements requirements_gpu.txt /app/requirements.txt

RUN uv pip --no-cache-dir install --system -r /app/requirements.txt

COPY --from=jaxeslib . /app

WORKDIR /app

ENTRYPOINT ["uv", "run", "python"]

CMD ["main.py"]
