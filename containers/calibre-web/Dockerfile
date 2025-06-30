ARG DEBIAN_VERSION=bookworm-slim

FROM debian:${DEBIAN_VERSION}

ARG TARGETARCH
ARG CALIBRE_ARCH=${TARGETARCH/amd64/x86_64}
ARG KEPUBIFY_VERSION
ARG CALIBRE_VERSION
ARG CALIBRE_WEB_VERSION

ENV \
  PYTHONDONTWRITEBYTECODE=1 \
  PYTHONUNBUFFERED=1 \
  PIP_BREAK_SYSTEM_PACKAGES=1 \
  PIP_ROOT_USER_ACTION=ignore \
  PIP_NO_CACHE_DIR=1 \
  PIP_DISABLE_PIP_VERSION_CHECK=1

ENV CALIBRE_DBPATH=/config

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN \
  apt-get update && apt-get install -y --no-install-recommends \
  curl \
  ca-certificates \
  catatonit \
  imagemagick \
  libldap-2.5-0 \
  libsasl2-2 \
  libx11-6 \
  libxcomposite1 \
  libxdamage1 \
  libxi6 \
  libxkbcommon0 \
  libxrandr2 \
  libxslt1.1 \
  python3 \
  python3-pip \
  xz-utils \
  && \
  # Build deps
  apt-get install -y --no-install-recommends \
  libldap2-dev \
  libsasl2-dev \
  python3-dev \
  gcc \
  g++ \
  libc6-dev \
  linux-libc-dev \
  && \
  # calibre-web
  mkdir -p /app \
  && curl -fsSL https://github.com/janeczku/calibre-web/archive/refs/tags/${CALIBRE_WEB_VERSION}.tar.gz \
  | tar xzf - -C /app --strip-components=1 \
  && pip install --upgrade --requirement /app/requirements.txt \
  && pip install --upgrade --requirement /app/optional-requirements.txt \
  # kepubify
  && mkdir -p /opt/kepubify \
  && curl -o \
  /opt/kepubify/kepubify -L \
  "https://github.com/pgaskin/kepubify/releases/download/v${KEPUBIFY_VERSION}/kepubify-linux-${TARGETARCH}" \
  && chmod +x /opt/kepubify/kepubify \
  # calibre
  && mkdir -p /opt/calibre \
  && curl -fsSL "https://github.com/kovidgoyal/calibre/releases/download/v${CALIBRE_VERSION}/calibre-${CALIBRE_VERSION}-${CALIBRE_ARCH}.txz" \
  | tar Jxvf - -C /opt/calibre \
  && apt-get purge -y \
  libldap2-dev \
  libsasl2-dev \
  python3-dev \
  gcc \
  g++ \
  libc6-dev \
  linux-libc-dev \
  && apt-get autoremove -y \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* /tmp/* \
  && chown -R root:root /app \
  && chmod -R 755 /app

USER nobody:nogroup
WORKDIR /config
VOLUME ["/config"]

ENTRYPOINT ["/usr/bin/catatonit", "--", "python3", "/app/cps.py", "-o", "/dev/stdout"]
