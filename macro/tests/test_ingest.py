import json

import pytest
import httpx
import pyarrow as pa

from macro.ingest.rba_csv import fetch_rba_csv, parse_rba_csv, build_bronze_table
from macro.ingest.abs_sdmx import fetch_abs_cpi, parse_observations as parse_abs
from macro.ingest.abs_sdmx import build_bronze_table as build_abs_bronze
from macro.ingest.aemo import fetch_nem_summary, build_bronze_table as build_aemo_bronze
from datalake.schemas import BRONZE_SCHEMA


@pytest.fixture(scope="module")
def rba_raw():
    try:
        return fetch_rba_csv()
    except (httpx.HTTPStatusError, httpx.TimeoutException) as e:
        pytest.skip(f"RBA endpoint unavailable: {e}")


@pytest.fixture(scope="module")
def abs_data():
    try:
        return fetch_abs_cpi()
    except (httpx.HTTPStatusError, httpx.TimeoutException) as e:
        pytest.skip(f"ABS endpoint unavailable: {e}")


@pytest.fixture(scope="module")
def aemo_rows():
    try:
        return fetch_nem_summary()
    except (httpx.HTTPStatusError, httpx.TimeoutException) as e:
        pytest.skip(f"AEMO endpoint unavailable: {e}")


def test_rba_csv_fetch_and_parse(rba_raw):
    assert len(rba_raw) > 0
    rows = parse_rba_csv(rba_raw)
    assert len(rows) > 100
    assert "Series ID" in rows[0]


def test_rba_bronze_table_schema(rba_raw):
    rows = parse_rba_csv(rba_raw)
    table = build_bronze_table(rows)
    assert table.schema == BRONZE_SCHEMA
    assert table.num_rows == len(rows)
    parsed = json.loads(table.column("_raw_payload")[0].as_py())
    assert isinstance(parsed, dict)


def test_abs_sdmx_fetch_and_parse(abs_data):
    rows = parse_abs(abs_data)
    assert len(rows) > 0
    first = rows[0]
    assert "indicator_id" in first
    assert "indicator_name" in first
    assert "time_period" in first
    assert "value" in first


def test_abs_bronze_table_schema(abs_data):
    rows = parse_abs(abs_data)
    table = build_abs_bronze(rows)
    assert table.schema == BRONZE_SCHEMA
    assert table.num_rows == len(rows)


def test_aemo_fetch_and_parse(aemo_rows):
    assert len(aemo_rows) == 5
    regions = {r["REGIONID"] for r in aemo_rows}
    assert regions == {"NSW1", "QLD1", "SA1", "TAS1", "VIC1"}


def test_aemo_bronze_table_schema(aemo_rows):
    table = build_aemo_bronze(aemo_rows)
    assert table.schema == BRONZE_SCHEMA
    assert table.num_rows == len(aemo_rows)
