import json
from datetime import datetime, timezone

import pyarrow as pa

from datalake import DatalakeConfig, read_parquet, write_silver

SILVER_ABS_SCHEMA = pa.schema(
    [
        pa.field("indicator_id", pa.string()),
        pa.field("indicator_name", pa.string()),
        pa.field("period", pa.string()),
        pa.field("frequency", pa.string()),
        pa.field("value", pa.float64()),
        pa.field("unit", pa.string()),
        pa.field("revision_number", pa.int32()),
        pa.field("release_date", pa.string()),
    ]
)


def promote_abs(
    bronze_path: str | None = None,
    config: DatalakeConfig | None = None,
) -> str:
    config = config or DatalakeConfig()

    if bronze_path is None:
        now = datetime.now(timezone.utc)
        bronze_path = f"bronze/abs/{now.year:04d}/{now.month:02d}/{now.day:02d}/cpi_monthly.parquet"

    bronze = read_parquet(bronze_path, config)
    payloads = bronze.column("_raw_payload").to_pylist()

    if not payloads:
        raise ValueError(f"No rows in {bronze_path}")

    indicator_ids = []
    indicator_names = []
    periods = []
    frequencies = []
    values = []
    units = []
    revision_numbers = []
    release_dates = []

    for row_json in payloads:
        row = json.loads(row_json)
        val = row.get("value")
        if val is None:
            continue
        try:
            val = float(val)
        except (ValueError, TypeError):
            continue

        indicator_ids.append(row.get("indicator_id", ""))
        indicator_names.append(row.get("indicator_name", ""))
        periods.append(row.get("time_period", ""))
        frequencies.append(row.get("frequency", "M"))
        values.append(val)
        units.append(row.get("unit", "Index Numbers"))
        rev = row.get("revision_number", 0)
        revision_numbers.append(int(rev) if rev is not None else 0)
        release_dates.append(row.get("release_date", ""))

    table = pa.table(
        {
            "indicator_id": indicator_ids,
            "indicator_name": indicator_names,
            "period": periods,
            "frequency": frequencies,
            "value": values,
            "unit": units,
            "revision_number": pa.array(revision_numbers, type=pa.int32()),
            "release_date": release_dates,
        },
        schema=SILVER_ABS_SCHEMA,
    )

    path = write_silver(
        table, "abs_indicators", "abs_indicators.parquet", config=config
    )
    print(f"Promoted {table.num_rows} rows to {path}")
    return path
