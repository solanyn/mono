from schema.tables import (
    GAME_DATA_SCHEMA,
    USAGE_STATS_SCHEMA,
    TEAM_SHEETS_SCHEMA,
    KNOWLEDGE_SCHEMA,
    CORES_SCHEMA,
    META_SNAPSHOTS_SCHEMA,
    CALCS_SCHEMA,
)
import pyarrow as pa


def test_usage_stats_schema_fields():
    assert USAGE_STATS_SCHEMA.field("game").type == pa.string()
    assert USAGE_STATS_SCHEMA.field("elo_bracket").type == pa.int16()
    assert USAGE_STATS_SCHEMA.field("usage_pct").type == pa.float64()
    assert USAGE_STATS_SCHEMA.field("viability_ceiling").type == pa.int32()
    assert USAGE_STATS_SCHEMA.field("ingested_at").type == pa.timestamp("us", tz="UTC")


def test_game_data_schema_fields():
    assert GAME_DATA_SCHEMA.field("entity_type").type == pa.string()
    assert GAME_DATA_SCHEMA.field("id").type == pa.int32()
    assert GAME_DATA_SCHEMA.field("generation").type == pa.int8()


def test_team_sheets_schema_fields():
    assert TEAM_SHEETS_SCHEMA.field("team_id").type == pa.string()
    assert TEAM_SHEETS_SCHEMA.field("placement").type == pa.int16()
    assert TEAM_SHEETS_SCHEMA.field("date").type == pa.date32()


def test_knowledge_schema_fields():
    assert KNOWLEDGE_SCHEMA.field("fact_id").type == pa.string()
    assert KNOWLEDGE_SCHEMA.field("valid_until").type == pa.date32()


def test_cores_schema_has_list_field():
    assert CORES_SCHEMA.field("pokemon").type == pa.list_(pa.string())


def test_calcs_schema_fields():
    assert CALCS_SCHEMA.field("damage_min").type == pa.int16()
    assert CALCS_SCHEMA.field("damage_max_pct").type == pa.float64()
    assert CALCS_SCHEMA.field("ko_chance").type == pa.string()
