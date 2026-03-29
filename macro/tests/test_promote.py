import json
from datetime import datetime

import pytest
import httpx
import pyarrow as pa

from macro.ingest.rba_csv import fetch_rba_csv, parse_rba_csv, build_bronze_table
from macro.promote.bronze_to_silver import SILVER_RBA_SCHEMA, SERIES_NAMES


@pytest.fixture(scope="module")
def rba_bronze():
    try:
        raw = fetch_rba_csv()
    except (httpx.HTTPStatusError, httpx.TimeoutException) as e:
        pytest.skip(f"RBA endpoint unavailable: {e}")
    rows = parse_rba_csv(raw)
    return build_bronze_table(rows)


def test_rba_silver_schema():
    assert "date" in SILVER_RBA_SCHEMA.names
    assert "series_id" in SILVER_RBA_SCHEMA.names
    assert "series_name" in SILVER_RBA_SCHEMA.names
    assert "value" in SILVER_RBA_SCHEMA.names


def test_rba_bronze_to_silver_transform(rba_bronze):
    payloads = rba_bronze.column("_raw_payload").to_pylist()
    first_row = json.loads(payloads[0])
    series_ids = [k for k in first_row.keys() if k != "Series ID"]

    dates, s_ids, s_names, values = [], [], [], []
    for row_json in payloads:
        row = json.loads(row_json)
        date_str = row.get("Series ID", "")
        try:
            dt = datetime.strptime(date_str, "%d-%b-%Y").date()
        except ValueError:
            continue
        for sid in series_ids:
            val_str = row.get(sid, "")
            if not val_str or val_str.strip() == "":
                continue
            try:
                val = float(val_str)
            except ValueError:
                continue
            dates.append(dt)
            s_ids.append(sid)
            s_names.append(SERIES_NAMES.get(sid, sid))
            values.append(val)

    silver = pa.table(
        {
            "date": pa.array(dates, type=pa.date32()),
            "series_id": s_ids,
            "series_name": s_names,
            "value": values,
        },
        schema=SILVER_RBA_SCHEMA,
    )

    assert silver.num_rows > 1000
    assert silver.schema == SILVER_RBA_SCHEMA
    assert all(isinstance(v, float) for v in silver.column("value").to_pylist())
    assert "FIRMMCRTD" in set(silver.column("series_id").to_pylist())
