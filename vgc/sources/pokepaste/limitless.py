from __future__ import annotations

import json
import hashlib
from datetime import datetime, timezone, date

import httpx
import pyarrow as pa
import pyarrow.parquet as pq

from schema.tables import TEAM_SHEETS_SCHEMA

LIMITLESS_API = "https://play.limitlesstcg.com/api"


def fetch_tournaments(game: str = "VGC", format_: str = "M-A") -> list[dict]:
    r = httpx.get(
        f"{LIMITLESS_API}/tournaments",
        params={"game": game, "format": format_},
        timeout=30,
    )
    r.raise_for_status()
    return r.json()


def fetch_standings(tournament_id: str) -> list[dict]:
    r = httpx.get(
        f"{LIMITLESS_API}/tournaments/{tournament_id}/standings",
        timeout=30,
    )
    r.raise_for_status()
    return r.json()


def make_team_id(pokemon_list: list[dict]) -> str:
    key = json.dumps(sorted([p["id"] for p in pokemon_list]))
    return hashlib.sha256(key.encode()).hexdigest()[:16]


def standings_to_rows(standings: list[dict], tournament: dict) -> list[dict]:
    rows = []
    now = datetime.now(timezone.utc)
    event_date = None
    if tournament.get("date"):
        event_date = date.fromisoformat(tournament["date"][:10])

    for entry in standings:
        decklist = entry.get("decklist")
        if not decklist:
            continue

        team_id = make_team_id(decklist)
        rows.append({
            "team_id": team_id,
            "source": "limitless",
            "source_url": f"https://play.limitlesstcg.com/tournament/{tournament['id']}/player/{entry.get('player', '')}",
            "game": tournament.get("game", "VGC"),
            "regulation": tournament.get("format", "M-A"),
            "event_name": tournament.get("name", ""),
            "placement": entry.get("placing", 0),
            "player_name": entry.get("name", ""),
            "date": event_date,
            "pokemon": json.dumps(decklist),
            "ingested_at": now,
        })
    return rows


def ingest_tournaments(output_dir: str, format_: str = "M-A", min_players: int = 16) -> str | None:
    tournaments = fetch_tournaments(format_=format_)
    all_rows = []

    for t in tournaments:
        if t.get("players", 0) < min_players:
            continue
        try:
            standings = fetch_standings(t["id"])
            rows = standings_to_rows(standings, t)
            all_rows.extend(rows)
        except Exception:
            continue

    if not all_rows:
        return None

    table = pa.Table.from_pylist(all_rows, schema=TEAM_SHEETS_SCHEMA)
    path = f"{output_dir}/team_sheets_limitless.parquet"
    pq.write_table(table, path, compression="zstd")
    return path


if __name__ == "__main__":
    import sys

    output_dir = sys.argv[1] if len(sys.argv) > 1 else "."
    path = ingest_tournaments(output_dir, min_players=30)
    if path:
        table = pq.read_table(path)
        print(f"Wrote {table.num_rows} team sheets to {path}")
        import polars as pl
        df = pl.from_arrow(table)
        print(f"Tournaments: {df['event_name'].n_unique()}")
        print(f"Players: {df['player_name'].n_unique()}")
    else:
        print("No data ingested")
