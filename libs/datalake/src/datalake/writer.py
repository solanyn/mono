from datetime import datetime, timezone

import pyarrow as pa
import pyarrow.parquet as pq

from datalake.client import get_s3fs
from datalake.config import DatalakeConfig


def _partition_path(bucket: str, prefix: str, dt: datetime | None = None) -> str:
    dt = dt or datetime.now(timezone.utc)
    return f"{bucket}/{prefix}/{dt.year:04d}/{dt.month:02d}/{dt.day:02d}"


def write_bronze(
    table: pa.Table,
    source: str,
    filename: str,
    config: DatalakeConfig | None = None,
    dt: datetime | None = None,
) -> str:
    fs = get_s3fs(config)
    path = f"{_partition_path('bronze', source, dt)}/{filename}"
    with fs.open(path, "wb") as f:
        pq.write_table(table, f)
    return path


def write_silver(
    table: pa.Table,
    source: str,
    filename: str,
    config: DatalakeConfig | None = None,
    dt: datetime | None = None,
) -> str:
    fs = get_s3fs(config)
    path = f"{_partition_path('silver', source, dt)}/{filename}"
    with fs.open(path, "wb") as f:
        pq.write_table(table, f)
    return path


def write_gold(
    table: pa.Table,
    source: str,
    filename: str,
    config: DatalakeConfig | None = None,
    dt: datetime | None = None,
) -> str:
    fs = get_s3fs(config)
    path = f"{_partition_path('gold', source, dt)}/{filename}"
    with fs.open(path, "wb") as f:
        pq.write_table(table, f)
    return path
