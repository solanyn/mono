ARG DEBIAN_VERSION=bookworm-slim

FROM debian:${DEBIAN_VERSION}

ARG CALIBRE_VERSION
ARG TARGETARCH
ARG TARGETARCH=${TARGETARCH/amd64/x86_64}

ENV CALIBRE__CREATE_LIBRARY="true" \
  CALIBRE__LIBRARY="/config" \
  CALIBRE__PORT="8080"

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN \
  apt-get update && apt-get install -y --no-install-recommends \
  curl \
  ca-certificates \
  catatonit \
  xz-utils \
  && mkdir -p /app \
  && curl -fsSL "https://github.com/kovidgoyal/calibre/releases/download/v${CALIBRE_VERSION}/calibre-${CALIBRE_VERSION}-${TARGETARCH}.txz" \
  | tar Jxvf - -C /app \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/* /tmp/* \
  && chown -R root:root /app \
  && chmod -R 755 /app

COPY ./entrypoint.sh /entrypoint.sh

USER nobody:nogroup
WORKDIR /config
VOLUME ["/config"]

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]
