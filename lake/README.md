# lake

Medallion architecture data lake. Ingests Australian macro-financial data into S3 (Garage), promotes through bronze/silver/gold layers via Redpanda events.

```mermaid
graph LR
    sources["Data Sources<br/>RBA, ABS, AEMO,<br/>RSS, Reddit,<br/>Domain, NSW VG"] --> ingest
    subgraph lake["lake (Go)"]
        ingest --> promote --> aggregate
    end
    subgraph s3["S3 (Garage)"]
        bronze --> silver --> gold
    end
    ingest -->|write| bronze
    promote -->|write| silver
    aggregate -->|write| gold
    redpanda["Redpanda"] -.->|events| promote
    redpanda -.->|events| aggregate
    ingest -.->|publish| redpanda
    mcp["mcp/ (Python)"] -->|read| silver
    mcp -->|read| gold
```

## Services

| Target | What |
|--------|------|
| `//lake:ingest` | Cron-based data ingestion from external sources |
| `//lake:promote` | Event-driven bronze → silver promotion |
| `//lake:aggregate` | Event-driven silver → gold aggregation |
| `mcp/` | MCP server exposing data to AI agents (FastMCP + Pydantic AI) |

## Build

```bash
bazel build //lake:ingest //lake:promote //lake:aggregate
```
