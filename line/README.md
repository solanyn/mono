# line

Real-time market data platform. Captures, assembles, and serves market data via Kafka, Postgres, S3, and gRPC.

```mermaid
graph LR
    feed["Market Feed"] --> capture
    subgraph services["Go Services"]
        capture --> kafka["Kafka"]
        kafka --> assembler
        assembler --> pg["Postgres"]
        assembler --> s3["S3 (Parquet)"]
        simulator --> kafka
        refdata --> pg
    end
    api --> pg
    api --> s3
    api -->|gRPC + WS| web["React Frontend<br/>(Three.js 3D viz)"]
```

## Services

| Service | What |
|---------|------|
| `cmd/capture` | Ingests raw market data |
| `cmd/assembler` | Aggregates into Postgres + S3 |
| `cmd/simulator` | Generates synthetic data |
| `cmd/api` | Serves via gRPC + WebSocket |
| `cmd/refdata` | Reference data management |
| `cmd/migrate` | Database migrations (goose) |

## Build

```bash
bazel run //line/cmd/api
cd line/web && pnpm dev
```

## Stack

Go: franz-go, pgx/v5, aws-sdk-go-v2, parquet-go, gRPC, gorilla/websocket
Web: React 19, Three.js/R3F, Recharts, TailwindCSS 4, Vite
