ARG UBUNTU_VERSION
ARG TARGETARCH
ARG CALIBRE_ARCH=${TARGETARCH/amd64/x86_64}

FROM ubuntu:${UBUNTU_VERSION}

ENV \
  PYTHONDONTWRITEBYTECODE=1 \
  PYTHONUNBUFFERED=1 \
  PIP_BREAK_SYSTEM_PACKAGES=1 \
  PIP_ROOT_USER_ACTION=ignore \
  PIP_NO_CACHE_DIR=1 \
  PIP_DISABLE_PIP_VERSION_CHECK=1

ENV CALIBRE_DBPATH=/config

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# hadolint ignore=DL3008,DL3015,SC2086
RUN \
  apt-get update \
  && \
  apt-get install -y \
  curl \
  catatonit \
  imagemagick \
  libegl1 \
  libfontconfig1 \
  libgl1 \
  libglx-mesa0 \
  libldap2 \
  libnss3 \
  libopengl0 \
  libsasl2-2 \
  libxcomposite1 \
  libxdamage1 \
  libxi6 \
  libxkbcommon0 \
  libxrandr2 \
  libxslt1.1 \
  python3 \
  python3-pip \
  python3-pkg-resources \
  && \
  # Dev
  apt-get install -y \
  libldap2-dev \
  libsasl2-dev \
  python3-dev \
  gcc \
  g++ \
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
  # calibre
  && mkdir -p /opt/calibre \
  && curl -fsSL "https://download.calibre-ebook.com/${CALIBRE_VERSION}/calibre-${CALIBRE_VERSION}-${CALIBRE_ARCH}.txz" \
  | tar Jxvf - -C /opt/calibre \
  && /app/calibre_postinstall \
  && apt-get purge -y --auto-remove libldap2-dev libsasl2-dev python3-dev gcc g++\
  && apt-get purge -y --auto-remove -o APT::AutoRemove::RecommendsImportant=false \
  && apt-get autoremove -y \
  && apt-get clean \
  && \
  rm -rf \
  /tmp/* \
  /var/lib/apt/lists/* \
  /var/tmp/ \
  && chown -R root:root /app \
  && chmod -R 755 /app \
  && printf "umask %d" "${UMASK}" >> /etc/bash.bashrc

USER nobody:nogroup
WORKDIR /config
VOLUME ["/config"]

ENTRYPOINT ["/usr/bin/catatonit", "--", "python3", "/app/cps.py", "-o", "/dev/stdout"]
