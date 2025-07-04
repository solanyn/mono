# Agent Documentation for `goyangi`

## Agent Guidelines

- Do not attribute agents in commit messages
- Always use `gh` CLI to interact with GitHub
- PRs must be linked to an issue. Open one if there is no relevant issue.
- Documentation is always informative and technical in detail
- Do not add unnecessary information
- Do not use emojis

## Project Overview

Goyangi is a polyglot monorepo using Bazel with Bzlmod for build management and hybrid Nix toolchains for hermetic cross-compilation. The project includes:

- **Frontend**: SvelteKit application (TypeScript)
- **Backend**: Rust services
- **Python**: ML/data processing components with JAX support

## Build System

### Prerequisites

Install Nix before development:

```bash
curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install
```

### Bazel with Aspect CLI

All bazel commands use bazelisk (which uses Aspect CLI):

```bash
bazelisk build //...
bazelisk test //...
bazelisk run //target:name
```

**Configuration Files:**

- `.bazeliskrc` - Configures bazelisk to use Aspect CLI
- `MODULE.bazel` - Bzlmod dependencies
- `.bazelrc` - Bazel configuration options

Use Bazel to build all projects except Python projects. For Python projects, use Bazel to setup a common virtual environment with `bazelisk run //{project}:venv`.

Package using Dockerfiles and GitHub Actions Docker builds. Add dependencies to `requirements.txt` and `requirements_darwin.txt` files. Run `bazelisk run //{project}:requirements` to update lock files.

### Essential Commands

#### Build and Test

```bash
bazelisk build //...
bazelisk test //...
bazelisk build //tldr/frontend:svelte_app
bazelisk run //tldr/frontend:dev
bazelisk run //torches:venv
bazelisk run //jaxes:venv
bazelisk run //cronprint:venv
```

#### Build containers

```bash
./tools/containers/build.sh airflow
./tools/containers/build.sh calibre-web
./tools/containers/build.sh calibre
./tools/containers/build.sh cnpg
./tools/containers/build.sh cups
for container in containers/*/; do ./tools/containers/build.sh $(basename "$container"); done
```

**Container build requirements:**

- Docker Desktop running with buildx support
- Network access to pull base images
- For registry push: authentication to ghcr.io

**Container versioning:**

- Containers in `./containers/` use their main dependency version (e.g., Airflow version for airflow container)
- Applications built with Bazel use monorepo version from `./tools/get-version.sh`

#### Dependency Management

```bash
cd tldr/frontend && npm update
```

## Language-Specific Guidelines

### TypeScript/JavaScript (Frontend)

- **Framework**: SvelteKit with Vite
- **Package Manager**: npm (with Bazel npm rules)

```bash
bazelisk run //tldr/frontend:dev
bazelisk run //tldr/frontend:build
```

### Rust

- **Build System**: Bazel with rules_rust
- **Dependency Management**: Cargo.toml with Bazel crate_universe

```bash
bazelisk build //tldr/backend:tldr-backend
bazelisk build //gt7/telemetry:telemetry_server
```

### Python

- **Build System**: Bazel with `rules_uv` to build virtual environments
- **Dependency Management**: requirements.txt with rules_uv

```bash
bazelisk run //torches:requirements
bazelisk run //jaxes:requirements
bazelisk run //cronprint:requirements
```

## Development Workflow

### After Making Changes

```bash
bazelisk test //...
bazelisk build //...
```

### CI/CD Integration

The project uses GitHub Actions with Nix and bazelisk:

- **Nix**: Installed via `cachix/install-nix-action@v25` for hermetic toolchains
- **Bazel**: All commands use `bazel` but this runs through bazelisk → Aspect CLI
- **Cross-compilation**: Linux x86_64 and ARM64 targets supported via Nix

#### GitHub Workflows

**Bazel Build (`bazel.yaml`)**

- Builds and tests all targets with `//...`
- Builds container images using Bazel OCI targets
- Tags containers with both service-specific and monorepo versions
- Pushes to GHCR on main branch commits

**Container Build (`containers.yaml`)**

- Auto-discovers all containers in `./containers/` directory
- Builds only changed containers on PRs/pushes
- Supports manual dispatch for specific containers or all containers
- Multi-platform builds (linux/amd64, linux/arm64) with Docker Buildx
- Uses unified build script `./tools/containers/build.sh`

**Other Workflows**

- `renovate.yaml` - Dependency updates via Renovate bot
- `website.yaml` - Builds and deploys Quartz documentation site

## Configuration Files

### `.bazeliskrc`

```
BAZELISK_BASE_URL=https://github.com/aspect-build/aspect-cli/releases/download
USE_BAZEL_VERSION=aspect/2025.11.0
```

## Troubleshooting

### Common Issues

**"No repository visible as '@repo_name'"**

- Check MODULE.bazel for missing dependencies
- Ensure Bzlmod is enabled in .bazelrc

**Build failures after adding new files**

- Check that new dependencies are added to MODULE.bazel
- Ensure BUILD files include new targets

### Debug Commands

```bash
bazelisk query //...
bazelisk query //tldr/frontend:all
bazelisk query "deps(//target:name)"
bazelisk info
```

## Best Practices

- Always use `bazelisk` instead of `bazel` directly
- Test changes with `bazelisk test //...` before submitting
- Check CI compatibility - all commands should work in GitHub Actions
- Use `.yaml` extension for all YAML files (not `.yml`)
- Update this documentation when adding new tools or workflows
- Use Plan Mode + ultrathink when making TODOs.

## External Dependencies

- **Nix**: Hermetic cross-compilation toolchains for system dependencies
- **Aspect CLI**: Enhanced Bazel experience
- **Bzlmod**: Modern Bazel dependency management (preferred over WORKSPACE)
- **rules_js**: JavaScript/TypeScript support
- **rules_rust**: Rust language support with cross-compilation targets
- **aspect_rules_py**: Python support with modern tooling
- **rules_nixpkgs_core**: Nix integration for hermetic builds

## Support

For questions about this setup:

1. Check the Aspect CLI documentation: <https://docs.aspect.build/cli>
2. Consult Bazel Bzlmod guide: <https://bazel.build/external/migration>
3. See the specific Bazel ruleset documentation
