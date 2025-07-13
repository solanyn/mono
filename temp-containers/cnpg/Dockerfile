ARG CNPG_VERSION
ARG TIMESCALEDB_VERSION
ARG DEBIAN_VERSION

FROM ghcr.io/cloudnative-pg/postgresql:${CNPG_VERSION}-${DEBIAN_VERSION}

ARG PG_MAJOR
ARG CNPG_VERSION
ARG TIMESCALEDB_VERSION
ARG TARGETARCH

# drop to root to install packages
USER root
RUN <<EOT
  set -eux

  # Install dependencies
  apt-get update
  apt-get install -y --no-install-recommends curl

  # Source OS release info
  . /etc/os-release 2>/dev/null

  # Add Timescale apt repo
  echo "deb https://packagecloud.io/timescale/timescaledb/debian/ ${VERSION_CODENAME} main" >/etc/apt/sources.list.d/timescaledb.list
  curl -Lsf https://packagecloud.io/timescale/timescaledb/gpgkey | gpg --dearmor >/etc/apt/trusted.gpg.d/timescale.gpg

  # Install Timescale
  apt-get update
  apt-get install -y --no-install-recommends "timescaledb-2-postgresql-${PG_MAJOR}=${TIMESCALEDB_VERSION}~debian${VERSION_ID}"

  # Cleanup
  apt-get purge -y curl
  rm /etc/apt/sources.list.d/timescaledb.list /etc/apt/trusted.gpg.d/timescale.gpg
  rm -rf /var/cache/apt/*
EOT

USER postgres
