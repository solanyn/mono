variable "APP" {
  default = "mysql-init"
}

variable "DEBIAN_VERSION" {
  // renovate: datasource=docker depName=debian
  default = "bookworm-slim"
}

variable "MYSQL_CLIENT_VERSION" {
  // renovate: datasource=repology depName=debian_12/mysql-client versioning=loose
  default = "8.0"
}

variable "VERSION" {
  // renovate: datasource=docker depName=mysql versioning=semver updateType=major
  default = "8"
}

variable "SOURCE" {
  default = "https://github.com/solanyn/goyangi"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    DEBIAN_VERSION = "${DEBIAN_VERSION}"
    MYSQL_CLIENT_VERSION = "${MYSQL_CLIENT_VERSION}"
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
