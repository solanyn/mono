ARG ALPINE_VERSION
FROM docker.io/library/alpine:${ALPINE_VERSION}

ARG MYSQL_CLIENT_VERSION
USER root
WORKDIR /app

RUN \
  apk add --no-cache \
  bash \
  ca-certificates \
  catatonit \
  mysql-client~${MYSQL_CLIENT_VERSION}

COPY entrypoint.sh /entrypoint.sh

USER nobody:nogroup

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]
