load("@rules_python//python:defs.bzl", "py_library")

package(default_visibility = ["//visibility:public"])

py_library(
    name = "pkg",
    srcs = glob(["**/*.py"]),
    data = glob([
        "**/*.so*",
        "**/*.dll",
        "**/*.dylib",
        "**/*.pyd",
        "**/*.ptx",
        "**/*.cubin",
    ]),
    imports = ["."],
)