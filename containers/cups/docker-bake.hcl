variable "APP" {
  default = "cups"
}

variable "UBUNTU_VERSION" {
  // renovate: datasource=docker depName=ubuntu
  default = "24.04"
}

variable "REGISTRY" {
  default = "ghcr.io/solanyn"
}

variable "SOURCE" {
  default = "https://github.com/OpenPrinting/cups"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    UBUNTU_VERSION = "${UBUNTU_VERSION}"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
    "org.opencontainers.image.title" = "${APP}"
    "org.opencontainers.image.version" = "${UBUNTU_VERSION}"
  }
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
  tags = ["${APP}:${UBUNTU_VERSION}", "${APP}:latest"]
}

target "image-all" {
  inherits = ["image"]
  output = ["type=registry"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
  tags = [
    "${REGISTRY}/${APP}:${UBUNTU_VERSION}",
    "${REGISTRY}/${APP}:latest"
  ]
}
