# syntax=docker/dockerfile:1

# Stage 1: build the cronprint binary. Mirror the Bazel go_binary target
# using the Go toolchain directly so the runtime image stays self-contained.
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY cronprint/go.mod cronprint/go.sum ./
RUN go mod download
COPY cronprint/*.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cronprint .

# Stage 2: CUPS + ESC/P-R runtime.
#
# The Epson ET-2810 advertises IPP Everywhere but silently drops PWG raster
# jobs. Reliable printing requires CUPS with the ESC/P-R driver, which
# converts PDF/postscript to Epson's native protocol and sends it over a raw
# AppSocket (port 9100). No TLS/IPP quirks on that path.
FROM debian:bookworm-slim AS runtime

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        cups \
        cups-client \
        cups-filters \
        printer-driver-escpr \
        ghostscript \
        libcupsimage2 \
        xz-utils \
        tzdata \
    && rm -rf /var/lib/apt/lists/*

# CUPS daemon state dirs (the Debian package creates them for its own user
# but we run as root in a container; make sure they exist and are writable).
RUN mkdir -p /var/run/cups /var/spool/cups /var/cache/cups /etc/cups

COPY --from=build /out/cronprint /usr/local/bin/cronprint
COPY cronprint/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080 631

ENTRYPOINT ["/entrypoint.sh"]