from __future__ import annotations

import json
import os

import httpx


AGENTGATEWAY_URL = os.getenv(
    "AGENTGATEWAY_URL",
    "https://gateway.goyangi.io/v1/gemma-4",
)


def generate_narrative(stats_summary: dict, prev_summary: dict | None = None) -> str:
    system_prompt = """You are a competitive Pokemon VGC analyst writing a weekly meta report.
Write 3-4 concise paragraphs analyzing the current state of the metagame.
Focus on: what's dominant, what's rising/falling, speed vs bulk trends, and tournament prep angles.
Use specific Pokemon names and statistics from the data provided.
Write in an informative but engaging tone for competitive players."""

    user_content = f"Current week data:\n{json.dumps(stats_summary, indent=2)}"
    if prev_summary:
        user_content += f"\n\nPrevious week data:\n{json.dumps(prev_summary, indent=2)}"
        user_content += "\n\nHighlight week-over-week changes and what players should adapt to."

    response = httpx.post(
        f"{AGENTGATEWAY_URL}/chat/completions",
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {os.getenv('AGENTGATEWAY_API_KEY', '')}",
        },
        json={
            "model": "mlx-community/gemma-4-e4b-it-4bit",
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_content},
            ],
            "max_tokens": 1024,
            "temperature": 0.7,
        },
        timeout=120,
    )
    response.raise_for_status()
    data = response.json()
    return data["choices"][0]["message"]["content"]


def build_stats_summary(usage_df, base_stats: dict, top_n: int = 20) -> dict:
    import polars as pl

    elo_1500 = usage_df.filter(pl.col("elo_bracket") == 1500).sort("rank")
    top = elo_1500.head(top_n)

    top_pokemon = []
    for r in top.iter_rows(named=True):
        top_pokemon.append({
            "pokemon": r["pokemon"],
            "usage_pct": round(r["usage_pct"], 2),
            "rank": r["rank"],
        })

    from reports.helpers import parse_spreads_with_speed
    import pandas as pd

    spread_df = parse_spreads_with_speed(elo_1500, base_stats, top_n=top_n)
    avg_speed = 0
    avg_bulk = 0
    if not spread_df.empty:
        sum_pct = spread_df["Spread %"].sum()
        if sum_pct > 0:
            avg_speed = round((spread_df["Actual Speed"] * spread_df["Spread %"]).sum() / sum_pct, 1)
            avg_bulk = round((spread_df["Bulk EVs"] * spread_df["Spread %"]).sum() / sum_pct, 1)

    return {
        "top_pokemon": top_pokemon,
        "avg_speed_stat": avg_speed,
        "avg_bulk_evs": avg_bulk,
        "total_pokemon_seen": len(elo_1500),
    }
