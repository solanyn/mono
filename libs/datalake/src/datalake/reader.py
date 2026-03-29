import pyarrow.parquet as pq

from datalake.client import get_s3fs
from datalake.config import DatalakeConfig


def read_parquet(path: str, config: DatalakeConfig | None = None):
    fs = get_s3fs(config)
    with fs.open(path, "rb") as f:
        return pq.read_table(f)
