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

# Rules for JavaScript/TypeScript (Aspect rules_js)
http_archive(
    name = "aspect_rules_js",
    sha256 = "83e5af4d17385d1c3268c31ae217dbfc8525aa7bcf52508dc6864baffc8b9501",
    strip_prefix = "rules_js-2.3.7",
    url = "https://github.com/aspect-build/rules_js/releases/download/v2.3.7/rules_js-v2.3.7.tar.gz",
)

load("@aspect_rules_js//js:repositories.bzl", "rules_js_dependencies")
rules_js_dependencies()

load("@aspect_rules_js//js:toolchains.bzl", "DEFAULT_NODE_VERSION", "rules_js_register_toolchains")
rules_js_register_toolchains(node_version = DEFAULT_NODE_VERSION)

load("@aspect_rules_js//npm:repositories.bzl", "npm_translate_lock")
npm_translate_lock(
    name = "npm",
    pnpm_lock = "//tldr/frontend:pnpm-lock.yaml",
    verify_node_modules_ignored = "//:.bazelignore",
)

load("@npm//:repositories.bzl", "npm_repositories")
npm_repositories()

# Rules for TypeScript (Aspect rules_ts)
http_archive(
    name = "aspect_rules_ts",
    sha256 = "6b15ac1c69f2c0f1282e41ab469fd63cd40eb2e2d83075e19b68a6a76669773f",
    strip_prefix = "rules_ts-3.6.0",
    url = "https://github.com/aspect-build/rules_ts/releases/download/v3.6.0/rules_ts-v3.6.0.tar.gz",
)

load("@aspect_rules_ts//ts:repositories.bzl", "rules_ts_dependencies")
rules_ts_dependencies(
    ts_version_from = "//tldr/frontend:package.json",
)

# Rules for Shell (required by rules_ts)
http_archive(
    name = "rules_shell",
    sha256 = "b15cc2e698a3c553d773ff4af35eb4b3ce2983c319163707dddd9e70faaa062d",
    strip_prefix = "rules_shell-0.5.0",
    url = "https://github.com/bazelbuild/rules_shell/releases/download/v0.5.0/rules_shell-v0.5.0.tar.gz",
)

load("@rules_shell//shell:repositories.bzl", "rules_shell_dependencies", "rules_shell_toolchains")
rules_shell_dependencies()
rules_shell_toolchains()

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

# Rules for Rust
http_archive(
    name = "rules_rust",
    integrity = "sha256-U8G6x+xI985IxMHGqgBvJ1Fa3SrrBXJZNyJObgDsfOo=",
    urls = ["https://github.com/bazelbuild/rules_rust/releases/download/0.61.0/rules_rust-0.61.0.tar.gz"],
)

load("@rules_rust//rust:repositories.bzl", "rules_rust_dependencies", "rust_register_toolchains")
load("@rules_rust//crate_universe:repositories.bzl", "crate_universe_dependencies")

rules_rust_dependencies()

rust_register_toolchains(
    edition = "2021", 
    versions = ["1.80.0"],
)

crate_universe_dependencies()

load("@rules_rust//crate_universe:defs.bzl", "crates_repository", "render_config")

crates_repository(
    name = "crate_index",
    cargo_lockfile = "//gt7/telemetry-server:Cargo.lock",
    lockfile = "//gt7/telemetry-server:Cargo.Bazel.lock",
    manifests = ["//gt7/telemetry-server:Cargo.toml"],
    render_config = render_config(
        default_package_name = ""
    ),
)

load("@crate_index//:defs.bzl", "crate_repositories")
crate_repositories()

# Rules for OCI containers
http_archive(
    name = "rules_oci",
    sha256 = "5994ec0e8df92c319ef5da5e1f9b514628ceb8fc5824b4234f2fe635abb8cc2e",
    strip_prefix = "rules_oci-2.2.6",
    url = "https://github.com/bazel-contrib/rules_oci/releases/download/v2.2.6/rules_oci-v2.2.6.tar.gz",
)

load("@rules_oci//oci:dependencies.bzl", "rules_oci_dependencies")
rules_oci_dependencies()

load("@rules_oci//oci:repositories.bzl", "oci_register_toolchains")
oci_register_toolchains(name = "oci")

load("@rules_oci//oci:pull.bzl", "oci_pull")

# Pull base images for different language ecosystems
oci_pull(
    name = "distroless_base",
    digest = "sha256:ccaef5ee2f1850270d453fdf700a5392534f8d1a8ca2acda391fbb6a06b81c86",
    image = "gcr.io/distroless/base",
    platforms = [
        "linux/amd64",
        "linux/arm64",
    ],
)

oci_pull(
    name = "distroless_java",
    digest = "sha256:161a1d97d592b3f1919801578c3a47c8e932071168a96267698f4b669c24c76d",
    image = "gcr.io/distroless/java17",
    platforms = [
        "linux/amd64", 
        "linux/arm64",
    ],
)

# Protocol buffer support is provided by rules_go