import json
import uuid
from datetime import datetime, timezone

import httpx
import pyarrow as pa

from datalake import write_bronze, DatalakeConfig, BRONZE_SCHEMA

RBA_CSV_URL = "https://rba.gov.au/statistics/tables/csv/f1-data.csv"


def fetch_rba_csv(url: str = RBA_CSV_URL) -> str:
    resp = httpx.get(url, follow_redirects=True, timeout=30)
    resp.raise_for_status()
    return resp.text


def parse_rba_csv(raw: str) -> list[dict]:
    lines = raw.splitlines()
    header_idx = None
    for i, line in enumerate(lines):
        if line.startswith("Series ID"):
            header_idx = i
            break
    if header_idx is None:
        raise ValueError("Could not find header row starting with 'Series ID'")

    headers = lines[header_idx].split(",")
    rows = []
    for line in lines[header_idx + 1 :]:
        if not line.strip():
            continue
        parts = line.split(",")
        row = dict(zip(headers, parts))
        rows.append(row)
    return rows


def build_bronze_table(rows: list[dict], batch_id: str | None = None) -> pa.Table:
    batch_id = batch_id or str(uuid.uuid4())
    now = datetime.now(timezone.utc)
    sources = []
    ingested = []
    payloads = []
    batch_ids = []
    for row in rows:
        sources.append("rba.f1")
        ingested.append(now)
        payloads.append(json.dumps(row))
        batch_ids.append(batch_id)
    return pa.table(
        {
            "_source": sources,
            "_ingested_at": pa.array(ingested, type=pa.timestamp("us", tz="UTC")),
            "_raw_payload": payloads,
            "_batch_id": batch_ids,
        },
        schema=BRONZE_SCHEMA,
    )


def ingest(config: DatalakeConfig | None = None) -> str:
    raw = fetch_rba_csv()
    rows = parse_rba_csv(raw)
    table = build_bronze_table(rows)
    path = write_bronze(table, "rba", "f1-data.parquet", config=config)
    print(f"Wrote {len(rows)} rows to {path}")
    return path
