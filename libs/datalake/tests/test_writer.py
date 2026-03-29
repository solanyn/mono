import pyarrow as pa

from datalake.schemas import BRONZE_SCHEMA, SILVER_SCHEMA
from datalake.writer import _partition_path

from datetime import datetime, timezone


def test_bronze_schema_has_required_fields():
    names = set(BRONZE_SCHEMA.names)
    assert "_source" in names
    assert "_ingested_at" in names
    assert "_raw_payload" in names
    assert "_batch_id" in names


def test_silver_schema_has_required_fields():
    names = set(SILVER_SCHEMA.names)
    assert "source" in names
    assert "series_id" in names
    assert "date" in names
    assert "value" in names


def test_partition_path_structure():
    dt = datetime(2026, 3, 15, 10, 0, 0, tzinfo=timezone.utc)
    path = _partition_path("bronze", "rba", dt)
    assert path == "bronze/rba/2026/03/15"


def test_partition_path_defaults_to_now():
    path = _partition_path("silver", "test")
    now = datetime.now(timezone.utc)
    assert path.startswith(f"silver/test/{now.year:04d}/{now.month:02d}/{now.day:02d}")


def test_bronze_table_creation():
    table = pa.table(
        {
            "_source": ["test"],
            "_ingested_at": pa.array(
                [datetime.now(timezone.utc)], type=pa.timestamp("us", tz="UTC")
            ),
            "_raw_payload": ['{"key": "value"}'],
            "_batch_id": ["batch-1"],
        },
        schema=BRONZE_SCHEMA,
    )
    assert table.num_rows == 1
    assert table.schema == BRONZE_SCHEMA
