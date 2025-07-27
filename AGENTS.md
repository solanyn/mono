# Agent Documentation

## Projects

Build polyglot services using Bazel with Bzlmod and Nix toolchains:

- **tldr**: News aggregation (SvelteKit + Rust)
- **cronprint**: Scheduled printing (Python + CUPS)
- **gt7**: GT7 telemetry bridge (Rust + Pulsar)
- **jaxes**: JAX ML components (Python + GPU)
- **torches**: PyTorch ML components (Python + CUDA)
- **chorus-rs**: Speaker diarization API (Rust, design phase)

## Build Patterns

**TypeScript/JavaScript**:
- Check `package.json` for dependencies
- Use `bazel run //project:dev` for development
- Use `bazel build //project:build` for production

**Rust**:
- Check `Cargo.toml` for dependencies
- Use `bazel run //project:binary_name` to run
- Use `bazel build //project:image` for containers

**Python**:
- Check `pyproject.toml` and `requirements-*.lock` for dependencies
- Use `bazel run //project:venv` for virtual environments
- Use `bazel build //project:image` for containers

## Repository Structure

- Look for `design/` folders for technical context
- Check `tests.yaml` for container validation requirements
- Expect `BUILD.bazel` files for all buildable targets

## Build System

Alias bazel to bazelisk for all builds:

```bash
alias bazel=bazelisk
bazel build //...
bazel test //...
bazel run //target:name
```

## Essential Commands

**Build and Test**:
```bash
bazel build //...
bazel test //...
```

**Development**:
```bash
bazel run //project:dev          # Frontend development
bazel run //project:binary_name  # Run Rust binaries
bazel run //project:venv         # Python virtual environments
```

**Containers**:
```bash
bazel build //project:image
./tools/build-linux.sh build //project:image  # Cross-platform builds
```

## Configuration Files

- `.bazeliskrc` - Aspect CLI configuration
- `MODULE.bazel` - Bzlmod dependencies
- `.bazelrc` - Bazel build options

## Development Workflow

1. Alias bazel to bazelisk: `alias bazel=bazelisk`
2. Test with `bazel test //...` before submitting
3. Check container builds with `bazel build //project:image`
4. Use `./tools/build-linux.sh` for cross-platform containers

## External Dependencies

- **LLVM toolchain**: Hermetic C++/Rust builds
- **Aspect CLI**: Enhanced Bazel experience
- **Bzlmod**: Modern dependency management
- **rules_js**: JavaScript/TypeScript support
- **rules_rust**: Rust language support
- **aspect_rules_py**: Python support
