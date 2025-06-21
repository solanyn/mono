# Custom BUILD file for nvidia_cusolver_cu12 package
# NVIDIA cuSOLVER - dense and sparse direct solvers for CUDA
# This is a stub BUILD file to handle missing BUILD file in the pip package

load("@aspect_rules_py//py:defs.bzl", "py_library")

py_library(
    name = "pkg",
    srcs = glob(["**/*.py"], allow_empty = True),
    data = glob([
        "**/*.so*",
        "**/*.dll", 
        "**/*.dylib",
        "**/*.pyd",
        "**/*.json",
        "**/*.txt",
        "**/*.whl",
    ], allow_empty = True),
    imports = ["."],
    visibility = ["//visibility:public"],
)

py_library(
    name = "nvidia_cusolver_cu12",
    deps = [":pkg"],
    visibility = ["//visibility:public"],
)