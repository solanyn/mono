FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y \
  cups \
  cups-filters \
  ghostscript \
  poppler-utils \
  && rm -rf /var/lib/apt/lists/*

COPY ../../cronprint/** ./
ENV UV_PROJECT_ENVIRONMENT="/usr/local/"
RUN uv sync --locked --no-dev

COPY cupsd.conf /etc/cups/cupsd.conf

RUN useradd -m -s /bin/bash cronprint
USER cronprint

EXPOSE 8080

CMD ["python", "main.py"]
