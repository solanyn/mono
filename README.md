# Goyangi

Polyglot monorepo using Bazel with Bzlmod for build management and hybrid Nix toolchains for hermetic cross-compilation.

## Architecture

- **Frontend**: SvelteKit application (TypeScript)
- **Backend**: Rust services with cross-compilation support
- **Python**: ML/data processing components with JAX and PyTorch support
- **Containers**: Multi-platform container builds with structure testing

## Prerequisites

Install Nix for hermetic cross-compilation:

```bash
curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install
```

## Build System

### Bazel with Aspect CLI

All commands use bazelisk configured to use Aspect CLI:

```bash
bazelisk build //...
bazelisk test //...
bazelisk run //target:name
```

**Configuration Files:**

- `.bazeliskrc` - Aspect CLI configuration
- `MODULE.bazel` - Bzlmod dependencies
- `.bazelrc` - Bazel build options

### Project Structure

```
├── tldr/frontend/          # SvelteKit TypeScript application
├── tldr/backend/           # Rust backend service
├── gt7/telemetry/          # GT7 telemetry server (Rust)
├── cronprint/              # Flask cron job printer (Python)
├── jaxes/                  # JAX ML components (Python)
├── torches/                # PyTorch ML components (Python)
└── containers/             # Multi-platform container definitions
```

## Development

### Build and Test

```bash
# Build all targets
bazelisk build //...

# Run all tests
bazelisk test //...

# Frontend development
bazelisk run //tldr/frontend:dev

# Python virtual environments
bazelisk run //cronprint:venv
bazelisk run //jaxes:venv
bazelisk run //torches:venv
```

### Container Development

```bash
# Build individual containers
./tools/containers/build.sh airflow
./tools/containers/build.sh calibre

# Build all containers
for container in containers/*/; do
  ./tools/containers/build.sh $(basename "$container")
done

# Bazel container builds
bazelisk run //gt7/telemetry:telemetry_push
bazelisk test //gt7/telemetry:telemetry_structure_test
```

## Language-Specific Details

### TypeScript/JavaScript

- **Framework**: SvelteKit with Vite
- **Package Manager**: npm with Bazel rules_js
- **Build**: `bazelisk run //tldr/frontend:build`

### Rust

- **Build System**: Bazel with rules_rust
- **Cross-compilation**: Linux x86_64 and ARM64 via Nix
- **Dependencies**: Cargo.toml with crate_universe

### Python

- **Build System**: Bazel with rules_uv
- **Virtual Environments**: Per-project with `bazelisk run //{project}:venv`
- **Dependencies**: requirements.txt with lock files via uv

## Container Infrastructure

### Build Requirements

- Docker Desktop with buildx support
- Network access for base images
- GHCR authentication for registry push

### Container Types

- **Dockerfile-based**: Multi-platform builds in `containers/`
- **Bazel OCI**: Native builds with `oci_push` targets
- **Structure Testing**: Automated validation for all containers

### Versioning Strategy

- **Containers**: Main dependency version (e.g., Airflow version)
- **Applications**: Monorepo version from `./tools/get-version.sh`

## CI/CD Integration

### GitHub Actions Workflows

**Bazel Build (`bazel.yaml`)**

- Builds and tests all targets with hermetic Nix toolchains
- Auto-discovers OCI targets with matrix strategy
- Container structure tests integration
- Registry push with `oci_push` targets

**Container Build (`containers.yaml`)**

- Auto-discovery of containers in `./containers/`
- Changed container detection for efficient builds
- Multi-platform builds (linux/amd64, linux/arm64)
- Container structure test validation

### Dependency Management

**Rust**: `Cargo.toml` with Bazel crate_universe
**JavaScript**: `package.json` with npm and Bazel rules_js  
**Python**: Per-project `requirements.txt` with uv compilation

## External Dependencies

- **Nix**: Hermetic cross-compilation toolchains
- **Aspect CLI**: Enhanced Bazel experience
- **Bzlmod**: Modern dependency management
- **rules_rust**: Rust support with cross-compilation
- **rules_js**: JavaScript/TypeScript support
- **aspect_rules_py**: Python support with modern tooling
- **rules_oci**: Container image builds and registry operations
- **container_structure_test**: Container validation testing

