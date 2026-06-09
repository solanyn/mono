from __future__ import annotations

import json
from datetime import datetime, timezone

import httpx
import pyarrow as pa
import pyarrow.parquet as pq

from schema.tables import GAME_DATA_SCHEMA

POKEAPI_BASE = "https://pokeapi.co/api/v2"

ENTITY_ENDPOINTS = {
    "pokemon": "/pokemon?limit=2000",
    "move": "/move?limit=1000",
    "ability": "/ability?limit=400",
    "item": "/item?limit=2500",
    "type": "/type?limit=25",
    "nature": "/nature?limit=25",
}


def fetch_list(entity_type: str) -> list[dict]:
    url = f"{POKEAPI_BASE}{ENTITY_ENDPOINTS[entity_type]}"
    r = httpx.get(url, timeout=30)
    r.raise_for_status()
    return r.json()["results"]


def fetch_entity(url: str) -> dict:
    r = httpx.get(url, timeout=30)
    r.raise_for_status()
    return r.json()


def extract_pokemon(data: dict) -> dict:
    return {
        "base_stats": {s["stat"]["name"]: s["base_stat"] for s in data["stats"]},
        "types": [t["type"]["name"] for t in data["types"]],
        "abilities": [a["ability"]["name"] for a in data["abilities"]],
        "weight": data["weight"],
        "sprites": {
            "front": data["sprites"]["front_default"],
            "official": data["sprites"].get("other", {}).get("official-artwork", {}).get("front_default"),
        },
    }


def extract_move(data: dict) -> dict:
    return {
        "power": data["power"],
        "accuracy": data["accuracy"],
        "pp": data["pp"],
        "type": data["type"]["name"],
        "damage_class": data["damage_class"]["name"],
        "priority": data["priority"],
        "target": data["target"]["name"],
        "meta": {
            "category": data.get("meta", {}).get("category", {}).get("name") if data.get("meta") else None,
        },
    }


def extract_ability(data: dict) -> dict:
    effect = ""
    for entry in data.get("effect_entries", []):
        if entry["language"]["name"] == "en":
            effect = entry["short_effect"]
            break
    return {"effect": effect, "is_main_series": data.get("is_main_series", True)}


def extract_type(data: dict) -> dict:
    dr = data["damage_relations"]
    return {
        "double_damage_to": [t["name"] for t in dr["double_damage_to"]],
        "half_damage_to": [t["name"] for t in dr["half_damage_to"]],
        "no_damage_to": [t["name"] for t in dr["no_damage_to"]],
        "double_damage_from": [t["name"] for t in dr["double_damage_from"]],
        "half_damage_from": [t["name"] for t in dr["half_damage_from"]],
        "no_damage_from": [t["name"] for t in dr["no_damage_from"]],
    }


def extract_item(data: dict) -> dict:
    effect = ""
    for entry in data.get("effect_entries", []):
        if entry["language"]["name"] == "en":
            effect = entry["short_effect"]
            break
    return {"effect": effect, "category": data.get("category", {}).get("name", "")}


def extract_nature(data: dict) -> dict:
    return {
        "increased_stat": data["increased_stat"]["name"] if data["increased_stat"] else None,
        "decreased_stat": data["decreased_stat"]["name"] if data["decreased_stat"] else None,
    }


EXTRACTORS = {
    "pokemon": extract_pokemon,
    "move": extract_move,
    "ability": extract_ability,
    "type": extract_type,
    "item": extract_item,
    "nature": extract_nature,
}


def sync_entity_type(entity_type: str, generation: int = 9) -> list[dict]:
    entities = fetch_list(entity_type)
    rows = []
    now = datetime.now(timezone.utc)
    extractor = EXTRACTORS[entity_type]

    for entity in entities:
        try:
            data = fetch_entity(entity["url"])
            extracted = extractor(data)
            rows.append({
                "entity_type": entity_type,
                "name": entity["name"],
                "id": data["id"],
                "data": json.dumps(extracted),
                "generation": generation,
                "synced_at": now,
            })
        except Exception:
            continue

    return rows


def sync_all(output_dir: str, entity_types: list[str] | None = None, pokemon_filter: list[str] | None = None) -> dict[str, str]:
    if entity_types is None:
        entity_types = ["type", "nature", "ability"]

    paths = {}
    for entity_type in entity_types:
        if entity_type == "pokemon" and pokemon_filter:
            rows = sync_pokemon_filtered(pokemon_filter)
        else:
            rows = sync_entity_type(entity_type)
        if rows:
            table = pa.Table.from_pylist(rows, schema=GAME_DATA_SCHEMA)
            path = f"{output_dir}/game_data_{entity_type}.parquet"
            pq.write_table(table, path, compression="zstd")
            paths[entity_type] = path
    return paths


def sync_pokemon_filtered(pokemon_names: list[str], generation: int = 9) -> list[dict]:
    rows = []
    now = datetime.now(timezone.utc)
    for name in pokemon_names:
        slug = name.lower().replace(" ", "-").replace("'", "").replace(".", "")
        try:
            data = fetch_entity(f"{POKEAPI_BASE}/pokemon/{slug}")
            extracted = extract_pokemon(data)
            rows.append({
                "entity_type": "pokemon",
                "name": slug,
                "id": data["id"],
                "data": json.dumps(extracted),
                "generation": generation,
                "synced_at": now,
            })
        except Exception:
            continue
    return rows


if __name__ == "__main__":
    import sys

    output_dir = sys.argv[1] if len(sys.argv) > 1 else "."
    entity_types = sys.argv[2:] if len(sys.argv) > 2 else ["type", "nature"]
    paths = sync_all(output_dir, entity_types)
    for entity_type, path in paths.items():
        table = pq.read_table(path)
        print(f"{entity_type}: {table.num_rows} rows -> {path}")
