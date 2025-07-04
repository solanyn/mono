FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim

WORKDIR /app

COPY ../../torches/requirements_gpu.txt .
RUN uv pip install --system --no-cache-dir -r requirements_gpu.txt

COPY ../../torches/*.py ./

RUN useradd -m -s /bin/bash torches
USER torches

EXPOSE 8080

CMD ["python", "main.py"]