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
  default = "8.5.0"
}

variable "CALIBRE_WEB_VERSION" {
  // renovate: datasource=github-releases depName=janeczku/calibre-web
  default = "0.6.24"
}

variable "UBUNTU_VERSION" {
  // renovate: datasource=docker depName=ubuntu
  default = "24.04"
}

variable "REGISTRY" {
  default = "ghcr.io/solanyn"
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
    UBUNTU_VERSION = "${UBUNTU_VERSION}"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
    "org.opencontainers.image.title" = "${APP}"
    "org.opencontainers.image.version" = "${CALIBRE_WEB_VERSION}"
  }
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
  tags = ["${APP}:${CALIBRE_VERSION}"]
}

target "image-all" {
  inherits = ["image"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
}
