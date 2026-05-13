# libs/datalake

Shared Python library for reading/writing Parquet files to S3 medallion layers.

```mermaid
graph LR
    caller["lake / lake-mcp"] --> lib["datalake"]
    lib --> pyarrow["PyArrow"]
    lib --> s3fs["s3fs / boto3"]
    pyarrow --> s3["S3 (bronze/silver/gold)"]
    s3fs --> s3
```

## Install

```bash
uv pip install -e libs/datalake
```

## Config

Env vars: `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_REGION`

## Modules

`client.py` (S3 client), `reader.py` / `writer.py` (Parquet I/O), `config.py`, `schemas.py`
