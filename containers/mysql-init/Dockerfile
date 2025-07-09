ARG DEBIAN_VERSION=bookworm-slim
FROM docker.io/library/debian:${DEBIAN_VERSION}

ARG MYSQL_CLIENT_VERSION=8.0
USER root
WORKDIR /app

# Install MySQL client from Debian repository
RUN \
  apt-get update \
  && apt-get install -y \
    bash \
    catatonit \
    default-mysql-client \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /entrypoint.sh

USER nobody:nogroup

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]
