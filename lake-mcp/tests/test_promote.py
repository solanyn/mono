import json
from datetime import datetime, timezone

import pyarrow as pa

from lake_mcp.ingest.rba_csv import parse_rba_csv, build_bronze_table as build_rba_bronze
from lake_mcp.ingest.abs_sdmx import (
    parse_observations as parse_abs,
    build_bronze_table as build_abs_bronze,
)
from lake_mcp.ingest.aemo import build_bronze_table as build_aemo_bronze
from lake_mcp.ingest.reddit import build_bronze_table as build_reddit_bronze
from lake_mcp.promote.bronze_to_silver import SILVER_RBA_SCHEMA, SERIES_NAMES, promote_rba
from lake_mcp.promote.abs_to_silver import SILVER_ABS_SCHEMA
from lake_mcp.promote.aemo_to_silver import SILVER_AEMO_SCHEMA
from lake_mcp.promote.reddit_to_silver import SILVER_REDDIT_SCHEMA
from datalake.schemas import BRONZE_SCHEMA


class TestRbaPromote:
    def test_silver_schema_fields(self):
        assert set(SILVER_RBA_SCHEMA.names) == {
            "date",
            "series_id",
            "series_name",
            "value",
        }

    def test_bronze_to_silver(self, rba_csv_text):
        rows = parse_rba_csv(rba_csv_text)
        bronze = build_rba_bronze(rows)
        payloads = bronze.column("_raw_payload").to_pylist()

        first_row = json.loads(payloads[0])
        series_ids = [k for k in first_row.keys() if k != "Series ID"]

        dates, s_ids, s_names, values = [], [], [], []
        for row_json in payloads:
            row = json.loads(row_json)
            date_str = row.get("Series ID", "")
            try:
                dt = datetime.strptime(date_str, "%d-%b-%Y").date()
            except ValueError:
                continue
            for sid in series_ids:
                val_str = row.get(sid, "")
                if not val_str or val_str.strip() == "":
                    continue
                try:
                    val = float(val_str)
                except ValueError:
                    continue
                dates.append(dt)
                s_ids.append(sid)
                s_names.append(SERIES_NAMES.get(sid, sid))
                values.append(val)

        silver = pa.table(
            {
                "date": pa.array(dates, type=pa.date32()),
                "series_id": s_ids,
                "series_name": s_names,
                "value": values,
            },
            schema=SILVER_RBA_SCHEMA,
        )

        assert silver.num_rows > 0
        assert silver.schema == SILVER_RBA_SCHEMA
        assert all(isinstance(v, float) for v in silver.column("value").to_pylist())
        assert "FIRMMCRTD" in set(silver.column("series_id").to_pylist())

    def test_silver_dates_are_valid(self, rba_csv_text):
        rows = parse_rba_csv(rba_csv_text)
        bronze = build_rba_bronze(rows)
        payloads = bronze.column("_raw_payload").to_pylist()

        for row_json in payloads:
            row = json.loads(row_json)
            date_str = row.get("Series ID", "")
            try:
                dt = datetime.strptime(date_str, "%d-%b-%Y").date()
                assert dt.year >= 2000
                assert dt.year <= 2030
            except ValueError:
                continue

    def test_cash_rate_values_in_range(self, rba_csv_text):
        rows = parse_rba_csv(rba_csv_text)
        bronze = build_rba_bronze(rows)
        payloads = bronze.column("_raw_payload").to_pylist()

        for row_json in payloads:
            row = json.loads(row_json)
            val_str = row.get("FIRMMCRTD", "")
            if not val_str:
                continue
            val = float(val_str)
            assert 0.0 <= val <= 20.0


class TestAbsPromote:
    def test_silver_schema_fields(self):
        expected = {
            "indicator_id",
            "indicator_name",
            "period",
            "frequency",
            "value",
            "unit",
            "revision_number",
            "release_date",
        }
        assert set(SILVER_ABS_SCHEMA.names) == expected

    def test_bronze_to_silver(self, abs_cpi_json):
        rows = parse_abs(abs_cpi_json)
        bronze = build_abs_bronze(rows)
        payloads = bronze.column("_raw_payload").to_pylist()

        indicator_ids, indicator_names, periods, frequencies = [], [], [], []
        values, units, revisions, releases = [], [], [], []

        for row_json in payloads:
            r = json.loads(row_json)
            v = r.get("value")
            if v is None:
                continue
            indicator_ids.append(r.get("indicator_id", ""))
            indicator_names.append(r.get("indicator_name", ""))
            periods.append(r.get("time_period", ""))
            frequencies.append(r.get("frequency", "M"))
            values.append(float(v))
            units.append(r.get("unit", ""))
            rev = r.get("revision_number", 0)
            revisions.append(int(rev) if rev is not None else 0)
            releases.append(r.get("release_date", ""))

        silver = pa.table(
            {
                "indicator_id": indicator_ids,
                "indicator_name": indicator_names,
                "period": periods,
                "frequency": frequencies,
                "value": values,
                "unit": units,
                "revision_number": pa.array(revisions, type=pa.int32()),
                "release_date": releases,
            },
            schema=SILVER_ABS_SCHEMA,
        )

        assert silver.num_rows == 5
        assert silver.schema == SILVER_ABS_SCHEMA
        assert all(isinstance(v, float) for v in silver.column("value").to_pylist())


class TestAemoPromote:
    def test_silver_schema_fields(self):
        expected = {
            "timestamp",
            "region",
            "price_aud_mwh",
            "demand_mw",
            "generation_mw",
        }
        assert set(SILVER_AEMO_SCHEMA.names) == expected

    def test_bronze_to_silver(self, aemo_nem_json):
        rows = aemo_nem_json["ELEC_NEM_SUMMARY"]
        bronze = build_aemo_bronze(rows)
        payloads = bronze.column("_raw_payload").to_pylist()

        timestamps, regions, prices, demands, generations = [], [], [], [], []
        for row_json in payloads:
            r = json.loads(row_json)
            ts = datetime.fromisoformat(r["SETTLEMENTDATE"]).replace(
                tzinfo=timezone.utc
            )
            sched = r.get("SCHEDULEDGENERATION", 0) or 0
            semi = r.get("SEMISCHEDULEDGENERATION", 0) or 0
            timestamps.append(ts)
            regions.append(r["REGIONID"])
            prices.append(float(r["PRICE"]))
            demands.append(float(r["TOTALDEMAND"]))
            generations.append(float(sched) + float(semi))

        silver = pa.table(
            {
                "timestamp": pa.array(timestamps, type=pa.timestamp("us", tz="UTC")),
                "region": regions,
                "price_aud_mwh": pa.array(prices, type=pa.float64()),
                "demand_mw": pa.array(demands, type=pa.float64()),
                "generation_mw": pa.array(generations, type=pa.float64()),
            },
            schema=SILVER_AEMO_SCHEMA,
        )

        assert silver.num_rows == 5
        assert set(silver.column("region").to_pylist()) == {
            "NSW1",
            "QLD1",
            "SA1",
            "TAS1",
            "VIC1",
        }


class TestRedditPromote:
    def test_silver_schema_fields(self):
        expected = {
            "captured_at",
            "post_id",
            "title",
            "score",
            "num_comments",
            "upvote_ratio",
            "flair",
            "key_topics",
        }
        assert set(SILVER_REDDIT_SCHEMA.names) == expected

    def test_bronze_to_silver(self, reddit_hot_json):
        posts = reddit_hot_json["data"]["children"]
        raw_posts = []
        for child in posts:
            p = child["data"]
            raw_posts.append(
                {
                    "post_id": p["id"],
                    "title": p["title"],
                    "score": p["score"],
                    "num_comments": p["num_comments"],
                    "upvote_ratio": p["upvote_ratio"],
                    "flair": p.get("link_flair_text") or "",
                    "selftext": p.get("selftext", ""),
                    "url": p.get("url", ""),
                    "created_utc": p.get("created_utc", 0),
                }
            )

        bronze = build_reddit_bronze(raw_posts)
        payloads = bronze.column("_raw_payload").to_pylist()
        ingested_ats = bronze.column("_ingested_at").to_pylist()

        captured_ats, post_ids, titles, scores = [], [], [], []
        num_comments, upvote_ratios, flairs, key_topics = [], [], [], []

        for row_json, ingested_at in zip(payloads, ingested_ats):
            r = json.loads(row_json)
            captured_ats.append(ingested_at)
            post_ids.append(r["post_id"])
            titles.append(r["title"])
            scores.append(int(r["score"]))
            num_comments.append(int(r["num_comments"]))
            upvote_ratios.append(float(r["upvote_ratio"]))
            flairs.append(r.get("flair", ""))
            key_topics.append([])

        silver = pa.table(
            {
                "captured_at": pa.array(
                    captured_ats, type=pa.timestamp("us", tz="UTC")
                ),
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

        assert silver.num_rows == 3
        assert silver.schema == SILVER_REDDIT_SCHEMA
        assert all(isinstance(s, int) for s in silver.column("score").to_pylist())
