# Agent Documentation for Goyangi Project

This document provides essential information for AI agents working on the Goyangi project.

## Project Overview

Goyangi is a polyglot monorepo using Bazel with Bzlmod for build management and hybrid Nix toolchains for hermetic cross-compilation. The project includes:

- **Frontend**: SvelteKit application (TypeScript)
- **Backend**: Rust services
- **Python**: ML/data processing components with PyTorch GPU support
- **Go**: Various utilities and services
- **GT7 Telemetry**: Real-time racing telemetry streaming via Kafka

## Build System

### Prerequisites

**Nix Installation Required**: This project uses hybrid Nix+Bazel toolchains for hermetic cross-compilation. Install Nix before development:

```bash
# macOS/Linux
curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install
```

### Bazel with Aspect CLI

The project uses **Aspect CLI** instead of standard Bazel for enhanced developer experience:

```bash
# All bazel commands should use bazelisk (which uses Aspect CLI)
bazelisk build //...
bazelisk test //...
bazelisk run //target:name
```

**Key Configuration Files:**

- `.bazeliskrc` - Configures bazelisk to use Aspect CLI
- `MODULE.bazel` - Bzlmod dependencies (preferred over WORKSPACE)
- `.bazelrc` - Bazel configuration options

### Essential Commands for Agents

#### Build and Test

```bash
# Build all targets
bazelisk build //...

# Run all tests
bazelisk test //...

# Build specific target
bazelisk build //tldr/frontend:svelte_app

# Run development server
bazelisk run //tldr/frontend:dev
```

#### Code Quality and Formatting

```bash
# Format all files in the repository
bazelisk run //:format

# Auto-generate/update BUILD files (like gazelle)
bazelisk configure

# Show what configure would change without applying
bazelisk configure --mode=diff

# Print generated BUILD files to stdout
bazelisk configure --mode=print

# Lint all targets (when aspects are configured)
bazelisk lint //...
```

#### Dependency Management

```bash
# Update Python dependencies
bazelisk run //:requirements_cpu.update
bazelisk run //:requirements_gpu.update

# Update npm dependencies (run in frontend directory)
cd tldr/frontend && npm update
```

## Project Structure

```
goyangi/
├── .aspect/cli/config.yaml    # Aspect CLI configuration
├── .bazeliskrc               # Bazelisk configuration (uses Aspect CLI)
├── MODULE.bazel              # Bzlmod dependencies
├── BUILD.bazel               # Root build file with format target
├── tldr/                     # TLDR application
│   ├── frontend/             # SvelteKit frontend
│   │   ├── BUILD.bazel
│   │   ├── package.json
│   │   ├── eslint.config.js
│   │   └── src/
│   └── backend/              # Rust backend
│       ├── BUILD.bazel
│       ├── Cargo.toml
│       └── src/
├── gt7/                      # GT7 telemetry server
├── torches/                  # PyTorch utilities
└── tools/                    # Build tools and utilities
```

## Language-Specific Guidelines

### TypeScript/JavaScript (Frontend)

- **Framework**: SvelteKit with Vite
- **Package Manager**: npm (with Bazel npm rules)
- **Linting**: ESLint with TypeScript and Svelte plugins
- **Formatting**: Prettier (via rules_lint)

**Key Commands:**

```bash
# Development server
bazelisk run //tldr/frontend:dev

# Production build
bazelisk run //tldr/frontend:build
```

### Rust

- **Build System**: Bazel with rules_rust
- **Dependency Management**: Cargo.toml with Bazel crate_universe

**Key Commands:**

```bash
# Build Rust targets
bazelisk build //tldr/backend:tldr-backend
bazelisk build //gt7/telemetry:telemetry_server

# Run Rust formatting
bazelisk run @rules_rust//:rustfmt_test

# Run Clippy linting
bazelisk run @rules_rust//:clippy_test
```

### Python

- **Build System**: Bazel with aspect_rules_py
- **Dependency Management**: requirements.txt with rules_uv
- **Formatting**: Ruff (via rules_lint)
- **ML Framework**: PyTorch with CUDA 11.8 support (Linux) and CPU fallback (macOS)

**Key Commands:**

```bash
# Build Python targets
bazelisk build //torches:main

# Run PyTorch demo with GPU detection
bazelisk run //torches:main

# Update Python dependencies
bazelisk run //:requirements.update
```

### Go

- **Build System**: Bazel with rules_go
- **Dependency Management**: go.mod with Gazelle

## Development Workflow for Agents

### 1. Before Making Changes

```bash
# Update BUILD files for any new source files
bazelisk configure

# Check current formatting
bazelisk run //:format --mode=check
```

### 2. After Making Changes

```bash
# Auto-generate BUILD files for new code
bazelisk configure

# Format all code
bazelisk run //:format

# Run tests
bazelisk test //...

# Build everything to ensure no breakage
bazelisk build //...
```

### 3. CI/CD Integration

The project uses GitHub Actions with Nix and bazelisk:

- **Nix**: Installed via `cachix/install-nix-action@v25` for hermetic toolchains
- **Bazel**: All commands use `bazel` but this runs through bazelisk → Aspect CLI
- **Cross-compilation**: Linux x86_64 and ARM64 targets supported via Nix

## Configuration Files

### `.aspect/cli/config.yaml`

```yaml
lint:
  aspects:
    - "@aspect_rules_lint//lint:eslint.bzl%eslint"
    - "@aspect_rules_lint//lint:ruff.bzl%ruff"

configure:
  languages:
    javascript: true
    go: true
    python: true
    protobuf: true
    bzl: true
    rust: true
```

### `.bazeliskrc`

```
BAZELISK_BASE_URL=https://github.com/aspect-build/aspect-cli/releases/download
USE_BAZEL_VERSION=aspect/2025.11.0
```

## Troubleshooting

### Common Issues

1. **"No repository visible as '@repo_name'"**

   - Check MODULE.bazel for missing dependencies
   - Ensure Bzlmod is enabled in .bazelrc

2. **Build failures after adding new files**

   - Run `bazelisk configure` to update BUILD files
   - Check that new dependencies are added to MODULE.bazel

3. **Formatting not working**
   - Ensure rules_lint is properly configured in MODULE.bazel
   - Check that multitool extension is set up correctly

### Useful Debug Commands

```bash
# Check what targets are available
bazelisk query //...

# Check specific package targets
bazelisk query //tldr/frontend:all

# Analyze dependency graph
bazelisk query "deps(//target:name)"

# Check Bazel info
bazelisk info
```

## Best Practices for Agents

1. **Always use `bazelisk`** instead of `bazel` directly
2. **Run `bazelisk configure`** after adding new source files
3. **Use `bazelisk run //:format`** before committing changes
4. **Test changes with `bazelisk test //...`** before submitting
5. **Check CI compatibility** - all commands should work in GitHub Actions
6. **Update this documentation** when adding new tools or workflows

## External Dependencies

- **Nix**: Hermetic cross-compilation toolchains for system dependencies
- **Aspect CLI**: Enhanced Bazel experience with additional commands
- **rules_lint**: Unified linting and formatting across languages
- **Bzlmod**: Modern Bazel dependency management (preferred over WORKSPACE)
- **rules_js**: JavaScript/TypeScript support
- **rules_rust**: Rust language support with cross-compilation targets
- **aspect_rules_py**: Python support with modern tooling
- **rules_nixpkgs_core**: Nix integration for hermetic builds

## Support

For questions about this setup:

1. Check the Aspect CLI documentation: <https://docs.aspect.build/cli>
2. Review rules_lint documentation: <https://github.com/aspect-build/rules_lint>
3. Consult Bazel Bzlmod guide: <https://bazel.build/external/migration>
