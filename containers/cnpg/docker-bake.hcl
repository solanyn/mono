variable "APP" {
  default = "cnpg"
}

variable "CNPG_VERSION" {
  // renovate: datasource=docker depName=ghcr.io/cloudnative-pg/postgresql
  default = "17.5"
}

variable "TIMESCALEDB_VERSION" {
  // renovate: datasource=github-releases depName=ghcr.io/timescale/timescaledb
  default = "2.20.2"
}

variable "REGISTRY" {
  default = "ghcr.io/solanyn"
}

variable "SOURCE" {
  default = "https://github.com/cloudnative-pg/cloudnative-pg"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    CNPG_VERSION = "${CNPG_VERSION}"
    TIMESCALEDB_VERSION = "${TIMESCALEDB_VERSION}"
    PG_MAJOR = "17"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
    "org.opencontainers.image.title" = "${APP}"
    "org.opencontainers.image.version" = "${CNPG_VERSION}"
  }
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
  tags = ["${APP}:${CNPG_VERSION}", "${APP}:latest"]
}

target "image-all" {
  inherits = ["image"]
  output = ["type=registry"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
  tags = [
    "${REGISTRY}/${APP}:${CNPG_VERSION}",
    "${REGISTRY}/${APP}:latest"
  ]
}
