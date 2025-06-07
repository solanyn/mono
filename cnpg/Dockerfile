ARG CNPG_TAG

FROM ghcr.io/cloudnative-pg/postgresql:$CNPG_TAG-bookworm

ARG PG_MAJOR
ARG CNPG_TAG
ARG VECTORCHORD_TAG
ARG DOCUMENTDB_TAG
ARG TIMESCALEDB_TAG
ARG TARGETARCH

# drop to root to install packages
USER root
ADD https://github.com/tensorchord/VectorChord/releases/download/${VECTORCHORD_TAG}/postgresql-${PG_MAJOR}-vchord_${VECTORCHORD_TAG#"v"}-1_${TARGETARCH}.deb /tmp/vchord.deb
RUN apt-get install -y /tmp/vchord.deb && rm -f /tmp/vchord.deb

ADD https://github.com/microsoft/documentdb/releases/download/v${DOCUMENTDB_TAG}/deb11-postgresql-${PG_MAJOR}-documentdb_${DOCUMENTDB_TAG/-/.}_$TARGETARCH.deb /tmp/documentdb.deb 
RUN apt-get install -y /tmp/documentdb.deb && rm -f /tmp/documentdb.deb

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
  apt-get install -y --no-install-recommends "timescaledb-2-postgresql-${PG_MAJOR}=${TIMESCALEDB_TAG}~debian${VERSION_ID}"

  # Cleanup
  apt-get purge -y curl
  rm /etc/apt/sources.list.d/timescaledb.list /etc/apt/trusted.gpg.d/timescale.gpg
  rm -rf /var/cache/apt/*
EOT

USER postgres
