# Custom BUILD file for nvidia_nvtx_cu12 package
# NVIDIA Tools Extension for CUDA - provides profiling and debugging tools
# This is a stub BUILD file to handle missing BUILD file in the pip package

load("@aspect_rules_py//py:defs.bzl", "py_library")

# Main package target (matches what rules_python expects)
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

# Alternative target name that might be expected
py_library(
    name = "nvidia_nvtx_cu12",
    deps = [":pkg"],
    visibility = ["//visibility:public"],
)