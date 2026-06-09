---
date: "2026-06-09"
title: "Building an Automated VGC Meta Analysis Platform"
tags: ["pokemon", "data-engineering", "bazel", "kubernetes"]
description: "A data pipeline that ingests competitive Pokemon VGC ladder and tournament data, computes speed tiers, and publishes weekly meta reports."
---

## The gap between ladder and tournaments

Here's something that bugged me about VGC resources: they all show you what's popular on ladder. Usage stats, top 30 Pokemon, common spreads. That's useful, but it's not the full picture.

What wins on ladder doesn't always win tournaments. Ladder rewards consistency over 100+ games. Tournaments reward preparation and matchup knowledge over 9 rounds. A mon that's 15th in usage but shows up in top 8 at three regionals in a row is telling you something, and most stat sites don't surface that signal.

So I built a pipeline that merges both datasets and highlights where they disagree.

## What it actually does

Three data sources feed into an Iceberg lakehouse:

- **Smogon chaos JSON** provides ladder stats. Usage, items, abilities, EV spreads, teammates, tera types. Updated continuously across ELO brackets.
- **Limitless TCG** provides tournament results. Full team sheets with placements from real events.
- **PokeAPI** provides base stats for computing actual speed values.

A weekly CronJob pulls everything, runs analysis, generates a [marimo](https://marimo.io) report, and a Gemma 4 model writes a short meta commentary. The output is published to the site.

## Speed tiers are the interesting bit

Most meta resources show you EV spreads. "252 Spe Jolly" is cool, but what's the actual number at level 50? And more importantly, where does that sit relative to everything else in the format?

The report computes final speed stats for every popular spread across the top 30 Pokemon, weighted by usage. Aggregate that and you get a single "meta speed" number. Is the format fast and offensive, or slow and bulky? Are people investing in speed control or dumping into survivability?

You can watch this shift week to week. After a big regional where Trick Room dominates, ladder speeds drop for two weeks as everyone copies the winning archetype. Then the fast stuff comes back to punish it. The cycle is real and you can see it in the numbers.

## Ladder vs tournament divergence

This is the part I actually built this for. When a Pokemon is significantly more successful in tournaments than its ladder usage would suggest, that's a signal. It usually means one of three things:

1. The mon rewards preparation because you need to know your matchups.
2. It's good in best-of-3 but bad in best-of-1 due to side game adaptation.
3. It's a meta call that only works when you know exactly what you're targeting.

Conversely, stuff that's everywhere on ladder but missing from top cuts is usually autopilot-friendly but exploitable by anyone who preps for it.

## The calc service

The Smogon damage calculator runs as an HTTP service in the cluster. TypeScript bundled into a single file, always on. Any part of the system can ask "does this KO?" without needing Node locally. It's overkill for now but I want it available for an interactive teambuilder eventually.

## Infrastructure (briefly)

Bazel monorepo, FluxCD, home k8s cluster. Two OCI images for the Python pipeline and Node calc service. Iceberg on Lakekeeper makes the raw data queryable if I want to explore beyond the published reports. Output lands on R2 behind Cloudflare.

I went with Iceberg over plain parquet because I want to query across weeks without managing file paths manually. Table evolution is nice when you inevitably want to add a column three weeks in.

## What surprised me

The biggest gap between ladder and tournaments isn't in Pokemon choice. It's in EV spreads. Tournament players run way more specific speed tiers targeting exact threats. Ladder players just go max speed or max bulk. That spread optimization is where the skill gap lives, and it's invisible if you're only looking at usage percentages.
