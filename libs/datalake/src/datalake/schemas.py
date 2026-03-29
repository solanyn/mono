import pyarrow as pa

BRONZE_SCHEMA = pa.schema(
    [
        pa.field("_source", pa.string()),
        pa.field("_ingested_at", pa.timestamp("us", tz="UTC")),
        pa.field("_raw_payload", pa.string()),
        pa.field("_batch_id", pa.string()),
    ]
)

SILVER_SCHEMA = pa.schema(
    [
        pa.field("source", pa.string()),
        pa.field("series_id", pa.string()),
        pa.field("date", pa.date32()),
        pa.field("value", pa.float64()),
        pa.field("unit", pa.string()),
        pa.field("frequency", pa.string()),
    ]
)
