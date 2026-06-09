---
date: "2026-06-09"
title: "Building an Automated VGC Meta Analysis Platform"
tags: ["pokemon", "data-engineering", "bazel", "kubernetes"]
description: "A data pipeline that ingests competitive Pokemon VGC ladder and tournament data, computes speed tiers, and publishes weekly meta reports."
---

## Ladder stats lie (a little)

Every VGC resource shows you what's popular on ladder. Usage rates, top 30 Pokemon, most common items. I've been staring at Pikalytics and various stat dumps for years and something always bugged me: what wins on ladder doesn't always win events.

Ladder rewards consistency over hundreds of games. Tournaments reward preparation over 9 rounds. A mon sitting at 15th in usage that keeps showing up in top 8 at regionals is telling you something, and most stat sites just don't surface that signal.

So I built a pipeline that pulls from both, merges them, and highlights where they disagree.

## Three data sources, one lakehouse

**Smogon chaos JSON** is the good stuff. Per-Pokemon usage data for every format: abilities, items, EV spreads, moves, tera types, teammates, split by ELO bracket. This is the ladder: millions of games, updated continuously.

**Limitless TCG** gives you tournament results. Full team sheets from real events with actual placements. This is what wins when people are trying their hardest.

**PokeAPI** for base stats. Needed because I don't want to compute speed tiers by hand.

Everything lands in Iceberg tables on [Lakekeeper](https://github.com/lakekeeper/lakekeeper) (backed by Garage S3). A weekly CronJob pulls, processes, and generates a [marimo](https://marimo.io) report. A Gemma 4 model writes a short meta commentary via agentgateway because why not.

## Speed tiers are where it gets interesting

Most meta resources show EV spreads. "252 Spe Jolly" is cool, but what's the actual speed stat at level 50? And where does that sit relative to everything else in the format?

The report computes final speed for every popular spread across the top 30 mons, weighted by usage. Aggregate that and you get a "meta speed" number. Is the format fast and offensive? Slow and bulky? Are people investing in speed control or dumping into survivability?

You can watch this shift week to week. After a regional where Trick Room dominates, ladder speeds drop as everyone copies the winning archetype. Then the fast stuff comes back to punish it. The cycle is real and it shows up in the numbers.

## The actual point: ladder vs tournament divergence

This is what I built the whole thing for.

When a Pokemon is way more successful in tournaments than its ladder usage suggests, that usually means one of three things:

1. It rewards preparation. You need to know your matchups cold.
2. It's a best-of-3 pick. Adapts in side games, mediocre in best-of-1 ladder.
3. It's a meta call. Only works when you know exactly what you're targeting.

The flip side is interesting too. Stuff that's everywhere on ladder but missing from top cuts is usually autopilot-friendly but exploitable by anyone who preps for it. If you're heading into a regional, these are the mons you want answers to but probably don't need to bring yourself.

## EV spread divergence (the surprise)

I expected the biggest ladder/tournament gap to be in Pokemon choice. It's not. It's in EV spreads.

Tournament players run way more specific speed tiers targeting exact threats. "I need to outspeed max speed Kingambit after one Electro Drift boost" kind of precision. Ladder players just go max speed or max bulk because it's safe and doesn't require thinking about the matchup.

That spread optimization is where the skill gap actually lives. And it's completely invisible if you're only looking at usage percentages.

## The calc service

[`@smogon/calc`](https://github.com/smogon/damage-calc) runs as a standalone HTTP service in the cluster. TypeScript bundled into a single file, always available. Any part of the system can ask "does this KO?" without needing Node locally.

Overkill for just reports but I want it for an interactive teambuilder eventually.

## Infra

Bazel monorepo, FluxCD, home k8s cluster. Two OCI images: Python pipeline and Node calc service. Reports land on R2 behind Cloudflare.

I went with Iceberg over plain parquet because querying across weeks without managing file paths manually is nice. And table evolution means I can add columns three weeks in without rewriting everything, which I already had to do twice during development. Glad I didn't go with raw files.

## What I learned

The meta moves in predictable cycles if you have enough data. Big tournament, then the meta copies winners, then counter-meta emerges, then it stabilizes, then the next tournament disrupts again. The speed tier aggregate makes this legible at a glance instead of requiring you to manually track 30 different Pokemon's spreads.

Also: Smogon's data quality is incredible for a community-run project. Limitless is good but inconsistent. Some events have full sheets, others just have top cut. The merge logic handles gaps gracefully but it means some weeks have better tournament signal than others.

## Where this is going

The meta analysis pipeline is step one. All this data collection and the calc service exist because I want to build something more ambitious: an AI agent that actually plays VGC.

The structured knowledge base (what beats what, speed tiers, optimal spreads, team cores) is exactly the kind of context an LLM agent needs to make turn-by-turn decisions. And the damage calculator gives it ground truth for "will this KO" without hallucinating numbers.

The longer term dream is an RL agent trained on replay data that learns positioning, prediction patterns, and endgame play. The meta analysis pipeline feeds it the current metagame so it adapts as the format shifts. More on that when I have something worth showing.
