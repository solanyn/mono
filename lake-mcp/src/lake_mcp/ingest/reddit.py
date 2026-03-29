import json
import uuid
from datetime import datetime, timezone

import httpx
import pyarrow as pa

from datalake import write_bronze, DatalakeConfig, BRONZE_SCHEMA

SUBREDDIT_URLS = {
    "hot": "https://www.reddit.com/r/AusFinance/hot.json?limit=25",
    "new": "https://www.reddit.com/r/AusFinance/new.json?limit=25",
}

HEADERS = {
    "User-Agent": "lake-agent/0.1 (macro-financial data lake)",
}


def fetch_reddit(urls: dict[str, str] | None = None, timeout: int = 15) -> list[dict]:
    urls = urls or SUBREDDIT_URLS
    posts = []
    seen = set()
    for sort, url in urls.items():
        try:
            resp = httpx.get(
                url, headers=HEADERS, follow_redirects=True, timeout=timeout
            )
            resp.raise_for_status()
            data = resp.json()
        except Exception as e:
            print(f"Warning: failed to fetch r/AusFinance/{sort}: {e}")
            continue

        for child in data.get("data", {}).get("children", []):
            post = child.get("data", {})
            post_id = post.get("id", "")
            if post_id in seen:
                continue
            seen.add(post_id)
            posts.append(
                {
                    "post_id": post_id,
                    "title": post.get("title", ""),
                    "score": post.get("score", 0),
                    "num_comments": post.get("num_comments", 0),
                    "upvote_ratio": post.get("upvote_ratio", 0.0),
                    "flair": post.get("link_flair_text", "") or "",
                    "selftext": post.get("selftext", ""),
                    "url": post.get("url", ""),
                    "created_utc": post.get("created_utc", 0),
                }
            )
    return posts


def build_bronze_table(posts: list[dict], batch_id: str | None = None) -> pa.Table:
    batch_id = batch_id or str(uuid.uuid4())
    now = datetime.now(timezone.utc)
    return pa.table(
        {
            "_source": ["reddit.ausfinance"] * len(posts),
            "_ingested_at": pa.array(
                [now] * len(posts), type=pa.timestamp("us", tz="UTC")
            ),
            "_raw_payload": [json.dumps(p) for p in posts],
            "_batch_id": [batch_id] * len(posts),
        },
        schema=BRONZE_SCHEMA,
    )


def ingest(config: DatalakeConfig | None = None) -> str:
    posts = fetch_reddit()
    if not posts:
        print("No posts fetched from r/AusFinance")
        return ""
    table = build_bronze_table(posts)
    path = write_bronze(table, "reddit", "ausfinance.parquet", config=config)
    print(f"Wrote {len(posts)} posts to {path}")
    return path
