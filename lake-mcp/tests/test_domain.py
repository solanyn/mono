import json
from unittest.mock import patch, MagicMock

from lake_mcp.ingest.domain import (
    _get_token,
    fetch_auction_results,
    fetch_listings,
    build_bronze_table,
)
from lake_mcp.promote.domain_to_silver import SILVER_DOMAIN_SCHEMA
from datalake.schemas import BRONZE_SCHEMA


def _mock_response(text=None, json_data=None, status_code=200):
    resp = MagicMock()
    resp.status_code = status_code
    resp.text = text or ""
    resp.json.return_value = json_data
    resp.raise_for_status.return_value = None
    return resp


class TestDomainAuth:
    @patch("lake.ingest.domain.httpx.post")
    def test_get_token(self, mock_post):
        mock_post.return_value = _mock_response(
            json_data={"access_token": "test-token", "expires_in": 3600}
        )
        token = _get_token("client-id", "client-secret")
        assert token == "test-token"
        mock_post.assert_called_once()
        call_data = mock_post.call_args[1]["data"]
        assert call_data["grant_type"] == "client_credentials"
        assert call_data["client_id"] == "client-id"


class TestDomainAuctionIngest:
    def test_parse_auction_results(self, domain_auctions_json):
        results = (
            fetch_auction_results.__wrapped__(domain_auctions_json)
            if hasattr(fetch_auction_results, "__wrapped__")
            else None
        )
        if results is None:
            with patch("lake.ingest.domain._api_get") as mock_get:
                mock_get.return_value = domain_auctions_json
                results = fetch_auction_results("fake-token")
        assert len(results) == 3
        assert results[0]["listing_id"] == "12345"
        assert results[0]["suburb"] == "Chatswood"
        assert results[0]["sold_price"] == 2500000

    def test_auction_fields(self, domain_auctions_json):
        with patch("lake.ingest.domain._api_get") as mock_get:
            mock_get.return_value = domain_auctions_json
            results = fetch_auction_results("fake-token")
        first = results[0]
        assert first["state"] == "NSW"
        assert first["postcode"] == "2067"
        assert first["property_type"] == "House"
        assert first["bedrooms"] == 4
        assert first["bathrooms"] == 2
        assert first["latitude"] == -33.7969
        assert first["longitude"] == 151.1832

    def test_bronze_table_schema(self, domain_auctions_json):
        with patch("lake.ingest.domain._api_get") as mock_get:
            mock_get.return_value = domain_auctions_json
            results = fetch_auction_results("fake-token")
        table = build_bronze_table(results)
        assert table.schema == BRONZE_SCHEMA
        assert table.num_rows == 3

    def test_bronze_payloads_are_valid_json(self, domain_auctions_json):
        with patch("lake.ingest.domain._api_get") as mock_get:
            mock_get.return_value = domain_auctions_json
            results = fetch_auction_results("fake-token")
        table = build_bronze_table(results)
        for payload in table.column("_raw_payload").to_pylist():
            parsed = json.loads(payload)
            assert "listing_id" in parsed
            assert "suburb" in parsed


class TestDomainListingsIngest:
    def test_parse_listings(self, domain_listings_json):
        with patch("lake.ingest.domain.httpx.post") as mock_post:
            mock_post.return_value = _mock_response(json_data=domain_listings_json)
            results = fetch_listings("fake-token", suburbs=["Sydney"])
        assert len(results) == 2
        assert results[0]["listing_id"] == "99001"
        assert results[0]["suburb"] == "Sydney"
        assert results[0]["property_type"] == "Apartment"

    def test_listing_fields(self, domain_listings_json):
        with patch("lake.ingest.domain.httpx.post") as mock_post:
            mock_post.return_value = _mock_response(json_data=domain_listings_json)
            results = fetch_listings("fake-token", suburbs=["Sydney"])
        second = results[1]
        assert second["bedrooms"] == 3
        assert second["bathrooms"] == 2
        assert second["price_guide"] == "$2,200,000"
        assert second["latitude"] == -33.87

    def test_bronze_table_schema(self, domain_listings_json):
        with patch("lake.ingest.domain.httpx.post") as mock_post:
            mock_post.return_value = _mock_response(json_data=domain_listings_json)
            results = fetch_listings("fake-token", suburbs=["Sydney"])
        table = build_bronze_table(results)
        assert table.schema == BRONZE_SCHEMA
        assert table.num_rows == 2


class TestDomainPromote:
    def test_silver_schema_fields(self):
        expected = {
            "listing_id",
            "suburb",
            "state",
            "postcode",
            "property_type",
            "bedrooms",
            "bathrooms",
            "price_guide",
            "auction_date",
            "sold_price",
            "days_on_market",
            "latitude",
            "longitude",
        }
        assert set(SILVER_DOMAIN_SCHEMA.names) == expected

    def test_bronze_to_silver(self, domain_auctions_json):
        with patch("lake.ingest.domain._api_get") as mock_get:
            mock_get.return_value = domain_auctions_json
            results = fetch_auction_results("fake-token")
        bronze = build_bronze_table(results)
        payloads = bronze.column("_raw_payload").to_pylist()

        rows = []
        for row_json in payloads:
            row = json.loads(row_json)
            if row.get("listing_id"):
                rows.append(row)
        assert len(rows) == 3
        assert rows[0]["sold_price"] == 2500000
