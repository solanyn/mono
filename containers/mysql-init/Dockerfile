ARG ALPINE_VERSION

FROM docker.io/library/alpine:${ALPINE_VERSION}

ARG TARGETPLATFORM

USER root
WORKDIR /app

RUN \
  apk add --no-cache \
  bash \
  ca-certificates \
  catatonit \
  mysql-client

COPY entrypoint.sh /entrypoint.sh

USER nobody:nogroup

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]

LABEL org.opencontainers.image.source="https://github.com/alpinelinux/aports"
