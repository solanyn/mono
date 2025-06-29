target "docker-metadata-action" {}

variable "APP" {
  default = "airflow"
}

variable "AIRFLOW_VERSION" {
  // renovate: datasource=docker depName=apache/airflow
  default = "3.0.2"
}

variable "REGISTRY" {
  default = "ghcr.io/solanyn"
}

variable "SOURCE" {
  default = "https://github.com/apache/airflow"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  args = {
    AIRFLOW_TAG = "${AIRFLOW_TAG}"
  }
  labels = {
    "org.opencontainers.image.source" = "${SOURCE}"
  }
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
  tags = ["${APP}:${AIRFLOW_TAG}"]
}

target "image-all" {
  inherits = ["image"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
}
