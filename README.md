# mono

Monorepo built with Bazel. Medallion architecture data lake (bronze/silver/gold S3 buckets) with event-driven promotion via Redpanda.

## Projects

| Project | Description | Language |
|---|---|---|
| [lake](./lake/) | Data lake ingest service — RBA, ABS, AEMO, RSS, Reddit, Domain, NSW VG | Go |
| [lake-mcp](./lake-mcp/) | MCP server for querying the data lake. Pydantic AI agent with macro-financial reasoning | Python |
| [yield](./yield/) | Property analysis tool — rent fairness, investment tracking, listing analysis | Go, React |
| [libs/datalake](./libs/datalake/) | Shared S3 parquet read/write library | Python |
| [cronprint](./cronprint/) | Scheduled printing service using IPP | Go |
| [website](./website/) | Personal blog built with Astro | TypeScript |

## Architecture

```
lake (Go)                    Redpanda                     S3 (Garage)
┌──────────┐  bronze.written  ┌──────────┐                ┌─────────┐
│  ingest  │ ──────────────► │          │                │ bronze  │
│ (cron)   │                 │          │                │ silver  │
└──────────┘                 │          │                │ gold    │
                             │          │  ┌──────────┐  └─────────┘
                             │          │──│ promote  │──► silver
                             │          │  └──────────┘
                             │          │  ┌──────────┐
                             │          │──│aggregate │──► gold
                             └──────────┘  └──────────┘

lake-mcp (Python) ──reads──► silver/gold
```

## Build

```bash
bazel build //...
bazel test //...

bazel build //lake:ingest //lake:promote //lake:aggregate
bazel build //lake-mcp:mcp_server
bazel build //yield/api:api
```

## Deploy

Container images built via `rules_oci` and pushed to GHCR. Deployed to k8s via FluxCD + app-template Helm chart.

```bash
bazel run //lake:push_ingest -- --tag latest
bazel run //lake:push_promote -- --tag latest
bazel run //lake:push_aggregate -- --tag latest
bazel run //lake-mcp:push -- --tag latest
```
