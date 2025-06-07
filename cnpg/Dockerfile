ARG CNPG_TAG

FROM ghcr.io/cloudnative-pg/postgresql:$CNPG_TAG

ARG POSTGRES_VERSION
ARG CNPG_TAG
ARG VECTORCHORD_TAG
ARG DOCUMENTDB_TAG
ARG TIMESCALEDB_TAG
ARG TARGETARCH

# drop to root to install packages
USER root
RUN <<EOT
  set -eux

  # Install dependencies
  apt-get update
  apt-get install -y --no-install-recommends curl

  # Add system info environment variables
  . /etc/os-release 2>/dev/null

  curl -L -o /tmp/vchord.deb https://github.com/tensorchord/VectorChord/releases/download/${VECTORCHORD_TAG}/postgresql-${CNPG_TAG%.*}-vchord_${VECTORCHORD_TAG#"v"}-1_${TARGETARCH}.deb
  apt-get install -y /tmp/vchord.deb && rm -f /tmp/vchord.deb

  curl -L -o /tmp/documentdb.deb https://github.com/microsoft/documentdb/releases/download/v${DOCUMENTDB_TAG}/deb${VERSION_ID}-postgresql-${POSTGRES_VERSION}-documentdb_${DOCUMENTDB_TAG/-/.}_$TARGETARCH.deb 
  apt-get install -y /tmp/documentdb.deb && rm -f /tmp/documentdb.deb

  # Add Timescale apt repo
  echo "deb https://packagecloud.io/timescale/timescaledb/debian/ ${VERSION_CODENAME} main" >/etc/apt/sources.list.d/timescaledb.list
  curl -Lsf https://packagecloud.io/timescale/timescaledb/gpgkey | gpg --dearmor >/etc/apt/trusted.gpg.d/timescale.gpg

  # Install Timescale
  apt-get update
  apt-get install -y --no-install-recommends "timescaledb-2-postgresql-${POSTGRES_VERSION}=${TIMESCALE_VERSION}~debian${VERSION_ID}"

  # Cleanup
  apt-get purge -y curl
  rm /etc/apt/sources.list.d/timescaledb.list /etc/apt/trusted.gpg.d/timescale.gpg
  rm -rf /var/cache/apt/*
EOT

USER postgres
