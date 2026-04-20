# mono — Monorepo

## Projects

| Dir | Lang | What |
|-----|------|------|
| `cronprint/` | Go | Scheduled printing service (IPP) |
| `kobo/` | ? | Kobo e-reader tooling |
| `lake/` | Go | Data lake ingestion + MCP server |
| `lake-mcp/` | ? | Lake MCP integration |
| `libs/datalake/` | Python | Shared S3 data lake library (PyArrow, boto3, s3fs) |
| `macro/` | Python | Australian macro-financial data ingestion (RBA/ABS/AEMO) |
| `resume/` | ? | Resume/CV |
| `scrib/` | Go | Meeting audio capture, transcription & annotation |
| `website/` | Astro | Personal website (goyangi.io) |
| `yield/` | Go+Nix | Yield farming / DeFi tooling |

## Build System

- **Bazel** via Aspect CLI (bazelisk). Always `alias bazel=bazelisk`
- **MODULE.bazel** for bzlmod deps. `rules_go`, `gazelle`, `rules_oci`
- Each project has its own `BUILD.bazel`
- Go projects: `bazel build //project:target`, `bazel test //project/...`
- Python projects: `uv` for local dev, pytest for tests (not wired to Bazel)
- OCI images: `bazel build //project:image` or `bazel run //project:push_image`

## Essential Commands

```bash
bazel build //...          # build everything
bazel test //...           # test everything
bazel run //target:name    # run a target
```

## Go Conventions

- Check `go.mod` in each Go project for deps
- Gazelle generates BUILD files: `bazel run //:gazelle`
- Single root `go.work` if present, otherwise per-project `go.mod`

## Python Conventions

- Use `uv` for venvs: `uv venv .venv && uv pip install -e libs/datalake`
- S3 config via env: `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_REGION`

## CI

- GitHub Actions per-project (`.github/workflows/`)
- OCI push on main merge, build on PR
- Renovate handles dependency updates (operator in k8s, not GitHub Actions)

## Git

- Commit directly to main for personal work
- Conventional commits: `feat(project):`, `fix(project):`, `chore(project):`
- Author: `bot-goyangi[bot]` for automated commits
