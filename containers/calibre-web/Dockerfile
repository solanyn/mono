ARG UBUNTU_VERSION
ARG CALIBRE_VERSION

FROM ubuntu:${UBUNTU_VERSION}

ARG TARGETPLATFORM
ARG CALIBRE_VERSION

ENV CALIBRE__CREATE_LIBRARY="true" \
  CALIBRE__LIBRARY="/config" \
  CALIBRE__PORT="8080"

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# hadolint ignore=DL3008,DL3015,SC2086
RUN \
  case "${TARGETPLATFORM}" in \
  'linux/amd64') \
  export ARCH='x86_64'; \
  ;; \
  'linux/arm64') \
  export ARCH='arm64'; \
  ;; \
  esac \
  && \
  apt-get -qq update \
  && \
  apt-get -qq install -y \
  curl \
  catatonit \
  libegl1 \
  libfontconfig1 \
  libgl1 \
  libglx-mesa0 \
  libopengl0 \
  libxdamage1 \
  libxkbcommon0 \
  python3 \
  xz-utils \
  && \
  mkdir -p /opt/calibre \
  && mkdir -p /app \
  && \
  curl -fsSL "https://download.calibre-ebook.com/${CALIBRE_VERSION}/calibre-${CALIBRE_VERSION}-${ARCH}.txz" \
  | tar Jxvf - -C /app \
  && /app/calibre_postinstall \
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

COPY ./entrypoint.sh /entrypoint.sh

USER nobody:nogroup
WORKDIR /config
VOLUME ["/config"]

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]

LABEL org.opencontainers.image.source="https://github.com/kovidgoyal/calibre"
