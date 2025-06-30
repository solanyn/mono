variable "APP" {
  default = "mysql-init"
}

variable "ALPINE_VERSION" {
  // renovate: datasource=docker depName=alpine
  default = "3.22"
}

variable "MYSQL_CLIENT_VERSION" {
  // renovate: datasource=repology depName=alpine_3_22/mysql-client versioning=loose
  default = "11.4.5-r2"
}

variable "VERSION" {
  // renovate: datasource=docker depName=mysql versioning=semver updateType=major
  default = "8"
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
