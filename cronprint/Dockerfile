FROM ghcr.io/astral-sh/uv:python3.13-bookworm-slim

RUN apt update -y \
  && apt install -y \
  gcc \
  cups \
  && apt-get autoremove -y \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* /tmp/*

COPY --from=requirements requirements_lock.txt /app/requirements.txt

COPY --from=cups cupsd.conf /etc/cups/cupsd.conf
RUN uv pip --no-cache-dir install --system -r /app/requirements.txt

COPY --from=app . /app

WORKDIR /app

ENTRYPOINT ["uv", "run", "python"]

CMD ["main.py"]

