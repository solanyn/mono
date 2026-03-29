import json
from datetime import datetime, timezone

import pyarrow as pa

from datalake import DatalakeConfig, read_parquet, write_silver

SILVER_REDDIT_SCHEMA = pa.schema(
    [
        pa.field("captured_at", pa.timestamp("us", tz="UTC")),
        pa.field("post_id", pa.string()),
        pa.field("title", pa.string()),
        pa.field("score", pa.int64()),
        pa.field("num_comments", pa.int64()),
        pa.field("upvote_ratio", pa.float64()),
        pa.field("flair", pa.string()),
        pa.field("key_topics", pa.list_(pa.string())),
    ]
)


def promote_reddit(
    bronze_path: str | None = None,
    config: DatalakeConfig | None = None,
) -> str:
    config = config or DatalakeConfig()

    if bronze_path is None:
        now = datetime.now(timezone.utc)
        bronze_path = f"bronze/reddit/{now.year:04d}/{now.month:02d}/{now.day:02d}/ausfinance.parquet"

    bronze = read_parquet(bronze_path, config)
    payloads = bronze.column("_raw_payload").to_pylist()
    ingested_ats = bronze.column("_ingested_at").to_pylist()

    if not payloads:
        raise ValueError(f"No rows in {bronze_path}")

    captured_ats = []
    post_ids = []
    titles = []
    scores = []
    num_comments = []
    upvote_ratios = []
    flairs = []
    key_topics = []

    for row_json, ingested_at in zip(payloads, ingested_ats):
        row = json.loads(row_json)
        captured_ats.append(ingested_at)
        post_ids.append(row.get("post_id", ""))
        titles.append(row.get("title", ""))
        scores.append(int(row.get("score", 0)))
        num_comments.append(int(row.get("num_comments", 0)))
        upvote_ratios.append(float(row.get("upvote_ratio", 0.0)))
        flairs.append(row.get("flair", ""))
        key_topics.append([])

    table = pa.table(
        {
            "captured_at": pa.array(captured_ats, type=pa.timestamp("us", tz="UTC")),
            "post_id": post_ids,
            "title": titles,
            "score": pa.array(scores, type=pa.int64()),
            "num_comments": pa.array(num_comments, type=pa.int64()),
            "upvote_ratio": pa.array(upvote_ratios, type=pa.float64()),
            "flair": flairs,
            "key_topics": pa.array(key_topics, type=pa.list_(pa.string())),
        },
        schema=SILVER_REDDIT_SCHEMA,
    )

    path = write_silver(
        table, "reddit_sentiment", "reddit_sentiment.parquet", config=config
    )
    print(f"Promoted {table.num_rows} posts to {path}")
    return path
