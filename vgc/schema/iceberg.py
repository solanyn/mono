from __future__ import annotations

import os

from pyiceberg.catalog import load_catalog
from pyiceberg.schema import Schema
from pyiceberg.types import (
    StringType,
    IntegerType,
    LongType,
    FloatType,
    DoubleType,
    TimestamptzType,
    DateType,
    ListType,
    NestedField,
)
from pyiceberg.partitioning import PartitionSpec, PartitionField
from pyiceberg.transforms import MonthTransform, IdentityTransform


def get_catalog():
    return load_catalog(
        "lakekeeper",
        **{
            "type": "rest",
            "uri": os.getenv("ICEBERG_CATALOG_URI", "http://lakekeeper.storage.svc.cluster.local:8181/catalog"),
            "s3.endpoint": os.getenv("S3_ENDPOINT", "http://garage.storage.svc.cluster.local:3900"),
            "s3.access-key-id": os.getenv("S3_ACCESS_KEY", ""),
            "s3.secret-access-key": os.getenv("S3_SECRET_KEY", ""),
            "s3.region": os.getenv("S3_REGION", "us-east-1"),
            "s3.path-style-access": "true",
            "warehouse": "vgc",
        },
    )


GAME_DATA_ICEBERG_SCHEMA = Schema(
    NestedField(1, "entity_type", StringType(), required=True),
    NestedField(2, "name", StringType(), required=True),
    NestedField(3, "id", IntegerType()),
    NestedField(4, "data", StringType()),
    NestedField(5, "generation", IntegerType()),
    NestedField(6, "synced_at", TimestamptzType(), required=True),
)

USAGE_STATS_ICEBERG_SCHEMA = Schema(
    NestedField(1, "game", StringType(), required=True),
    NestedField(2, "regulation", StringType(), required=True),
    NestedField(3, "format", StringType(), required=True),
    NestedField(4, "period", StringType(), required=True),
    NestedField(5, "elo_bracket", IntegerType(), required=True),
    NestedField(6, "pokemon", StringType(), required=True),
    NestedField(7, "rank", IntegerType()),
    NestedField(8, "usage_pct", DoubleType()),
    NestedField(9, "raw_count", LongType()),
    NestedField(10, "abilities", StringType()),
    NestedField(11, "items", StringType()),
    NestedField(12, "spreads", StringType()),
    NestedField(13, "moves", StringType()),
    NestedField(14, "tera_types", StringType()),
    NestedField(15, "teammates", StringType()),
    NestedField(16, "checks_counters", StringType()),
    NestedField(17, "viability_ceiling", IntegerType()),
    NestedField(18, "ingested_at", TimestamptzType(), required=True),
)

TEAM_SHEETS_ICEBERG_SCHEMA = Schema(
    NestedField(1, "team_id", StringType(), required=True),
    NestedField(2, "source", StringType(), required=True),
    NestedField(3, "source_url", StringType()),
    NestedField(4, "game", StringType(), required=True),
    NestedField(5, "regulation", StringType(), required=True),
    NestedField(6, "event_name", StringType()),
    NestedField(7, "placement", IntegerType()),
    NestedField(8, "player_name", StringType()),
    NestedField(9, "date", DateType()),
    NestedField(10, "pokemon", StringType()),
    NestedField(11, "ingested_at", TimestamptzType(), required=True),
)

KNOWLEDGE_ICEBERG_SCHEMA = Schema(
    NestedField(1, "fact_id", StringType(), required=True),
    NestedField(2, "fact_type", StringType(), required=True),
    NestedField(3, "game", StringType()),
    NestedField(4, "regulation", StringType()),
    NestedField(5, "content", StringType()),
    NestedField(6, "confidence", StringType()),
    NestedField(7, "source_type", StringType()),
    NestedField(8, "source_url", StringType()),
    NestedField(9, "source_timestamp", StringType()),
    NestedField(10, "source_channel", StringType()),
    NestedField(11, "valid_until", DateType()),
    NestedField(12, "extracted_at", TimestamptzType(), required=True),
)


def ensure_tables(catalog):
    namespace = "vgc"
    try:
        catalog.create_namespace(namespace)
    except Exception:
        pass

    tables = {
        f"{namespace}.game_data": GAME_DATA_ICEBERG_SCHEMA,
        f"{namespace}.usage_stats": USAGE_STATS_ICEBERG_SCHEMA,
        f"{namespace}.team_sheets": TEAM_SHEETS_ICEBERG_SCHEMA,
        f"{namespace}.knowledge": KNOWLEDGE_ICEBERG_SCHEMA,
    }

    for table_name, schema in tables.items():
        try:
            catalog.create_table(table_name, schema=schema)
        except Exception:
            pass


if __name__ == "__main__":
    catalog = get_catalog()
    ensure_tables(catalog)
    print("Tables ensured in vgc namespace")
    for t in catalog.list_tables("vgc"):
        print(f"  {t}")
