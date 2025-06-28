variable "APP" {
  default = "calibre"
}

variable "KEPUBIFY_VERSION" {
  // renovate: datasource=github-releases depName=pgaskin/kepubify
  default = "4.0.4"
}

variable "CALIBRE_VERSION" {
  // renovate: datasource=github-releases depName=kovidgoyal/calibre
  default = "8.5.0"
}

variable "UBUNTU_VERSION" {
  // renovate: datasource=docker depName=ubuntu
  default = "24.04"
}

variable "REGISTRY" {
  default = "ghcr.io/solanyn"
}

variable "SOURCE" {
  default = "https://github.com/kovidgoyal/calibre"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    KEPUBIFY_VERSION = "${KEPUBIFY_VERSION}"
    CALIBRE_VERSION = "${CALIBRE_VERSION}"
    UBUNTU_VERSION = "${UBUNTU_VERSION}"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
    "org.opencontainers.image.title" = "${APP}"
    "org.opencontainers.image.version" = "${CALIBRE_VERSION}"
  }
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
  tags = ["${APP}:${CALIBRE_VERSION}", "${APP}:latest"]
}

target "image-all" {
  inherits = ["image"]
  output = ["type=registry"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
  tags = [
    "${REGISTRY}/${APP}:${CALIBRE_VERSION}",
    "${REGISTRY}/${APP}:latest"
  ]
}
