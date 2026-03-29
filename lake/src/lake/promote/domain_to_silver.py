import json
from datetime import datetime, timezone

import pyarrow as pa

from datalake import DatalakeConfig, read_parquet, write_silver

SILVER_DOMAIN_SCHEMA = pa.schema(
    [
        pa.field("listing_id", pa.string()),
        pa.field("suburb", pa.string()),
        pa.field("state", pa.string()),
        pa.field("postcode", pa.string()),
        pa.field("property_type", pa.string()),
        pa.field("bedrooms", pa.int32()),
        pa.field("bathrooms", pa.int32()),
        pa.field("price_guide", pa.string()),
        pa.field("auction_date", pa.string()),
        pa.field("sold_price", pa.float64()),
        pa.field("days_on_market", pa.int32()),
        pa.field("latitude", pa.float64()),
        pa.field("longitude", pa.float64()),
    ]
)


def promote_domain(
    bronze_path: str | None = None,
    config: DatalakeConfig | None = None,
) -> str:
    config = config or DatalakeConfig()

    if bronze_path is None:
        now = datetime.now(timezone.utc)
        bronze_path = f"bronze/domain/{now.year:04d}/{now.month:02d}/{now.day:02d}/listings.parquet"

    bronze = read_parquet(bronze_path, config)
    payloads = bronze.column("_raw_payload").to_pylist()

    if not payloads:
        raise ValueError(f"No rows in {bronze_path}")

    listing_ids = []
    suburbs = []
    states = []
    postcodes = []
    property_types = []
    bedrooms = []
    bathrooms = []
    price_guides = []
    auction_dates = []
    sold_prices = []
    days_on_market = []
    latitudes = []
    longitudes = []

    for row_json in payloads:
        row = json.loads(row_json)
        lid = row.get("listing_id", "")
        if not lid:
            continue

        listing_ids.append(lid)
        suburbs.append(row.get("suburb", ""))
        states.append(row.get("state", ""))
        postcodes.append(row.get("postcode", ""))
        property_types.append(row.get("property_type", ""))

        beds = row.get("bedrooms")
        bedrooms.append(int(beds) if beds is not None else None)
        baths = row.get("bathrooms")
        bathrooms.append(int(baths) if baths is not None else None)

        price_guides.append(row.get("price_guide", ""))
        auction_dates.append(row.get("auction_date", ""))

        sp = row.get("sold_price")
        sold_prices.append(float(sp) if sp is not None else None)

        dom = row.get("days_on_market")
        if isinstance(dom, int):
            days_on_market.append(dom)
        else:
            days_on_market.append(None)

        lat = row.get("latitude")
        latitudes.append(float(lat) if lat is not None else None)
        lon = row.get("longitude")
        longitudes.append(float(lon) if lon is not None else None)

    table = pa.table(
        {
            "listing_id": listing_ids,
            "suburb": suburbs,
            "state": states,
            "postcode": postcodes,
            "property_type": property_types,
            "bedrooms": pa.array(bedrooms, type=pa.int32()),
            "bathrooms": pa.array(bathrooms, type=pa.int32()),
            "price_guide": price_guides,
            "auction_date": auction_dates,
            "sold_price": pa.array(sold_prices, type=pa.float64()),
            "days_on_market": pa.array(days_on_market, type=pa.int32()),
            "latitude": pa.array(latitudes, type=pa.float64()),
            "longitude": pa.array(longitudes, type=pa.float64()),
        },
        schema=SILVER_DOMAIN_SCHEMA,
    )

    path = write_silver(
        table, "domain_listings", "domain_listings.parquet", config=config
    )
    print(f"Promoted {table.num_rows} rows to {path}")
    return path
