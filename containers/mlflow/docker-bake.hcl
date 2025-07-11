target "docker-metadata-action" {}

variable "APP" {
  default = "mlflow"
}

variable "MLFLOW_VERSION" {
  // renovate: datasource=pypi depName=mlflow
  default = "3.1.1"
}

variable "VERSION" {
  default = "${MLFLOW_VERSION}"
}

variable "SOURCE" {
  default = "https://github.com/mlflow/mlflow"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    VERSION = "${MLFLOW_VERSION}"
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