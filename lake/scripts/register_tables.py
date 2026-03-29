"""Register Iceberg tables in Lakekeeper for existing S3 parquet data."""

import os
import sys

import boto3
from pyiceberg.catalog import load_catalog
from pyiceberg.exceptions import (
    NamespaceAlreadyExistsError,
    NoSuchNamespaceError,
    TableAlreadyExistsError,
)
from pyiceberg.schema import Schema
from pyiceberg.types import (
    DoubleType,
    LongType,
    NestedField,
    StringType,
    TimestampType,
)

BRONZE_SCHEMA = Schema(
    NestedField(1, "_source", StringType(), required=False),
    NestedField(2, "_ingested_at", TimestampType(), required=False),
    NestedField(3, "_raw_payload", StringType(), required=False),
    NestedField(4, "_batch_id", StringType(), required=False),
)

SILVER_SCHEMAS = {
    "rba_rates": Schema(
        NestedField(1, "_source", StringType(), required=False),
        NestedField(2, "_ingested_at", TimestampType(), required=False),
        NestedField(3, "_raw_payload", StringType(), required=False),
        NestedField(4, "_batch_id", StringType(), required=False),
    ),
    "abs_indicators": Schema(
        NestedField(1, "_source", StringType(), required=False),
        NestedField(2, "_ingested_at", TimestampType(), required=False),
        NestedField(3, "_raw_payload", StringType(), required=False),
        NestedField(4, "_batch_id", StringType(), required=False),
    ),
    "aemo_prices": Schema(
        NestedField(1, "_source", StringType(), required=False),
        NestedField(2, "_ingested_at", TimestampType(), required=False),
        NestedField(3, "_raw_payload", StringType(), required=False),
        NestedField(4, "_batch_id", StringType(), required=False),
    ),
    "news_articles": Schema(
        NestedField(1, "_source", StringType(), required=False),
        NestedField(2, "_ingested_at", TimestampType(), required=False),
        NestedField(3, "_raw_payload", StringType(), required=False),
        NestedField(4, "_batch_id", StringType(), required=False),
    ),
    "reddit_sentiment": Schema(
        NestedField(1, "_source", StringType(), required=False),
        NestedField(2, "_ingested_at", TimestampType(), required=False),
        NestedField(3, "_raw_payload", StringType(), required=False),
        NestedField(4, "_batch_id", StringType(), required=False),
    ),
    "domain_listings": Schema(
        NestedField(1, "_source", StringType(), required=False),
        NestedField(2, "_ingested_at", TimestampType(), required=False),
        NestedField(3, "_raw_payload", StringType(), required=False),
        NestedField(4, "_batch_id", StringType(), required=False),
    ),
    "nsw_vg_sales": Schema(
        NestedField(1, "_source", StringType(), required=False),
        NestedField(2, "_ingested_at", TimestampType(), required=False),
        NestedField(3, "_raw_payload", StringType(), required=False),
        NestedField(4, "_batch_id", StringType(), required=False),
    ),
}

BRONZE_SOURCES = {
    "rba": "rba",
    "abs": "abs",
    "aemo": "aemo",
    "rss": "rss",
    "reddit": "reddit",
    "domain": "domain",
    "nsw_vg": "nsw_vg",
}

SILVER_SOURCES = {
    "rba_rates": "rba_rates",
    "abs_indicators": "abs_indicators",
    "aemo_prices": "aemo_prices",
    "news_articles": "news_articles",
    "reddit_sentiment": "reddit_sentiment",
    "domain_listings": "domain_listings",
    "nsw_vg_sales": "nsw_vg_sales",
}


def create_namespace(catalog, name):
    try:
        catalog.create_namespace(name)
        print(f"Created namespace: {name}")
    except NamespaceAlreadyExistsError:
        print(f"Namespace exists: {name}")


def create_table(catalog, namespace, table_name, schema, location):
    identifier = f"{namespace}.{table_name}"
    try:
        catalog.create_table(identifier, schema=schema, location=location)
        print(f"Created table: {identifier}")
    except TableAlreadyExistsError:
        print(f"Table exists: {identifier}")


def main():
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
    s3_region = os.environ.get("AWS_REGION", os.environ.get("S3_REGION", "us-east-1"))

    catalog = load_catalog(
        "lakekeeper",
        **{
            "type": "rest",
            "uri": catalog_uri,
            "s3.endpoint": s3_endpoint,
            "s3.access-key-id": s3_access_key,
            "s3.secret-access-key": s3_secret_key,
            "s3.region": s3_region,
            "s3.path-style-access": "true",
        },
    )

    for ns in ("bronze", "silver", "gold"):
        create_namespace(catalog, ns)

    for table_name, prefix in BRONZE_SOURCES.items():
        location = f"s3://bronze/{prefix}/"
        create_table(catalog, "bronze", table_name, BRONZE_SCHEMA, location)

    for table_name, prefix in SILVER_SOURCES.items():
        schema = SILVER_SCHEMAS.get(table_name, BRONZE_SCHEMA)
        location = f"s3://silver/{prefix}/"
        create_table(catalog, "silver", table_name, schema, location)

    print("Table registration complete")


if __name__ == "__main__":
    main()
