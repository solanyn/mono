target "docker-metadata-action" {}

variable "APP" {
  default = "torches"
}

variable "VERSION" {
  default = "latest"
}

variable "SOURCE" {
  default = "https://github.com/solanyn/goyangi"
}

group "default" {
  targets = ["image-local"]
}

target "image" {
  # Use workspace root as context to access source code in torches/ directory
  context = "../.."
  dockerfile = "./containers/torches/Dockerfile"
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