---
date: "2026-06-09"
title: "Building an Automated VGC Meta Analysis Platform"
tags: ["pokemon", "data-engineering", "bazel", "kubernetes"]
description: "A data pipeline that ingests competitive Pokemon VGC ladder and tournament data, computes speed tiers, and publishes weekly meta reports."
---

Competitive Pokemon VGC has a surprisingly rich data ecosystem. Smogon publishes raw ladder statistics as JSON, Limitless TCG exposes tournament results via API, and the damage calculator is open source TypeScript. Felt like a good excuse to build something.

## What it does

Every Monday, a pipeline runs in my home k8s cluster:

1. **Smogon chaos JSON** — pulls usage stats across ELO brackets (0, 1500, 1630, 1760) for the Champions format
2. **Limitless TCG** — scrapes tournament team sheets with full decklists (Pokemon, moves, items, abilities, tera types)
3. **PokeAPI** — syncs base stats for speed tier calculations
4. **Meta report** — generates an interactive [marimo](https://marimo.io) notebook with altair charts
5. **LLM narrative** — Gemma 4 writes a weekly meta pulse via agentgateway
6. **Publish** — uploads self-contained HTML to R2

The report covers: top 20 usage, actual speed tiers (not just EVs — real stats at Lv50), team cores, ladder vs tournament divergence, anti-meta picks at high ELO, and item/ability distributions.

## Speed tiers over EV investment

Most meta reports just show EV spreads. But knowing someone runs 252 Speed EVs on Garchomp isn't that useful without context. What matters is the final speed stat — `floor(((2 * base + IV + EV/4) * 50/100 + 5) * nature)`. The report computes this for every popular spread across the top 30 Pokemon, weighted by popularity. You can immediately see whether the meta is clustering around specific speed tiers or spreading out into bulk.

## The calc service

[`@smogon/calc`](https://github.com/smogon/damage-calc) is TypeScript — accurate damage calculations but not callable from Python directly. Instead of trying to port it, I wrapped it as a stateless HTTP service:

- `POST /calc` — single or batch calculations
- `GET /health` — liveness probe

It's bundled with esbuild into a single 812K file, runs on `node:22-slim`, and sits as an always-on Deployment in the cluster. The Python pipeline (or any future teambuilder UI) just calls it with httpx.

## Ladder vs tournaments

One of the more interesting sections. What's popular on ladder doesn't always match what wins events. The report merges Smogon 1500+ ELO data with Limitless tournament placements and highlights the gap — Pokemon that are "tournament-favoured" (higher usage in events than on ladder) versus "ladder-only" picks.

## Stack

- **Pipeline**: Python (polars, pyarrow, httpx, marimo)
- **Calc**: Node.js (@smogon/calc, esbuild bundle)
- **Lake**: pyiceberg → Lakekeeper REST catalog → Garage S3
- **Build**: Bazel (rules_python, rules_oci, aspect_rules_esbuild, aspect_rules_js)
- **Infra**: FluxCD CronJob, Garage S3, Cloudflare R2, agentgateway (Gemma 4)
- **Reports**: marimo export HTML, altair charts

All lives in the monorepo alongside the rest of my projects. Bazel builds both the Python pipeline image and the Node calc image, CI pushes them to GHCR on merge.

## What's next

- Showdown replay ingestion (actual game data — leads, KOs, win conditions)
- Historical trend lines (month-over-month speed/bulk shifts)
- Interactive dashboard (marimo server mode, always-on)
- End-of-regulation season report before format rotations
