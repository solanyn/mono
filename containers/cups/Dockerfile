ARG DEBIAN_VERSION

FROM debian:${DEBIAN_VERSION}

ARG YQ_VERSION
ARG TARGETARCH

RUN apt-get update && apt-get install -y \
  cups \
  wget \
  && wget "https://github.com/mikefarah/yq/releases/v${YQ_VERSION}/download/yq_linux_${TARGETARCH}" -O /usr/bin/yq && \
  && chmod +x /usr/bin/yq \
  && rm -rf /var/lib/apt/lists/*

COPY cupsd.conf /etc/cups/cupsd.conf
COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]

