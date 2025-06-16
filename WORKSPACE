workspace(name = "goyangi")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Rules for C/C++
http_archive(
    name = "rules_cc",
    urls = ["https://github.com/bazelbuild/rules_cc/releases/download/0.1.1/rules_cc-0.1.1.tar.gz"],
    sha256 = "712d77868b3152dd618c4d64faaddefcc5965f90f5de6e6dd1d5ddcd0be82d42",
    strip_prefix = "rules_cc-0.1.1",
)

# Rules for Go
http_archive(
    name = "io_bazel_rules_go",
    sha256 = "d93ef02f1e72c82d8bb3d5169519b36167b33cf68c252525e3b9d3d5dd143de7",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.49.0/rules_go-v0.49.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.49.0/rules_go-v0.49.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "32938bda16e6700063035479063d9d24c60eda8d79fd4739563f50d331cb3209",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.35.0/bazel-gazelle-v0.35.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.35.0/bazel-gazelle-v0.35.0.tar.gz",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("//:deps.bzl", "go_dependencies")

# gazelle:repository_macro deps.bzl%go_dependencies
go_dependencies()

go_rules_dependencies()

go_register_toolchains(version = "1.24.2")

gazelle_dependencies()

# TODO: Add JavaScript/TypeScript rules later
# Note: rules_nodejs ecosystem has been restructured; need to use rules_js/rules_ts from Aspect
# Current attempt commented out due to configuration complexity

# Rules for Python
http_archive(
    name = "rules_python",
    sha256 = "62ddebb766b4d6ddf1712f753dac5740bea072646f630eb9982caa09ad8a7687",
    strip_prefix = "rules_python-0.39.0",
    urls = [
        "https://github.com/bazelbuild/rules_python/releases/download/0.39.0/rules_python-0.39.0.tar.gz",
    ],
)

load("@rules_python//python:repositories.bzl", "py_repositories", "python_register_toolchains")

py_repositories()

python_register_toolchains(
    name = "python3_11",
    python_version = "3.11",
)

# TODO: Add Rust support later (blocked on edition2024 compatibility)
# Rules for Rust
# http_archive(
#     name = "rules_rust",
#     integrity = "sha256-U8G6x+xI985IxMHGqgBvJ1Fa3SrrBXJZNyJObgDsfOo=",
#     urls = ["https://github.com/bazelbuild/rules_rust/releases/download/0.61.0/rules_rust-0.61.0.tar.gz"],
# )
#
# load("@rules_rust//rust:repositories.bzl", "rules_rust_dependencies", "rust_register_toolchains")
#
# rules_rust_dependencies()
#
# rust_register_toolchains(
#     edition = "2021",
#     versions = ["1.80.0"],
# )
#
# load("@rules_rust//crate_universe:repositories.bzl", "crate_universe_dependencies")
#
# crate_universe_dependencies()
#
# load("@rules_rust//crate_universe:defs.bzl", "crates_repository")
#
# crates_repository(
#     name = "crate_index",
#     cargo_lockfile = "//gt7/telemetry-server:Cargo.lock",
#     lockfile = "//gt7/telemetry-server:Cargo.Bazel.lock",
#     manifests = ["//gt7/telemetry-server:Cargo.toml"],
# )
#
# load("@crate_index//:defs.bzl", "crate_repositories")
#
# crate_repositories()

# Protocol buffer support is provided by rules_go