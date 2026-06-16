from __future__ import annotations

import os

import pyarrow as pa
import pyarrow.parquet as pq

from schema.iceberg import get_catalog, ensure_tables


def _should_write_lake() -> bool:
    return bool(os.getenv("ICEBERG_CATALOG_URI"))


def write_to_lake(table: pa.Table, table_name: str):
    if not _should_write_lake():
        return

    catalog = get_catalog()
    ensure_tables(catalog)

    iceberg_table = catalog.load_table(f"vgc.{table_name}")
    iceberg_table.append(table)


def write_parquet_to_lake(parquet_path: str, table_name: str):
    if not _should_write_lake():
        return

    table = pq.read_table(parquet_path)
    write_to_lake(table, table_name)
