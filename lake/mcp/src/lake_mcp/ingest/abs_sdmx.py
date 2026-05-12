import json
import uuid
from datetime import datetime, timezone

import httpx
import pyarrow as pa

from datalake import write_bronze, DatalakeConfig, BRONZE_SCHEMA

ABS_CPI_URL = "https://api.data.abs.gov.au/data/CPI_M/..10.50.M?format=jsondata"


def fetch_abs_cpi(url: str = ABS_CPI_URL) -> dict:
    resp = httpx.get(url, follow_redirects=True, timeout=30)
    resp.raise_for_status()
    return resp.json()


def parse_observations(data: dict) -> list[dict]:
    datasets = data.get("data", {}).get("dataSets", [])
    if not datasets:
        return []

    structures = data.get("data", {}).get("structures", [{}])
    if not structures:
        return []

    struct = structures[0]
    series_dims = struct.get("dimensions", {}).get("series", [])
    obs_dims = struct.get("dimensions", {}).get("observation", [])

    time_dim = None
    for d in obs_dims:
        if d.get("id") == "TIME_PERIOD":
            time_dim = d
            break

    measure_dim = None
    index_dim = None
    for d in series_dims:
        if d.get("id") == "MEASURE":
            measure_dim = d
        elif d.get("id") == "INDEX":
            index_dim = d

    all_series = datasets[0].get("series", {})
    rows = []

    for series_key, series_data in all_series.items():
        key_parts = [int(x) for x in series_key.split(":")]

        measure_name = ""
        if measure_dim and key_parts[0] < len(measure_dim["values"]):
            mv = measure_dim["values"][key_parts[0]]
            measure_name = mv.get("name") or mv.get("names", {}).get("en", "")

        indicator_name = ""
        indicator_id = ""
        if index_dim and len(key_parts) > 1 and key_parts[1] < len(index_dim["values"]):
            iv = index_dim["values"][key_parts[1]]
            indicator_id = iv.get("id", "")
            indicator_name = iv.get("name") or iv.get("names", {}).get("en", "")

        observations = series_data.get("observations", {})
        for obs_key, obs_val in observations.items():
            obs_idx = int(obs_key)
            value = obs_val[0] if obs_val else None

            time_period = ""
            if time_dim and obs_idx < len(time_dim["values"]):
                time_period = time_dim["values"][obs_idx].get("id", "")

            rows.append(
                {
                    "indicator_id": indicator_id,
                    "indicator_name": indicator_name,
                    "unit": measure_name,
                    "time_period": time_period,
                    "frequency": "M",
                    "value": value,
                    "revision_number": obs_val[2]
                    if len(obs_val) > 2 and obs_val[2] is not None
                    else 0,
                }
            )

    return rows


def build_bronze_table(rows: list[dict], batch_id: str | None = None) -> pa.Table:
    batch_id = batch_id or str(uuid.uuid4())
    now = datetime.now(timezone.utc)
    return pa.table(
        {
            "_source": ["abs.cpi_monthly"] * len(rows),
            "_ingested_at": pa.array(
                [now] * len(rows), type=pa.timestamp("us", tz="UTC")
            ),
            "_raw_payload": [json.dumps(r) for r in rows],
            "_batch_id": [batch_id] * len(rows),
        },
        schema=BRONZE_SCHEMA,
    )


def ingest(config: DatalakeConfig | None = None) -> str:
    data = fetch_abs_cpi()
    rows = parse_observations(data)
    table = build_bronze_table(rows)
    path = write_bronze(table, "abs", "cpi_monthly.parquet", config=config)
    print(f"Wrote {len(rows)} rows to {path}")
    return path
