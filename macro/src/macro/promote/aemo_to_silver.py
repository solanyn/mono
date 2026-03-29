import json
from datetime import datetime, timezone

import pyarrow as pa

from datalake import DatalakeConfig, read_parquet, write_silver

SILVER_AEMO_SCHEMA = pa.schema(
    [
        pa.field("timestamp", pa.timestamp("us", tz="UTC")),
        pa.field("region", pa.string()),
        pa.field("price_aud_mwh", pa.float64()),
        pa.field("demand_mw", pa.float64()),
        pa.field("generation_mw", pa.float64()),
    ]
)


def promote_aemo(
    bronze_path: str | None = None,
    config: DatalakeConfig | None = None,
) -> str:
    config = config or DatalakeConfig()

    if bronze_path is None:
        now = datetime.now(timezone.utc)
        bronze_path = f"bronze/aemo/{now.year:04d}/{now.month:02d}/{now.day:02d}/nem_summary.parquet"

    bronze = read_parquet(bronze_path, config)
    payloads = bronze.column("_raw_payload").to_pylist()

    if not payloads:
        raise ValueError(f"No rows in {bronze_path}")

    timestamps = []
    regions = []
    prices = []
    demands = []
    generations = []

    for row_json in payloads:
        row = json.loads(row_json)
        ts_str = row.get("SETTLEMENTDATE")
        if not ts_str:
            continue
        try:
            ts = datetime.fromisoformat(ts_str).replace(tzinfo=timezone.utc)
        except ValueError:
            continue

        region = row.get("REGIONID", "")
        price = row.get("PRICE")
        demand = row.get("TOTALDEMAND")
        sched = row.get("SCHEDULEDGENERATION", 0) or 0
        semi = row.get("SEMISCHEDULEDGENERATION", 0) or 0

        timestamps.append(ts)
        regions.append(region)
        prices.append(float(price) if price is not None else None)
        demands.append(float(demand) if demand is not None else None)
        generations.append(float(sched) + float(semi))

    table = pa.table(
        {
            "timestamp": pa.array(timestamps, type=pa.timestamp("us", tz="UTC")),
            "region": regions,
            "price_aud_mwh": pa.array(prices, type=pa.float64()),
            "demand_mw": pa.array(demands, type=pa.float64()),
            "generation_mw": pa.array(generations, type=pa.float64()),
        },
        schema=SILVER_AEMO_SCHEMA,
    )

    path = write_silver(table, "aemo_prices", "aemo_prices.parquet", config=config)
    print(f"Promoted {table.num_rows} rows to {path}")
    return path
