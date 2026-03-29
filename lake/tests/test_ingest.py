import json
from unittest.mock import patch, MagicMock

import pyarrow as pa

from lake.ingest.rba_csv import fetch_rba_csv, parse_rba_csv, build_bronze_table
from lake.ingest.abs_sdmx import (
    fetch_abs_cpi,
    parse_observations as parse_abs,
    build_bronze_table as build_abs_bronze,
)
from lake.ingest.aemo import (
    fetch_nem_summary,
    build_bronze_table as build_aemo_bronze,
)
from lake.ingest.reddit import fetch_reddit, build_bronze_table as build_reddit_bronze
from lake.ingest.rss import fetch_feeds, build_bronze_table as build_rss_bronze
from datalake.schemas import BRONZE_SCHEMA


def _mock_response(text=None, json_data=None, status_code=200):
    resp = MagicMock()
    resp.status_code = status_code
    resp.text = text or ""
    resp.json.return_value = json_data
    resp.raise_for_status.return_value = None
    resp.is_success = True
    return resp


class TestRbaIngest:
    def test_parse_csv(self, rba_csv_text):
        rows = parse_rba_csv(rba_csv_text)
        assert len(rows) == 6
        assert "Series ID" in rows[0]
        assert "FIRMMCRTD" in rows[0]

    def test_parse_extracts_values(self, rba_csv_text):
        rows = parse_rba_csv(rba_csv_text)
        first = rows[0]
        assert first["FIRMMCRTD"] == "4.75"
        assert first["Series ID"] == "04-Jan-2011"

    def test_bronze_table_schema(self, rba_csv_text):
        rows = parse_rba_csv(rba_csv_text)
        table = build_bronze_table(rows)
        assert table.schema == BRONZE_SCHEMA
        assert table.num_rows == 6

    def test_bronze_payloads_are_valid_json(self, rba_csv_text):
        rows = parse_rba_csv(rba_csv_text)
        table = build_bronze_table(rows)
        for payload in table.column("_raw_payload").to_pylist():
            parsed = json.loads(payload)
            assert isinstance(parsed, dict)
            assert "Series ID" in parsed

    @patch("lake.ingest.rba_csv.httpx.get")
    def test_fetch_uses_correct_url(self, mock_get, rba_csv_text):
        mock_get.return_value = _mock_response(text=rba_csv_text)
        result = fetch_rba_csv()
        assert result == rba_csv_text
        mock_get.assert_called_once()
        assert "f1-data.csv" in mock_get.call_args[0][0]


class TestAbsIngest:
    def test_parse_observations(self, abs_cpi_json):
        rows = parse_abs(abs_cpi_json)
        assert len(rows) == 5
        assert rows[0]["indicator_id"] == "20004"
        assert rows[0]["indicator_name"] == "Furnishings"
        assert rows[0]["value"] == 100.0
        assert rows[0]["time_period"] == "2024-01"

    def test_two_series_parsed(self, abs_cpi_json):
        rows = parse_abs(abs_cpi_json)
        indicator_ids = {r["indicator_id"] for r in rows}
        assert "20004" in indicator_ids
        assert "131179" in indicator_ids

    def test_bronze_table_schema(self, abs_cpi_json):
        rows = parse_abs(abs_cpi_json)
        table = build_abs_bronze(rows)
        assert table.schema == BRONZE_SCHEMA
        assert table.num_rows == 5

    @patch("lake.ingest.abs_sdmx.httpx.get")
    def test_fetch_returns_json(self, mock_get, abs_cpi_json):
        mock_get.return_value = _mock_response(json_data=abs_cpi_json)
        result = fetch_abs_cpi()
        assert result == abs_cpi_json


class TestAemoIngest:
    def test_parse_nem_summary(self, aemo_nem_json):
        with patch("lake.ingest.aemo.httpx.get") as mock_get:
            mock_get.return_value = _mock_response(json_data=aemo_nem_json)
            rows = fetch_nem_summary()
        assert len(rows) == 5
        regions = {r["REGIONID"] for r in rows}
        assert regions == {"NSW1", "QLD1", "SA1", "TAS1", "VIC1"}

    def test_prices_are_numeric(self, aemo_nem_json):
        rows = aemo_nem_json["ELEC_NEM_SUMMARY"]
        for row in rows:
            assert isinstance(row["PRICE"], (int, float))
            assert isinstance(row["TOTALDEMAND"], (int, float))

    def test_bronze_table_schema(self, aemo_nem_json):
        rows = aemo_nem_json["ELEC_NEM_SUMMARY"]
        table = build_aemo_bronze(rows)
        assert table.schema == BRONZE_SCHEMA
        assert table.num_rows == 5


class TestRedditIngest:
    def test_parse_posts(self, reddit_hot_json):
        with patch("lake.ingest.reddit.httpx.get") as mock_get:
            mock_get.return_value = _mock_response(json_data=reddit_hot_json)
            posts = fetch_reddit({"hot": "http://fake"})
        assert len(posts) == 3
        assert posts[0]["post_id"] == "abc123"
        assert posts[0]["score"] == 245

    def test_deduplication(self, reddit_hot_json, reddit_new_json):
        call_count = 0

        def side_effect(*args, **kwargs):
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                return _mock_response(json_data=reddit_hot_json)
            return _mock_response(json_data=reddit_new_json)

        with patch("lake.ingest.reddit.httpx.get", side_effect=side_effect):
            posts = fetch_reddit({"hot": "http://fake/hot", "new": "http://fake/new"})
        post_ids = [p["post_id"] for p in posts]
        assert len(post_ids) == len(set(post_ids))
        assert len(posts) == 4

    def test_bronze_table_schema(self, reddit_hot_json):
        with patch("lake.ingest.reddit.httpx.get") as mock_get:
            mock_get.return_value = _mock_response(json_data=reddit_hot_json)
            posts = fetch_reddit({"hot": "http://fake"})
        table = build_reddit_bronze(posts)
        assert table.schema == BRONZE_SCHEMA
        assert table.num_rows == 3

    def test_null_flair_becomes_empty(self, reddit_hot_json):
        with patch("lake.ingest.reddit.httpx.get") as mock_get:
            mock_get.return_value = _mock_response(json_data=reddit_hot_json)
            posts = fetch_reddit({"hot": "http://fake"})
        energy_post = [p for p in posts if p["post_id"] == "ghi789"][0]
        assert energy_post["flair"] == ""


class TestRssIngest:
    def test_parse_feeds(self, rss_guardian_xml):
        with patch("lake.ingest.rss.httpx.get") as mock_get:
            resp = _mock_response(text=rss_guardian_xml)
            mock_get.return_value = resp
            articles = fetch_feeds({"guardian": "http://fake"})
        assert len(articles) == 2
        assert articles[0]["source"] == "guardian"
        assert "RBA" in articles[0]["title"]

    def test_bronze_table_schema(self, rss_guardian_xml):
        with patch("lake.ingest.rss.httpx.get") as mock_get:
            mock_get.return_value = _mock_response(text=rss_guardian_xml)
            articles = fetch_feeds({"guardian": "http://fake"})
        table = build_rss_bronze(articles)
        assert table.schema == BRONZE_SCHEMA
        assert table.num_rows == 2
