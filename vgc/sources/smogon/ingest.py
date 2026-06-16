from __future__ import annotations

import json
import hashlib
from datetime import datetime, timezone

import httpx
import pyarrow as pa
import pyarrow.parquet as pq

from schema.tables import USAGE_STATS_SCHEMA

SMOGON_STATS_BASE = "https://www.smogon.com/stats"

FORMATS = [
    {"game": "champions", "regulation": "M-A", "format": "gen9championsvgc2026regma"},
]

ELO_BRACKETS = [0, 1500, 1630, 1760]


def fetch_chaos_json(period: str, format_id: str, elo: int) -> dict | None:
    url = f"{SMOGON_STATS_BASE}/{period}/chaos/{format_id}-{elo}.json"
    r = httpx.get(url, timeout=60)
    if r.status_code == 200:
        return r.json()
    return None


def parse_chaos_to_rows(chaos: dict, game: str, regulation: str, format_id: str, period: str, elo: int) -> list[dict]:
    rows = []
    now = datetime.now(timezone.utc)
    pokemon_data = chaos.get("data", {})

    for rank_idx, (pokemon_name, stats) in enumerate(pokemon_data.items(), start=1):
        rows.append({
            "game": game,
            "regulation": regulation,
            "format": format_id,
            "period": period,
            "elo_bracket": elo,
            "pokemon": pokemon_name,
            "rank": rank_idx,
            "usage_pct": stats.get("usage", 0.0) * 100,
            "raw_count": int(stats.get("Raw count", 0)),
            "abilities": json.dumps(stats.get("Abilities", {})),
            "items": json.dumps(stats.get("Items", {})),
            "spreads": json.dumps(stats.get("Spreads", {})),
            "moves": json.dumps(stats.get("Moves", {})),
            "tera_types": json.dumps(stats.get("Tera Types", {})),
            "teammates": json.dumps(stats.get("Teammates", {})),
            "checks_counters": json.dumps(stats.get("Checks and Counters", {})),
            "viability_ceiling": int(stats["Viability Ceiling"][0]) if isinstance(stats.get("Viability Ceiling"), list) else int(stats.get("Viability Ceiling", 0)),
            "ingested_at": now,
        })
    return rows


def ingest_period(period: str, output_dir: str) -> str | None:
    all_rows = []
    for fmt in FORMATS:
        for elo in ELO_BRACKETS:
            chaos = fetch_chaos_json(period, fmt["format"], elo)
            if chaos is None:
                continue
            rows = parse_chaos_to_rows(
                chaos,
                game=fmt["game"],
                regulation=fmt["regulation"],
                format_id=fmt["format"],
                period=period,
                elo=elo,
            )
            all_rows.extend(rows)

    if not all_rows:
        return None

    table = pa.Table.from_pylist(all_rows, schema=USAGE_STATS_SCHEMA)
    path = f"{output_dir}/usage_stats_{period}.parquet"
    pq.write_table(table, path, compression="zstd")
    return path


def list_available_periods() -> list[str]:
    r = httpx.get(f"{SMOGON_STATS_BASE}/", timeout=30)
    import re
    return sorted(re.findall(r'href="(\d{4}-\d{2})/"', r.text))


if __name__ == "__main__":
    import sys

    output_dir = sys.argv[1] if len(sys.argv) > 1 else "."
    periods = list_available_periods()
    latest = periods[-1] if periods else None
    if latest:
        path = ingest_period(latest, output_dir)
        if path:
            table = pq.read_table(path)
            print(f"Wrote {table.num_rows} rows to {path}")
            print(f"Formats: {table.column('format').unique().to_pylist()}")
            print(f"ELO brackets: {table.column('elo_bracket').unique().to_pylist()}")
            print(f"Top 5 (1500 ELO):")
            import polars as pl
            df = pl.from_arrow(table)
            top = df.filter(pl.col("elo_bracket") == 1500).sort("rank").head(5)
            for row in top.iter_rows(named=True):
                print(f"  #{row['rank']} {row['pokemon']} ({row['usage_pct']:.1f}%)")
