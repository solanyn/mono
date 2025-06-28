variable "APP" {
  default = "mysql-init"
}


variable "ALPINE_VERSION" {
  // renovate: datasource=docker depName=alpine
  default = "3.22"
}

variable "REGISTRY" {
  default = "ghcr.io/solanyn"
}

variable "SOURCE" {
  default = "https://github.com/alpinelinux/aports"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    ALPINE_VERSION = "${ALPINE_VERSION}"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
    "org.opencontainers.image.title" = "${APP}"
    "org.opencontainers.image.version" = "${ALPINE_VERSION}"
  }
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
  tags = ["${APP}:${ALPINE_VERSION}", "${APP}:latest"]
}

target "image-all" {
  inherits = ["image"]
  output = ["type=registry"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
  tags = [
    "${REGISTRY}/${APP}:${ALPINE_VERSION}",
    "${REGISTRY}/${APP}:latest"
  ]
}