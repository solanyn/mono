FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim

WORKDIR /app

COPY ../../jaxes/requirements_gpu.txt .
RUN uv pip install --system --no-cache-dir -r requirements_gpu.txt

COPY ../../jaxes/*.py ./

RUN useradd -m -s /bin/bash jaxes
USER jaxes

EXPOSE 8080

CMD ["python", "main.py"]