# Custom BUILD file for triton package
# Triton is a language and compiler for writing highly efficient custom Deep-Learning primitives
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
        "**/*.ptx",  # CUDA PTX files
        "**/*.cubin",  # CUDA binary files
    ], allow_empty = True),
    imports = ["."],
    visibility = ["//visibility:public"],
)

# Alternative target name that might be expected
py_library(
    name = "triton",
    deps = [":pkg"],
    visibility = ["//visibility:public"],
)