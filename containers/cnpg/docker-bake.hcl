target "docker-metadata-action" {}

variable "APP" {
  default = "cnpg"
}

variable "DEBIAN_VERSION" {
  // renovate: datasource=docker depName=debian
  default = "bookworm"
}

variable "CNPG_VERSION" {
  // renovate: datasource=docker depName=ghcr.io/cloudnative-pg/postgresql
  default = "17.5"
}

variable "PG_MAJOR" {
  default = split(".", "${CNPG_VERSION}")[0]
}

variable "TIMESCALEDB_VERSION" {
  // renovate: datasource=github-releases depName=ghcr.io/timescale/timescaledb
  default = "2.20.2"
}

variable "SOURCE" {
  default = "https://github.com/cloudnative-pg/cloudnative-pg"
}

variable "VERSION" {
  default = "${CNPG_VERSION}"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    DEBIAN_VERSION = "${DEBIAN_VERSION}"
    CNPG_VERSION = "${CNPG_VERSION}"
    TIMESCALEDB_VERSION = "${TIMESCALEDB_VERSION}"
    PG_MAJOR = "${PG_MAJOR}"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
  }
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
  tags = ["${APP}:${VERSION}"]
}

target "image-all" {
  inherits = ["image"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
}
