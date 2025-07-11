target "docker-metadata-action" {}

variable "APP" {
  default = "calibre"
}

variable "CALIBRE_VERSION" {
  // renovate: datasource=github-releases depName=kovidgoyal/calibre
  default = "8.6.0"
}

variable "DEBIAN_VERSION" {
  // renovate: datasource=docker depName=debian
  default = "bookworm-slim"
}

variable "VERSION" {
  default = "${CALIBRE_VERSION}"
}

variable "SOURCE" {
  default = "https://github.com/kovidgoyal/calibre"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    CALIBRE_VERSION = "${CALIBRE_VERSION}"
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
