# Agent Documentation

## Projects

- **cronprint**: Scheduled printing service (Go + IPP)

## Build Patterns

**Go**:
- Check `go.mod` for dependencies
- Use `bazel build //cronprint:cronprint` to build
- Use `bazel run //cronprint:cronprint` to run

## Repository Structure

- Check `BUILD.bazel` files for all buildable targets
- `tools/` contains build tooling scripts

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

**Containers**:
```bash
bazel build //cronprint:image
```

## Configuration Files

- `.bazeliskrc` - Aspect CLI configuration
- `MODULE.bazel` - Bzlmod dependencies
- `.bazelrc` - Bazel build options

## Development Workflow

1. Alias bazel to bazelisk: `alias bazel=bazelisk`
2. Test with `bazel test //...` before submitting
3. Check container builds with `bazel build //cronprint:image`

## External Dependencies

- **Aspect CLI**: Enhanced Bazel experience
- **Bzlmod**: Modern dependency management
- **rules_go**: Go language support
- **gazelle**: Go BUILD file generation
- **rules_oci**: Container image builds
