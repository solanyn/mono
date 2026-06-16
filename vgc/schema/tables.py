import pyarrow as pa

GAME_DATA_SCHEMA = pa.schema([
    ("entity_type", pa.string()),  # pokemon | move | ability | item | type | nature
    ("name", pa.string()),
    ("id", pa.int32()),
    ("data", pa.string()),  # JSON blob for entity-specific fields
    ("generation", pa.int8()),
    ("synced_at", pa.timestamp("us", tz="UTC")),
])

USAGE_STATS_SCHEMA = pa.schema([
    ("game", pa.string()),  # champions | sv | showdown
    ("regulation", pa.string()),  # M-A, I, etc.
    ("format", pa.string()),  # gen9championsvgc2026regma
    ("period", pa.string()),  # 2026-05
    ("elo_bracket", pa.int16()),  # 0, 1500, 1630, 1760
    ("pokemon", pa.string()),
    ("rank", pa.int16()),
    ("usage_pct", pa.float64()),
    ("raw_count", pa.int64()),
    ("abilities", pa.string()),  # JSON: {"Adaptability": 89.2, ...}
    ("items", pa.string()),  # JSON: {"Choice Scarf": 53.8, ...}
    ("spreads", pa.string()),  # JSON: {"Jolly:2/32/0/0/0/32": 17.6, ...}
    ("moves", pa.string()),  # JSON: {"Last Respects": 99.7, ...}
    ("tera_types", pa.string()),  # JSON: {"nothing": ...} (or actual tera types for SV)
    ("teammates", pa.string()),  # JSON: {"Kingambit": 40.6, ...}
    ("checks_counters", pa.string()),  # JSON
    ("viability_ceiling", pa.int32()),
    ("ingested_at", pa.timestamp("us", tz="UTC")),
])

TEAM_SHEETS_SCHEMA = pa.schema([
    ("team_id", pa.string()),  # hash of sorted species+moves+items+evs
    ("source", pa.string()),  # pokepaste | rk9 | limitless | youtube
    ("source_url", pa.string()),
    ("game", pa.string()),
    ("regulation", pa.string()),
    ("event_name", pa.string()),
    ("placement", pa.int16()),
    ("player_name", pa.string()),
    ("date", pa.date32()),
    ("pokemon", pa.string()),  # JSON array of 6 pokemon objects
    ("ingested_at", pa.timestamp("us", tz="UTC")),
])

KNOWLEDGE_SCHEMA = pa.schema([
    ("fact_id", pa.string()),
    ("fact_type", pa.string()),  # synergy | ev_rationale | lead_pattern | matchup_note | meta_call | team_report | tech_pick | speed_tier
    ("game", pa.string()),
    ("regulation", pa.string()),
    ("content", pa.string()),  # structured JSON for the fact
    ("confidence", pa.string()),  # high | medium | low
    ("source_type", pa.string()),  # youtube | tournament | calc
    ("source_url", pa.string()),
    ("source_timestamp", pa.string()),  # timestamp within video if applicable
    ("source_channel", pa.string()),
    ("valid_until", pa.date32()),  # regulation end date
    ("extracted_at", pa.timestamp("us", tz="UTC")),
])

CORES_SCHEMA = pa.schema([
    ("core_id", pa.string()),  # hash of sorted pokemon names
    ("game", pa.string()),
    ("regulation", pa.string()),
    ("period", pa.string()),
    ("pokemon", pa.list_(pa.string())),  # 2-3 pokemon names
    ("co_occurrence_pct", pa.float64()),
    ("combined_usage_pct", pa.float64()),
    ("synergy_notes", pa.string()),  # JSON array of knowledge facts
    ("derived_at", pa.timestamp("us", tz="UTC")),
])

META_SNAPSHOTS_SCHEMA = pa.schema([
    ("game", pa.string()),
    ("regulation", pa.string()),
    ("period", pa.string()),  # 2026-05 or 2026-W22
    ("period_type", pa.string()),  # monthly | weekly
    ("top_pokemon", pa.string()),  # JSON array of top N with usage
    ("top_cores", pa.string()),  # JSON array of top cores
    ("archetype_distribution", pa.string()),  # JSON
    ("rising", pa.string()),  # JSON array of pokemon gaining usage
    ("falling", pa.string()),  # JSON array of pokemon losing usage
    ("total_battles", pa.int64()),
    ("derived_at", pa.timestamp("us", tz="UTC")),
])

CALCS_SCHEMA = pa.schema([
    ("calc_id", pa.string()),
    ("generation", pa.int8()),
    ("attacker", pa.string()),
    ("attacker_item", pa.string()),
    ("attacker_ability", pa.string()),
    ("attacker_nature", pa.string()),
    ("attacker_evs", pa.string()),  # JSON: {"atk": 252, "spe": 252, ...}
    ("defender", pa.string()),
    ("defender_item", pa.string()),
    ("defender_ability", pa.string()),
    ("defender_nature", pa.string()),
    ("defender_evs", pa.string()),  # JSON
    ("move", pa.string()),
    ("conditions", pa.string()),  # JSON: weather, terrain, etc.
    ("damage_min", pa.int16()),
    ("damage_max", pa.int16()),
    ("damage_min_pct", pa.float64()),
    ("damage_max_pct", pa.float64()),
    ("ko_chance", pa.string()),  # "guaranteed OHKO", "50% chance to 2HKO", etc.
    ("description", pa.string()),  # full human-readable description
    ("calc_version", pa.string()),  # @smogon/calc version
    ("computed_at", pa.timestamp("us", tz="UTC")),
])
