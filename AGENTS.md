# Agent Documentation

## Projects

- **cronprint**: Scheduled printing service (Go + IPP)
- **libs/datalake**: Shared S3 data lake library (Python, PyArrow, boto3, s3fs)
- **macro**: Australian macro-financial data ingestion (Python, RBA/ABS/AEMO)

## Build Patterns

**Go**:
- Check `go.mod` for dependencies
- Use `bazel build //cronprint:cronprint` to build
- Use `bazel run //cronprint:cronprint` to run

**Python (datalake/macro)**:
- Use `uv` for local dev: `uv venv .venv && uv pip install -e libs/datalake -e "macro[dev]"`
- Run ingest CLI: `python -m macro.ingest.cli rba|abs|aemo|rss|promote-rba|promote-abs|promote-aemo|promote-rss|all`
- Run tests: `python -m pytest libs/datalake/tests/ macro/tests/ -v`
- Bazel build: `bazel build //libs/datalake/... //macro/...`
- Bazel tests not wired (network-dependent tests + pytest not in pip lock); use pytest directly
- S3 config via env vars: `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_REGION`

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
