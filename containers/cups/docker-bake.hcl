target "docker-metadata-action" {}

variable "APP" {
  default = "cups"
}

variable "DEBIAN_VERSION" {
  // renovate: datasource=docker depName=debian
  default = "bookworm-slim"
}

variable "REGISTRY" {
  default = "ghcr.io/solanyn"
}

variable "SOURCE" {
  default = "https://github.com/OpenPrinting/cups"
}

variable "VERSION" {
  default = "${DEBIAN_VERSION}"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    DEBIAN_VERSION = "${DEBIAN_VERSION}"
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
