from __future__ import annotations

import json
import os
from pathlib import Path

import pandas as pd
import polars as pl


def data_dir() -> Path:
    env = os.getenv("VGC_DATA_DIR")
    if env:
        return Path(env)
    testdata = Path(__file__).parent.parent / "testdata"
    if testdata.exists() and list(testdata.glob("usage_stats_*.parquet")):
        return testdata
    return Path(".")


def load_usage_stats() -> pl.DataFrame:
    files = list(data_dir().glob("usage_stats_*.parquet"))
    if not files:
        return pl.DataFrame()
    return pl.concat([pl.read_parquet(f) for f in files])


def load_team_sheets() -> pl.DataFrame:
    files = list(data_dir().glob("team_sheets_*.parquet"))
    if not files:
        return pl.DataFrame()
    return pl.concat([pl.read_parquet(f) for f in files])


def load_base_stats() -> dict[str, int]:
    files = list(data_dir().glob("game_data_pokemon*.parquet"))
    if not files:
        return {}
    df = pl.concat([pl.read_parquet(f) for f in files])
    stats = {}
    for row in df.iter_rows(named=True):
        data = json.loads(row["data"])
        base_speed = data.get("base_stats", {}).get("speed")
        if base_speed is not None:
            stats[row["name"]] = base_speed
    return stats


def pokemon_to_slug(name: str) -> str:
    return name.lower().replace(" ", "-").replace("'", "").replace(".", "")


def calc_speed_stat(base_speed: int, speed_ev: int, nature: str, level: int = 50, iv: int = 31) -> int:
    nature_mod = 1.0
    if nature in ("Jolly", "Timid", "Naive", "Hasty"):
        nature_mod = 1.1
    elif nature in ("Brave", "Relaxed", "Quiet", "Sassy"):
        nature_mod = 0.9
    return int(((2 * base_speed + iv + speed_ev // 4) * level // 100 + 5) * nature_mod)


def nature_modifier_label(nature: str) -> str:
    if nature in ("Jolly", "Timid", "Naive", "Hasty"):
        return "+"
    if nature in ("Brave", "Relaxed", "Quiet", "Sassy"):
        return "-"
    return ""


def parse_spreads_with_speed(usage_df: pl.DataFrame, base_stats: dict[str, int], top_n: int = 30, spreads_per: int = 3) -> pd.DataFrame:
    rows = []
    for r in usage_df.head(top_n).iter_rows(named=True):
        slug = pokemon_to_slug(r["pokemon"])
        base_speed = base_stats.get(slug)
        if base_speed is None:
            continue
        spreads = json.loads(r["spreads"])
        for spread_key, pct in sorted(spreads.items(), key=lambda x: x[1], reverse=True)[:spreads_per]:
            parts = spread_key.split(":")
            if len(parts) != 2:
                continue
            nature = parts[0]
            evs = parts[1].split("/")
            if len(evs) != 6:
                continue
            hp, atk, def_, spa, spd, spe = (int(x) for x in evs)
            rows.append({
                "Pokemon": r["pokemon"],
                "Usage %": r["usage_pct"],
                "Spread %": round(pct, 1),
                "Base Speed": base_speed,
                "Speed EVs": spe,
                "Actual Speed": calc_speed_stat(base_speed, spe, nature),
                "Bulk EVs": hp + def_ + spd,
                "Offense EVs": atk + spa,
                "Nature": nature,
                "Nature Mod": nature_modifier_label(nature),
                "Spread": spread_key,
            })
    return pd.DataFrame(rows)


def parse_teammates(usage_df: pl.DataFrame, top_n: int = 15) -> pd.DataFrame:
    cores = []
    seen: set[tuple[str, str]] = set()
    for r in usage_df.head(top_n).iter_rows(named=True):
        raw_count = r["raw_count"]
        if raw_count <= 0:
            continue
        teammates = json.loads(r["teammates"])
        for mate, count in sorted(teammates.items(), key=lambda x: x[1], reverse=True)[:3]:
            pct = count / raw_count * 100
            if pct > 15:
                core = tuple(sorted([r["pokemon"], mate]))
                if core not in seen:
                    seen.add(core)
                    cores.append({"Pokemon 1": core[0], "Pokemon 2": core[1], "Co-usage %": round(pct, 1)})
    return pd.DataFrame(sorted(cores, key=lambda x: x["Co-usage %"], reverse=True)[:15])


def parse_items_abilities(usage_df: pl.DataFrame, top_n: int = 15) -> pd.DataFrame:
    rows = []
    for r in usage_df.head(top_n).iter_rows(named=True):
        items = json.loads(r["items"])
        abilities = json.loads(r["abilities"])
        item_total = sum(items.values()) or 1
        ability_total = sum(abilities.values()) or 1
        top_item = max(items.items(), key=lambda x: x[1]) if items else ("None", 0)
        top_ability = max(abilities.items(), key=lambda x: x[1]) if abilities else ("None", 0)
        rows.append({
            "Pokemon": r["pokemon"],
            "Item": top_item[0],
            "Item %": round(top_item[1] / item_total * 100, 1),
            "Ability": top_ability[0],
            "Ability %": round(top_ability[1] / ability_total * 100, 1),
        })
    return pd.DataFrame(rows)


def tournament_usage(team_sheets_df: pl.DataFrame) -> pd.DataFrame:
    if len(team_sheets_df) == 0:
        return pd.DataFrame(columns=["Pokemon", "Tournament Usage %", "Tournament Count"])
    all_pokemon: list[str] = []
    total_teams = len(team_sheets_df)
    for r in team_sheets_df.iter_rows(named=True):
        try:
            team = json.loads(r["pokemon"])
            for mon in team:
                name = mon.get("name") or mon.get("species") or mon.get("id", "")
                if name:
                    all_pokemon.append(name.title())
        except (json.JSONDecodeError, TypeError):
            continue
    counts = pd.Series(all_pokemon).value_counts()
    df = pd.DataFrame({"Pokemon": counts.index, "Tournament Count": counts.values})
    df["Tournament Usage %"] = round(df["Tournament Count"] / total_teams * 100, 1)
    return df.head(30)
