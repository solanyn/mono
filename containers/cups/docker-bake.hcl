target "docker-metadata-action" {}

variable "APP" {
  default = "cups"
}

variable "DEBIAN_VERSION" {
  // renovate: datasource=docker depName=debian
  default = "bookworm-slim"
}

variable "YQ_VERSION" {
  // renovate: datasource=github depName=mikefarah/yq
  default = "4.45.4"
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
    YQ_VERSION = "${YQ_VERSION}"
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
