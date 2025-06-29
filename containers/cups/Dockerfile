ARG DEBIAN_VERSION
ARG TARGETARCH

FROM debian:${DEBIAN_VERSION}

RUN apt-get update && apt-get install -y \
  cups \
  wget \
  && rm -rf /var/lib/apt/lists/*

RUN wget https://github.com/mikefarah/yq/releases/latest/download/yq_linux_${TARGETARCH} -O /usr/bin/yq && \
  chmod +x /usr/bin/yq

COPY cupsd.conf /etc/cups/cupsd.conf
COPY entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh

EXPOSE 631

ENTRYPOINT ["/entrypoint.sh"]

