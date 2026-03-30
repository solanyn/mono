"""Register Iceberg tables in Lakekeeper with full catalog metadata."""

import os

from pyiceberg.catalog import load_catalog
from pyiceberg.exceptions import NamespaceAlreadyExistsError, TableAlreadyExistsError
from pyiceberg.schema import Schema
from pyiceberg.types import NestedField, StringType, TimestampType

BRONZE_SCHEMA = Schema(
    NestedField(
        1,
        "_source",
        StringType(),
        required=False,
        doc="Source identifier (e.g. rba.f1, aemo.nem_summary)",
    ),
    NestedField(
        2,
        "_ingested_at",
        TimestampType(),
        required=False,
        doc="UTC timestamp when row was ingested",
    ),
    NestedField(
        3,
        "_raw_payload",
        StringType(),
        required=False,
        doc="JSON-encoded raw payload from source API",
    ),
    NestedField(
        4,
        "_batch_id",
        StringType(),
        required=False,
        doc="UUID identifying the ingest batch",
    ),
)

NAMESPACE_PROPS = {
    "bronze": {
        "description": "Raw ingested data from external APIs. Schema: _source, _ingested_at, _raw_payload, _batch_id. One table per data source."
    },
    "silver": {
        "description": "Cleaned and transformed data promoted from bronze. Structured columns extracted from raw JSON payloads."
    },
    "gold": {
        "description": "Aggregated and enriched data for analytics and agent consumption. Cross-source joins and time-series rollups."
    },
}

BRONZE_TABLES = {
    "rba": {
        "description": "RBA interest rate statistics (Table F1). Cash rate target, interbank rates, OIS, bank bills, treasury notes.",
        "source_url": "https://rba.gov.au/statistics/tables/csv/f1-data.csv",
        "source_name": "Reserve Bank of Australia",
        "update_frequency": "daily",
        "temporal_coverage": "2011-present",
        "auth": "none",
        "gotchas": "CSV has metadata header rows before the actual data. Series IDs change occasionally.",
    },
    "abs": {
        "description": "ABS CPI monthly index numbers across 34 sub-categories via SDMX JSON.",
        "source_url": "https://data.api.abs.gov.au/data/CPI",
        "source_name": "Australian Bureau of Statistics",
        "update_frequency": "monthly",
        "temporal_coverage": "2020-present",
        "auth": "none",
        "gotchas": "SDMX JSON format with nested series/observations structure. Dimension IDs need resolving.",
    },
    "aemo": {
        "description": "AEMO NEM 5-minute spot prices, demand, generation and interconnector flows by region.",
        "source_url": "https://visualisations.aemo.com.au/aemo/apps/api/report/ELEC_NEM_SUMMARY",
        "source_name": "Australian Energy Market Operator",
        "update_frequency": "5min",
        "temporal_coverage": "snapshot (latest interval)",
        "auth": "none",
        "gotchas": "Returns current interval only. Historical data requires separate NEMWEB downloads.",
    },
    "rss": {
        "description": "RSS feed articles from Guardian AU Business, RBA media releases and speeches.",
        "source_url": "https://www.theguardian.com/au/business/rss",
        "source_name": "Multiple (Guardian, RBA)",
        "update_frequency": "15min",
        "temporal_coverage": "rolling (latest ~50 articles per feed)",
        "auth": "none",
        "gotchas": "Feed items may duplicate across polls. Deduplicate by URL.",
    },
    "reddit": {
        "description": "Reddit r/AusFinance hot and new posts with scores, comments and flair.",
        "source_url": "https://www.reddit.com/r/AusFinance/.json",
        "source_name": "Reddit",
        "update_frequency": "30min",
        "temporal_coverage": "rolling (top 25 hot + 25 new)",
        "auth": "none",
        "gotchas": "Rate limited. User-Agent required. Posts deduplicated by post_id across hot/new.",
    },
    "domain": {
        "description": "Domain.com.au Sydney auction results and residential listings search.",
        "source_url": "https://api.domain.com.au/v1/salesResults/Sydney",
        "source_name": "Domain Group",
        "update_frequency": "daily",
        "temporal_coverage": "latest auction round + active listings",
        "auth": "oauth2_client_credentials",
        "gotchas": "Free tier: ~500 req/day. OAuth2 token expires after 1h. Rate limit with 500ms delays.",
    },
    "nsw_vg": {
        "description": "NSW Valuer General bulk property sales data. ZIP/CSV with 23 columns per sale.",
        "source_url": "https://valuation.property.nsw.gov.au/embed/propertySalesInformation",
        "source_name": "NSW Valuer General",
        "update_frequency": "weekly",
        "temporal_coverage": "current + previous year",
        "auth": "none",
        "gotchas": "ZIP contains multiple CSVs. Header row starts with 'A' record type. Large files (100k+ rows/year).",
    },
    "asx": {
        "description": "ASX company announcements for top tickers.",
        "source_url": "https://asx.api.markitdigital.com/asx-research/1.0/companies",
        "source_name": "ASX",
        "update_frequency": "hourly",
        "auth": "none",
    },
    "abs_ba": {
        "description": "ABS Building Approvals by GCCSA via SDMX JSON.",
        "source_url": "https://api.data.abs.gov.au/data/ABS,BA_GCCSA",
        "source_name": "Australian Bureau of Statistics",
        "update_frequency": "monthly",
        "auth": "none",
    },
    "abs_migration": {
        "description": "ABS Net Overseas Migration by visa type via SDMX JSON.",
        "source_url": "https://api.data.abs.gov.au/data/ABS,ABS_NOM_VISA_FY",
        "source_name": "Australian Bureau of Statistics",
        "update_frequency": "annual",
        "auth": "none",
    },
    "rba_credit": {
        "description": "RBA Credit Aggregates (Table D1). Housing, personal and business credit.",
        "source_url": "https://rba.gov.au/statistics/tables/csv/d1-data.csv",
        "source_name": "Reserve Bank of Australia",
        "update_frequency": "monthly",
        "auth": "none",
    },
    "weather": {
        "description": "Open-Meteo daily weather for 8 Australian capital cities.",
        "source_url": "https://api.open-meteo.com",
        "source_name": "Open-Meteo",
        "update_frequency": "daily",
        "auth": "none",
    },
    "github_trending": {
        "description": "GitHub trending repositories sorted by stars.",
        "source_url": "https://api.github.com/search/repositories",
        "source_name": "GitHub",
        "update_frequency": "daily",
        "auth": "token",
    },
    "pypi_stats": {
        "description": "PyPI recent download stats for tracked Python packages.",
        "source_url": "https://pypistats.org/api",
        "source_name": "PyPI",
        "update_frequency": "daily",
        "auth": "none",
    },
    "npm_stats": {
        "description": "npm weekly download stats for tracked JavaScript packages.",
        "source_url": "https://api.npmjs.org/downloads",
        "source_name": "npm",
        "update_frequency": "daily",
        "auth": "none",
    },
    "hn_stories": {
        "description": "Hacker News recent stories via Algolia search API.",
        "source_url": "https://hn.algolia.com/api/v1/search_by_date",
        "source_name": "Hacker News",
        "update_frequency": "hourly",
        "auth": "none",
    },
    "nsw_fuel": {
        "description": "NSW fuel prices from FuelCheck API (10k+ stations).",
        "source_url": "https://api.onegov.nsw.gov.au/FuelPriceCheck/v2/fuel/prices",
        "source_name": "NSW Government",
        "update_frequency": "hourly",
        "auth": "oauth2_client_credentials",
    },
    "nsw_property_licences": {
        "description": "NSW property agent and corporation licences.",
        "source_url": "https://api.onegov.nsw.gov.au/propertyregister/v1/browse",
        "source_name": "NSW Government",
        "update_frequency": "daily",
        "auth": "oauth2_client_credentials",
    },
    "nsw_trades_licences": {
        "description": "NSW trades and contractor licences.",
        "source_url": "https://api.onegov.nsw.gov.au/tradesregister/v1/browse",
        "source_name": "NSW Government",
        "update_frequency": "daily",
        "auth": "oauth2_client_credentials",
    },
}

SILVER_TABLES = {
    "rba_rates": {
        "description": "Cleaned RBA interest rates with parsed dates and numeric values.",
        "source_name": "Reserve Bank of Australia",
        "update_frequency": "daily",
    },
    "abs_indicators": {
        "description": "Structured ABS CPI indicators with resolved dimension labels.",
        "source_name": "Australian Bureau of Statistics",
        "update_frequency": "monthly",
    },
    "aemo_prices": {
        "description": "NEM spot prices and demand by region with parsed timestamps.",
        "source_name": "Australian Energy Market Operator",
        "update_frequency": "5min",
    },
    "news_articles": {
        "description": "Parsed news articles with title, URL, published date and source attribution.",
        "source_name": "Multiple (Guardian, RBA)",
        "update_frequency": "15min",
    },
    "reddit_sentiment": {
        "description": "Reddit posts with extracted sentiment signals, scores and engagement metrics.",
        "source_name": "Reddit r/AusFinance",
        "update_frequency": "30min",
    },
    "domain_listings": {
        "description": "Structured property listings with suburb, price, bedrooms, coordinates.",
        "source_name": "Domain Group",
        "update_frequency": "daily",
    },
    "nsw_vg_sales": {
        "description": "Cleaned NSW property sales with parsed addresses, dates and normalised prices.",
        "source_name": "NSW Valuer General",
        "update_frequency": "weekly",
    },
}


def get_catalog(warehouse):
    catalog_uri = os.environ.get(
        "ICEBERG_CATALOG_URI",
        "http://lakekeeper.storage.svc.cluster.local:8181/catalog",
    )
    s3_endpoint = os.environ.get(
        "AWS_ENDPOINT_URL", "http://garage.storage.svc.cluster.local:3900"
    )
    s3_access_key = os.environ.get(
        "AWS_ACCESS_KEY_ID", os.environ.get("S3_ACCESS_KEY", "")
    )
    s3_secret_key = os.environ.get(
        "AWS_SECRET_ACCESS_KEY", os.environ.get("S3_SECRET_KEY", "")
    )
    s3_region = os.environ.get("AWS_REGION", "us-east-1")
    return load_catalog(
        f"lakekeeper-{warehouse}",
        **{
            "type": "rest",
            "uri": catalog_uri,
            "warehouse": warehouse,
            "s3.endpoint": s3_endpoint,
            "s3.access-key-id": s3_access_key,
            "s3.secret-access-key": s3_secret_key,
            "s3.region": s3_region,
            "s3.path-style-access": "true",
        },
    )


def create_ns(catalog, name, props=None):
    try:
        catalog.create_namespace(name, properties=props or {})
        print(f"Created namespace: {name}")
    except NamespaceAlreadyExistsError:
        if props:
            try:
                catalog.update_namespace_properties(name, updates=props)
            except Exception:
                pass


def create_tbl(catalog, ns, name, location, props=None):
    try:
        tbl = catalog.create_table(
            f"{ns}.{name}",
            schema=BRONZE_SCHEMA,
            location=location,
            properties=props or {},
        )
        print(f"Created table: {ns}.{name}")
        return tbl
    except TableAlreadyExistsError:
        if props:
            try:
                tbl = catalog.load_table(f"{ns}.{name}")
                with tbl.transaction() as tx:
                    tx.set_properties(**props)
            except Exception:
                pass
        return None


def main():
    for layer in ("bronze", "silver", "gold"):
        cat = get_catalog(layer)
        ns_props = NAMESPACE_PROPS.get(layer, {})
        create_ns(cat, "default", ns_props)

    bronze_cat = get_catalog("bronze")
    for name, meta in BRONZE_TABLES.items():
        create_tbl(
            bronze_cat, "default", name, f"s3://bronze/iceberg/{name}/", props=meta
        )

    silver_cat = get_catalog("silver")
    for name, meta in SILVER_TABLES.items():
        create_tbl(
            silver_cat, "default", name, f"s3://silver/iceberg/{name}/", props=meta
        )

    print("Table registration complete")


if __name__ == "__main__":
    main()
