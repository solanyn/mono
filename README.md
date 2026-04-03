# mono

Personal monorepo built with Bazel.

## Projects

| Project | Description |
|---|---|
| [lake](./lake/) | Data lake ingest/promote/aggregate pipeline — RBA, ABS, AEMO, RSS, Reddit, Domain, NSW VG |
| [lake-mcp](./lake-mcp/) | MCP server for querying the data lake with Pydantic AI |
| [yield](./yield/) | Property analysis tool — rent fairness, investment tracking, listing analysis |
| [libs/datalake](./libs/datalake/) | Shared S3 parquet read/write library |
| [kobo](./kobo/) | E-ink dashboard for Kobo e-reader — server renders markdown to PNG, client writes to framebuffer |
| [cronprint](./cronprint/) | Scheduled printing service using IPP |
| [website](./website/) | Personal blog built with Astro |
