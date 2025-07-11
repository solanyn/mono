target "docker-metadata-action" {}

variable "APP" {
  default = "calibre-web"
}

variable "KEPUBIFY_VERSION" {
  // renovate: datasource=github-releases depName=pgaskin/kepubify
  default = "4.0.4"
}

variable "CALIBRE_VERSION" {
  // renovate: datasource=github-releases depName=kovidgoyal/calibre
  default = "8.6.0"
}

variable "CALIBRE_WEB_VERSION" {
  // renovate: datasource=github-releases depName=janeczku/calibre-web
  default = "0.6.24"
}

variable "DEBIAN_VERSION" {
  // renovate: datasource=docker depName=debian
  default = "bookworm-slim"
}

variable "VERSION" {
  default = "${CALIBRE_WEB_VERSION}"
}

variable "SOURCE" {
  default = "https://github.com/janeczku/calibre-web"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    CALIBRE_VERSION = "${CALIBRE_VERSION}"
    CALIBRE_WEB_VERSION = "${CALIBRE_WEB_VERSION}"
    KEPUBIFY_VERSION = "${KEPUBIFY_VERSION}"
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
