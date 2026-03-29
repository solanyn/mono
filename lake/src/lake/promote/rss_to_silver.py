import json
from datetime import datetime, timezone

import pyarrow as pa

from datalake import DatalakeConfig, read_parquet, write_silver

SILVER_NEWS_SCHEMA = pa.schema(
    [
        pa.field("published_at", pa.timestamp("us", tz="UTC")),
        pa.field("source", pa.string()),
        pa.field("title", pa.string()),
        pa.field("url", pa.string()),
        pa.field("summary", pa.string()),
        pa.field("full_text", pa.string()),
    ]
)


def promote_rss(
    bronze_path: str | None = None,
    config: DatalakeConfig | None = None,
) -> str:
    config = config or DatalakeConfig()

    if bronze_path is None:
        now = datetime.now(timezone.utc)
        bronze_path = (
            f"bronze/rss/{now.year:04d}/{now.month:02d}/{now.day:02d}/news.parquet"
        )

    bronze = read_parquet(bronze_path, config)
    payloads = bronze.column("_raw_payload").to_pylist()

    if not payloads:
        raise ValueError(f"No rows in {bronze_path}")

    published_ats = []
    sources = []
    titles = []
    urls = []
    summaries = []
    full_texts = []

    for row_json in payloads:
        row = json.loads(row_json)
        pub_str = row.get("published_at", "")
        if pub_str:
            try:
                ts = datetime.fromisoformat(pub_str)
                if ts.tzinfo is None:
                    ts = ts.replace(tzinfo=timezone.utc)
            except ValueError:
                ts = datetime.now(timezone.utc)
        else:
            ts = datetime.now(timezone.utc)

        published_ats.append(ts)
        sources.append(row.get("source", ""))
        titles.append(row.get("title", ""))
        urls.append(row.get("url", ""))
        summaries.append(row.get("summary", ""))
        full_texts.append("")

    table = pa.table(
        {
            "published_at": pa.array(published_ats, type=pa.timestamp("us", tz="UTC")),
            "source": sources,
            "title": titles,
            "url": urls,
            "summary": summaries,
            "full_text": full_texts,
        },
        schema=SILVER_NEWS_SCHEMA,
    )

    path = write_silver(table, "news_articles", "news_articles.parquet", config=config)
    print(f"Promoted {table.num_rows} articles to {path}")
    return path
