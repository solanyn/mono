import json
import uuid
from datetime import datetime, timezone

import httpx
import pyarrow as pa

from datalake import write_bronze, DatalakeConfig, BRONZE_SCHEMA

AEMO_NEM_URL = (
    "https://visualisations.aemo.com.au/aemo/apps/api/report/ELEC_NEM_SUMMARY"
)


def fetch_nem_summary(url: str = AEMO_NEM_URL) -> list[dict]:
    resp = httpx.get(url, follow_redirects=True, timeout=30)
    resp.raise_for_status()
    data = resp.json()
    if isinstance(data, dict):
        return data.get("ELEC_NEM_SUMMARY", [])
    return data


def build_bronze_table(rows: list[dict], batch_id: str | None = None) -> pa.Table:
    batch_id = batch_id or str(uuid.uuid4())
    now = datetime.now(timezone.utc)
    return pa.table(
        {
            "_source": ["aemo.nem_summary"] * len(rows),
            "_ingested_at": pa.array(
                [now] * len(rows), type=pa.timestamp("us", tz="UTC")
            ),
            "_raw_payload": [json.dumps(r) for r in rows],
            "_batch_id": [batch_id] * len(rows),
        },
        schema=BRONZE_SCHEMA,
    )


def ingest(config: DatalakeConfig | None = None) -> str:
    rows = fetch_nem_summary()
    table = build_bronze_table(rows)
    path = write_bronze(table, "aemo", "nem_summary.parquet", config=config)
    print(f"Wrote {len(rows)} rows to {path}")
    return path
