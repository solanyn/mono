---
date: "2026-06-09"
title: "Building an Automated VGC Meta Analysis Platform"
tags: ["pokemon", "data-engineering", "bazel", "kubernetes"]
description: "A data pipeline that ingests competitive Pokemon VGC ladder and tournament data, computes speed tiers, and publishes weekly meta reports."
---

Competitive Pokemon VGC has a surprisingly rich data ecosystem. Smogon publishes raw ladder statistics as structured JSON, Limitless TCG exposes tournament results via API, and the damage calculator is open source TypeScript. The data is there — it just needs a pipeline to make it useful.

## Architecture

```
Smogon Chaos JSON ──┐
Limitless TCG API ──┼──→ Ingestion ──→ Parquet ──→ Iceberg (Lakekeeper)
PokeAPI ────────────┘         │
                              ▼
                    Analysis (polars + pyarrow)
                              │
                    ┌─────────┼─────────┐
                    ▼         ▼         ▼
              Meta Report   LLM Pulse   Lake Tables
              (marimo)      (Gemma 4)   (queryable)
                    │         │
                    ▼         ▼
                 R2 (CDN) ──→ goyangi.io/vgc
```

The pipeline runs weekly as a k8s CronJob. Each stage writes parquet locally, appends to Iceberg tables via Lakekeeper, then generates reports.

## Data sources

### Smogon chaos JSON

The goldmine. Smogon publishes usage statistics for every format at `smogon.com/stats/{period}/chaos/{format}-{elo}.json`. For VGC Champions, that's `gen9championsvgc2026regma` across four ELO brackets (0, 1500, 1630, 1760).

Each file contains per-Pokemon data: raw usage count, abilities, items, spreads, moves, tera types, teammates and checks/counters — all as weighted counts. The key insight is these aren't percentages. Teammates values like `462985.06` need dividing by the Pokemon's raw count to get the actual co-usage rate.

A single month gives ~250 Pokemon per bracket, ~1000 rows total. The spreads field alone contains hundreds of EV distributions per Pokemon with their relative popularity.

### Limitless TCG

Tournament platform with a public API. Pulls completed VGC events with full team sheets — six Pokemon each with moves, items, abilities, tera types. ~1000 teams from ~20 tournaments per pull. Each team gets a deterministic ID (SHA256 of sorted species).

### PokeAPI

Base stats for speed tier calculations. Rather than syncing all 2000+ Pokemon (which takes forever with rate limiting), the pipeline filters to only Pokemon appearing in the current usage stats. ~250 targeted fetches instead of 2000.

## Schema design

Seven PyArrow schemas feeding three Iceberg tables:

```python
USAGE_STATS_SCHEMA = pa.schema([
    ("game", pa.string()),          # champions | sv
    ("regulation", pa.string()),    # M-A, I, etc.
    ("format", pa.string()),        # gen9championsvgc2026regma
    ("period", pa.string()),        # 2026-05
    ("elo_bracket", pa.int16()),    # 0, 1500, 1630, 1760
    ("pokemon", pa.string()),
    ("rank", pa.int16()),
    ("usage_pct", pa.float64()),
    ("raw_count", pa.int64()),
    ("abilities", pa.string()),     # JSON: {"Adaptability": 89.2, ...}
    ("items", pa.string()),         # JSON: {"Choice Scarf": 53.8, ...}
    ("spreads", pa.string()),       # JSON: {"Jolly:0/252/0/0/4/252": 17.6, ...}
    ("moves", pa.string()),
    ("tera_types", pa.string()),
    ("teammates", pa.string()),
    ("checks_counters", pa.string()),
    ("viability_ceiling", pa.int32()),
    ("ingested_at", pa.timestamp("us", tz="UTC")),
])
```

JSON-encoded maps for variable-length fields (abilities, items, spreads) rather than nested types. Keeps the schema flat for Iceberg compatibility while preserving full fidelity. The `spreads` field encodes nature and all six EVs as `Nature:HP/Atk/Def/SpA/SpD/Spe`.

## Speed tier computation

Most meta analysis shows EV investment. That's only half the picture — what matters is the final speed stat. The formula at level 50:

```
floor(((2 * base_speed + 31 + speed_ev / 4) * 50 / 100 + 5) * nature_mod)
```

For each of the top 30 Pokemon, the pipeline:
1. Looks up base speed from PokeAPI data
2. Parses the top 3 spreads (by popularity)
3. Extracts the speed EV and nature
4. Computes the actual speed stat

Then weights everything by spread popularity to produce a single number: the meta's average speed. This tells you immediately whether the format is trending towards speed control or bulk. A weighted average speed stat of 140+ means you need to respect fast threats; below 120 signals a bulkier, TR-friendly meta.

## The calc bridge

[`@smogon/calc`](https://github.com/smogon/damage-calc) is the standard damage calculator — TypeScript, well-maintained, accurate. Rather than porting it to Python or using FFI, it runs as a stateless HTTP service:

```javascript
const server = createServer((req, res) => {
  if (req.method === "POST" && req.url === "/calc") {
    // Parse body → run calculate() → return JSON
  }
});
```

Single file, bundled to 812K with esbuild via `aspect_rules_esbuild` in Bazel. Accepts single calcs or batches. The Python side is just:

```python
def run_calc(attacker, defender, move, field=None):
    resp = httpx.post(f"{CALC_URL}/calc", json=payload, timeout=10)
    return resp.json()
```

Runs as an always-on Deployment in the cluster — available for the pipeline, future teambuilder, or ad-hoc queries.

## Ladder vs tournament analysis

One of the more revealing analyses. The pipeline merges Smogon 1500+ ELO ladder data with Limitless tournament team sheets and computes a usage gap:

```
diff = tournament_usage_pct - ladder_usage_pct
```

Positive diff = "tournament-favoured" (wins events more than ladder popularity suggests). Negative = "ladder-only" (popular online but doesn't translate to tournament success). This surfaces Pokemon that reward preparation and matchup knowledge versus those that thrive in best-of-one ladder variance.

## Infrastructure

- **Compute**: FluxCD CronJob (weekly, Monday 6am AEST) on a home k8s cluster
- **Storage**: Garage S3 (in-cluster) for Iceberg data files, Cloudflare R2 for published reports
- **Catalog**: Lakekeeper REST catalog for Iceberg table metadata
- **LLM**: Gemma 4 via agentgateway for weekly narrative generation
- **Build**: Bazel (rules_python + rules_oci for pipeline, aspect_rules_esbuild + rules_oci for calc)
- **CI**: GitHub Actions — builds, tests, pushes OCI images to GHCR on merge
- **Reports**: marimo notebooks exported as self-contained HTML with altair charts

## What's next

- **Showdown replays** — actual game data (leads, KOs, win conditions) for matchup analysis
- **Historical trends** — month-over-month speed/bulk shifts, regulation lifecycle tracking
- **Interactive dashboard** — marimo server mode, always-on, queryable from Iceberg directly
- **End-of-season reports** — comprehensive analysis before format rotations
