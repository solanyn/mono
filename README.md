# Goyangi

Polyglot monorepo using Bazel with Bzlmod for build management and hybrid Nix toolchains for hermetic cross-compilation.

## Projects

| Project | Description | Language |
|---------|-------------|----------|
| [tldr](./tldr/) | News aggregation service with SvelteKit frontend and Rust backend | TypeScript/Rust |
| [cronprint](./cronprint/) | Scheduled printing service using CUPS with health monitoring | Python |
| [gt7](./gt7/) | Gran Turismo 7 telemetry bridge streaming to Apache Pulsar | Rust |
| [jaxes](./jaxes/) | JAX ML components with platform-specific GPU support | Python |
| [torches](./torches/) | PyTorch ML components with CUDA/CPU optimization | Python |
| [chorus-rs](./chorus-rs/) | High-throughput speaker diarization API (design phase) | Rust |

## Build System

Uses Bazel with Aspect CLI for all builds:

```bash
# Alias bazel to bazelisk
alias bazel=bazelisk

# Build all projects
bazel build //...

# Run tests
bazel test //...

# Development servers
bazel run //tldr/frontend:dev
bazel run //cronprint:venv
bazel run //jaxes:venv
bazel run //torches:venv
```

**Configuration**: `.bazeliskrc` (Aspect CLI), `MODULE.bazel` (Bzlmod dependencies), `.bazelrc` (build options)

## Container Builds

```bash
# Build Linux containers (cross-platform)
./tools/build-linux.sh build //cronprint:image
./tools/build-linux.sh build //jaxes:image
./tools/build-linux.sh build //torches:image
./tools/build-linux.sh build //gt7/telemetry:image
```

## Architecture

- **TypeScript/JavaScript**: SvelteKit with Vite, npm + Bazel rules_js
- **Rust**: Bazel with rules_rust, LLVM toolchain for cross-compilation
- **Python**: Bazel with rules_uv, per-project virtual environments
- **Containers**: Multi-platform builds with OCI targets and structure testing

