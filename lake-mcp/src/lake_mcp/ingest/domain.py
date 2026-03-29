import json
import os
import time
import uuid
from datetime import datetime, timezone

import httpx
import pyarrow as pa

from datalake import write_bronze, DatalakeConfig, BRONZE_SCHEMA

AUTH_URL = "https://auth.domain.com.au/v1/connect/token"
API_BASE = "https://api.domain.com.au/v1"
REQUEST_DELAY = 0.5


def _get_token(client_id: str, client_secret: str) -> str:
    resp = httpx.post(
        AUTH_URL,
        data={
            "grant_type": "client_credentials",
            "client_id": client_id,
            "client_secret": client_secret,
            "scope": "api_listings_read api_salesresults_read",
        },
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json()["access_token"]


def _api_get(endpoint: str, token: str, params: dict | None = None) -> dict | list:
    resp = httpx.get(
        f"{API_BASE}{endpoint}",
        headers={"Authorization": f"Bearer {token}"},
        params=params,
        timeout=30,
    )
    resp.raise_for_status()
    return resp.json()


def fetch_auction_results(token: str, city: str = "Sydney") -> list[dict]:
    data = _api_get(f"/salesResults/{city}", token)
    results = []
    for result in data if isinstance(data, list) else [data]:
        for listing in result.get("results", []):
            results.append(
                {
                    "listing_id": str(listing.get("id", "")),
                    "suburb": listing.get("suburb", ""),
                    "state": listing.get("state", ""),
                    "postcode": listing.get("postcode", ""),
                    "property_type": listing.get("propertyType", ""),
                    "bedrooms": listing.get("bedrooms"),
                    "bathrooms": listing.get("bathrooms"),
                    "price_guide": listing.get("price", ""),
                    "auction_date": result.get("auctionedDate", ""),
                    "sold_price": listing.get("reportedPrice"),
                    "days_on_market": listing.get("daysOnMarket"),
                    "latitude": listing.get("geoLocation", {}).get("latitude"),
                    "longitude": listing.get("geoLocation", {}).get("longitude"),
                    "source": "auction_results",
                }
            )
    return results


def fetch_listings(token: str, suburbs: list[str] | None = None) -> list[dict]:
    suburbs = suburbs or ["Sydney", "Parramatta", "Chatswood", "Bondi"]
    results = []
    for suburb in suburbs:
        time.sleep(REQUEST_DELAY)
        body = {
            "listingType": "Sale",
            "locations": [{"suburb": suburb, "state": "NSW"}],
            "pageSize": 50,
        }
        try:
            resp = httpx.post(
                f"{API_BASE}/listings/residential/_search",
                headers={"Authorization": f"Bearer {token}"},
                json=body,
                timeout=30,
            )
            resp.raise_for_status()
            data = resp.json()
        except Exception as e:
            print(f"Warning: failed to fetch listings for {suburb}: {e}")
            continue

        for item in data:
            listing = item.get("listing", {})
            prop = listing.get("propertyDetails", {})
            geo = prop.get("latitude"), prop.get("longitude")
            price_details = listing.get("priceDetails", {})
            results.append(
                {
                    "listing_id": str(listing.get("id", "")),
                    "suburb": prop.get("suburb", suburb),
                    "state": prop.get("state", "NSW"),
                    "postcode": prop.get("postcode", ""),
                    "property_type": prop.get("propertyType", ""),
                    "bedrooms": prop.get("bedrooms"),
                    "bathrooms": prop.get("bathrooms"),
                    "price_guide": price_details.get("displayPrice", ""),
                    "auction_date": listing.get("auctionSchedule", {}).get(
                        "auctionSchedule", ""
                    ),
                    "sold_price": None,
                    "days_on_market": listing.get("dateListed"),
                    "latitude": geo[0],
                    "longitude": geo[1],
                    "source": "listings_search",
                }
            )
    return results


def build_bronze_table(rows: list[dict], batch_id: str | None = None) -> pa.Table:
    batch_id = batch_id or str(uuid.uuid4())
    now = datetime.now(timezone.utc)
    return pa.table(
        {
            "_source": [f"domain.{r.get('source', 'unknown')}" for r in rows],
            "_ingested_at": pa.array(
                [now] * len(rows), type=pa.timestamp("us", tz="UTC")
            ),
            "_raw_payload": [json.dumps(r) for r in rows],
            "_batch_id": [batch_id] * len(rows),
        },
        schema=BRONZE_SCHEMA,
    )


def ingest(config: DatalakeConfig | None = None) -> str:
    client_id = os.environ.get("DOMAIN_CLIENT_ID", "")
    client_secret = os.environ.get("DOMAIN_CLIENT_SECRET", "")
    if not client_id or not client_secret:
        print("Warning: DOMAIN_CLIENT_ID/DOMAIN_CLIENT_SECRET not set, skipping")
        return ""

    token = _get_token(client_id, client_secret)

    rows = []
    auction = fetch_auction_results(token)
    rows.extend(auction)
    print(f"Fetched {len(auction)} auction results")

    time.sleep(REQUEST_DELAY)

    listings = fetch_listings(token)
    rows.extend(listings)
    print(f"Fetched {len(listings)} listings")

    if not rows:
        print("No Domain data fetched")
        return ""

    table = build_bronze_table(rows)
    path = write_bronze(table, "domain", "listings.parquet", config=config)
    print(f"Wrote {len(rows)} rows to {path}")
    return path
