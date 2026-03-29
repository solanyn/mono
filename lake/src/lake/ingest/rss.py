import json
import uuid
from datetime import datetime, timezone

import feedparser
import httpx
import pyarrow as pa

from datalake import write_bronze, DatalakeConfig, BRONZE_SCHEMA

FEEDS = {
    "guardian_au_business": "https://www.theguardian.com/au/business/rss",
    "rba_media_releases": "https://www.rba.gov.au/rss/rss-cb-media-releases.xml",
    "rba_speeches": "https://www.rba.gov.au/rss/rss-cb-speeches.xml",
}


def fetch_feeds(feeds: dict[str, str] | None = None, timeout: int = 10) -> list[dict]:
    feeds = feeds or FEEDS
    articles = []
    for source_id, url in feeds.items():
        try:
            resp = httpx.get(url, follow_redirects=True, timeout=timeout)
            resp.raise_for_status()
            feed = feedparser.parse(resp.text)
        except Exception as e:
            print(f"Warning: failed to fetch {source_id} ({url}): {e}")
            continue

        for entry in feed.entries:
            published = ""
            if hasattr(entry, "published_parsed") and entry.published_parsed:
                published = datetime(
                    *entry.published_parsed[:6], tzinfo=timezone.utc
                ).isoformat()
            elif hasattr(entry, "updated_parsed") and entry.updated_parsed:
                published = datetime(
                    *entry.updated_parsed[:6], tzinfo=timezone.utc
                ).isoformat()

            articles.append(
                {
                    "title": getattr(entry, "title", ""),
                    "url": getattr(entry, "link", ""),
                    "published_at": published,
                    "source": source_id,
                    "summary": getattr(entry, "summary", ""),
                }
            )
    return articles


def build_bronze_table(articles: list[dict], batch_id: str | None = None) -> pa.Table:
    batch_id = batch_id or str(uuid.uuid4())
    now = datetime.now(timezone.utc)
    return pa.table(
        {
            "_source": [f"rss.{a['source']}" for a in articles],
            "_ingested_at": pa.array(
                [now] * len(articles), type=pa.timestamp("us", tz="UTC")
            ),
            "_raw_payload": [json.dumps(a) for a in articles],
            "_batch_id": [batch_id] * len(articles),
        },
        schema=BRONZE_SCHEMA,
    )


def ingest(config: DatalakeConfig | None = None) -> str:
    articles = fetch_feeds()
    if not articles:
        print("No articles fetched from any feed")
        return ""
    table = build_bronze_table(articles)
    path = write_bronze(table, "rss", "news.parquet", config=config)
    print(f"Wrote {len(articles)} articles to {path}")
    return path
