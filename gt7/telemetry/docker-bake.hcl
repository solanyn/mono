variable "GITHUB_ACTOR" {
  default = "solanyn"
}

variable "GITHUB_REPOSITORY" {
  default = "solanyn/goyangi"
}

variable "VERSION" {
  default = "latest"
}

variable "TARGET" {
  default = "gt7-telemetry"
}

variable "REPOSITORY_SUBPATH" {
  default = "gt7/telemetry"
}

group "default" {
  targets = ["app"]
}

target "app" {
  tags = [
    "ghcr.io/${GITHUB_ACTOR}/${TARGET}:latest",
    "ghcr.io/${GITHUB_ACTOR}/${TARGET}:${VERSION}"
  ]
  labels = {
    "org.opencontainers.image.title" = "${TARGET}"
    "org.opencontainers.image.url" = "https://ghcr.io/${GITHUB_ACTOR}/${TARGET}"
    "org.opencontainers.image.description" = "GT7 telemetry server with WebSocket and Kafka support"
    "org.opencontainers.image.version" = "${VERSION}"
    "org.opencontainers.image.source" = "https://github.com/${GITHUB_REPOSITORY}/tree/main/${REPOSITORY_SUBPATH}"
  }
}
