variable "APP" {
  default = "airflow"
}

variable "AIRFLOW_TAG" {
  // renovate: datasource=docker depName=apache/airflow
  default = "3.0.1"
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
    "org.opencontainers.image.title" = "${APP}"
    "org.opencontainers.image.version" = "${AIRFLOW_TAG}"
  }
}

target "image-local" {
  inherits = ["image"]
  output = ["type=docker"]
  tags = ["${APP}:${AIRFLOW_TAG}", "${APP}:latest"]
}

target "image-all" {
  inherits = ["image"]
  output = ["type=registry"]
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
  tags = [
    "${REGISTRY}/${APP}:${AIRFLOW_TAG}",
    "${REGISTRY}/${APP}:latest"
  ]
}
