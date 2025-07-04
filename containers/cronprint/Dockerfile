FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y \
    cups \
    cups-filters \
    ghostscript \
    poppler-utils \
    && rm -rf /var/lib/apt/lists/*

COPY ../../cronprint/requirements.txt .
RUN uv pip install --system --no-cache-dir -r requirements.txt

COPY cupsd.conf /etc/cups/cupsd.conf
COPY ../../cronprint/*.py ./

RUN useradd -m -s /bin/bash cronprint
USER cronprint

EXPOSE 8080

CMD ["python", "main.py"]