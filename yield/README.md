# yield

Property investment analysis — rent fairness scoring, investment metrics, AI-powered property agent.

```mermaid
graph LR
    user["Browser"] --> web["React 19 Frontend<br/>(CopilotKit)"]
    web -->|REST| api["Go API<br/>(chi, pgx, Redis)"]
    web -->|CopilotKit| agent["Python Agent<br/>(Pydantic AI)"]
    api --> pg["Postgres"]
    api --> redis["Redis"]
    api -->|cron| scrape["Property Scrapers"]
    agent -->|tools| api
```

## Components

| Dir | What |
|-----|------|
| `api/` | Go backend — chi/v5, pgx/v5, go-redis/v9, goose, go-shp, cron |
| `agent/` | Python AI agent — pydantic-ai, fastapi, copilotkit |
| `web/` | React 19 — CopilotKit, React Router 7, TailwindCSS 4, Vite |

## Run

```bash
bazel run //yield/api              # API
uvicorn yield.agent.serve:app      # Agent
cd yield/web && pnpm dev           # Frontend
```
