"""Macro for lake services: builds a Go binary, packages it, builds an OCI image, and defines a push target."""

load("@rules_go//go:def.bzl", "go_binary")
load("@rules_oci//oci:defs.bzl", "oci_image", "oci_push")
load("@rules_pkg//pkg:tar.bzl", "pkg_tar")

def lake_service(name, srcs, deps, repository, base = "@distroless_static_linux_amd64", goos = "linux", goarch = "amd64"):
    """Define a lake service with binary, tar layer, OCI image, and push target.

    Args:
      name: Base name (e.g. "ingest"). Creates targets :{name}, :{name}_layer, :{name}_image, :push_{name}.
      srcs: Source files for the go_binary.
      deps: Dependencies for the go_binary.
      repository: OCI repository (e.g. "ghcr.io/solanyn/lake-ingest").
      base: Base OCI image.
      goos: Target OS.
      goarch: Target arch.
    """
    go_binary(
        name = name,
        srcs = srcs,
        goos = goos,
        goarch = goarch,
        pure = "on",
        deps = deps,
        visibility = ["//visibility:public"],
    )

    pkg_tar(
        name = name + "_layer",
        srcs = [":" + name],
        strip_prefix = ".",
    )

    oci_image(
        name = name + "_image",
        base = base,
        entrypoint = ["/" + name + "_/" + name],
        tars = [":" + name + "_layer"],
    )

    oci_push(
        name = "push_" + name,
        image = ":" + name + "_image",
        repository = repository,
    )
