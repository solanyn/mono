# mono — Monorepo

Personal monorepo built with Bazel (Aspect CLI).

## Directory Structure

```
mono/
├── cronprint/          # Go — Scheduled printing service (IPP/CUPS)
├── kobo/               # Go — E-ink dashboard for Kobo e-reader
│   ├── dash-server/    #   Server: renders markdown → PNG
│   └── kobo-dash/      #   Client: writes PNG to Kobo framebuffer
├── lake/               # Data lake platform
│   ├── cmd/            #   Go services: ingest, promote, aggregate
│   ├── internal/       #   Shared Go packages
│   └── mcp/            #   Python MCP server (FastMCP + Pydantic AI)
├── libs/
│   └── datalake/       # Python — Shared S3 parquet read/write library
├── line/               # Go + TS — Market data platform
│   ├── cmd/            #   Services: api, assembler, capture, migrate, refdata, simulator
│   ├── web/            #   React frontend (Vite + TypeScript)
│   ├── proto/          #   gRPC protobuf definitions
│   ├── analytics/      #   Analytics code
│   └── internal/       #   Shared Go packages
├── resume/             # Typst — Resume/CV (data in JSON)
├── scrib/              # Meeting audio capture + processing
│   ├── audio/          #   macOS audio capture (ScreenCaptureKit, cgo)
│   ├── client/         #   HTTP client for server API
│   ├── cmd/            #   Server binary entry point
│   ├── server/         #   Go HTTP server (transcription orchestration)
│   ├── tui/            #   Bubble Tea TUI
│   └── ml/             #   Python ML pipeline (diarization, transcription)
├── tools/              # Build tooling (lint rules, remote toolchains, scripts)
├── website/            # TypeScript/Astro 6 — Personal blog (goyangi.io)
├── yield/              # Go + Python + TS — Property analysis tool
│   ├── api/            #   Go backend
│   ├── agent/          #   Python AI agent (Pydantic AI)
│   └── web/            #   React 19 frontend (Vite + TailwindCSS + CopilotKit)
└── patches/            # pnpm dependency patches
```

## Projects

| Dir | Lang | What |
|-----|------|------|
| `cronprint/` | Go | Scheduled printing service (IPP/CUPS) |
| `kobo/dash-server/` | Go | E-ink dashboard server (markdown → PNG) |
| `kobo/kobo-dash/` | Go | Kobo framebuffer client |
| `lake/` | Go | Data lake pipeline — RBA, ABS, AEMO, RSS, Reddit, Domain, NSW VG |
| `lake/mcp/` | Python | MCP server for data lake queries (FastMCP + Pydantic AI) |
| `libs/datalake/` | Python | Shared S3 parquet library (PyArrow, boto3, s3fs) |
| `line/` | Go + TS | Market data platform (gRPC, Kafka, Postgres, S3) |
| `resume/` | Typst | Resume/CV |
| `scrib/` | Go | Meeting audio capture, transcription & annotation (Bubble Tea TUI) |
| `scrib/ml/` | Python | ML audio pipeline — diarization (Senko), transcription (MLX Parakeet) |
| `website/` | Astro 6 | Personal blog (goyangi.io) |
| `yield/` | Go + Python + TS | Property analysis — rent fairness, investment, listings |

## Build System

- **Bazel** via Aspect CLI (`bazelisk`). Always `alias bazel=bazelisk`
- **MODULE.bazel** for bzlmod deps: `rules_go`, `gazelle`, `rules_oci`, `rules_js`, `rules_python`, `rules_uv`, `aspect_rules_lint`
- **pnpm** workspace for JS/TS packages (`website`, `yield/web`, `line/web`)
- Each project has its own `BUILD.bazel`

## Essential Commands

```bash
bazel build //...          # build everything
bazel test //...           # test everything
bazel run //target:name    # run a target
bazel run //:gazelle       # regenerate BUILD files for Go
```

## Go Conventions

- Per-project `go.mod` (no root `go.work`)
- Gazelle generates BUILD files: `bazel run //:gazelle`
- Build: `bazel build //project:target`
- Test: `bazel test //project/...`

## Python Conventions

- Use `uv` for venvs: `uv venv .venv && uv pip install -e libs/datalake`
- `pyproject.toml` with hatchling for packaging
- pytest for tests (not wired to Bazel)
- S3 config via env: `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_REGION`

## TypeScript Conventions

- pnpm workspace (root `pnpm-workspace.yaml`)
- Vite for dev/build
- TailwindCSS for styling
- `rules_js` for Bazel integration

## OCI Images

- Built with `rules_oci`: `bazel build //project:image`
- Push: `bazel run //project:push_image`
- Registry: `ghcr.io/solanyn/`
- Deployed via FluxCD to k8s

## CI

- GitHub Actions (`.github/workflows/`)
- `bazel.yaml` — Build + test all on push/PR, push OCI images on main merge
- `lake.yaml` — Lake-specific pipeline
- BuildBuddy remote cache
- Renovate for dependency updates (operator in k8s)

## Git

- Commit directly to main for personal work
- Conventional commits: `feat(project):`, `fix(project):`, `chore(project):`
- Author: `bot-goyangi[bot]` for automated commits
