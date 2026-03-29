import json
from datetime import date, datetime, timedelta, timezone

import pyarrow as pa
import pyarrow.compute as pc
from fastmcp import FastMCP

from datalake import DatalakeConfig, read_parquet
from macro.state import (
    load_state,
    update_cycle_position as _update_cycle,
    track_narrative as _track_narrative,
    update_narrative as _update_narrative,
)

mcp = FastMCP("macro-finance")

CASH_RATE_SERIES = "FIRMMCRTD"


def _find_latest(
    prefix: str, filename: str, days: int = 7, config: DatalakeConfig | None = None
) -> pa.Table | None:
    config = config or DatalakeConfig()
    now = datetime.now(timezone.utc)
    for days_back in range(days):
        dt = now - timedelta(days=days_back)
        path = f"silver/{prefix}/{dt.year:04d}/{dt.month:02d}/{dt.day:02d}/{filename}"
        try:
            return read_parquet(path, config)
        except Exception:
            continue
    return None


def _find_latest_silver_rba(config: DatalakeConfig | None = None) -> pa.Table | None:
    return _find_latest("rba_rates", "rba_rates.parquet", config=config)


def _find_latest_silver_abs(config: DatalakeConfig | None = None) -> pa.Table | None:
    return _find_latest("abs_indicators", "abs_indicators.parquet", config=config)


def _find_latest_silver_aemo(config: DatalakeConfig | None = None) -> pa.Table | None:
    return _find_latest("aemo_prices", "aemo_prices.parquet", config=config)


def _find_latest_silver_news(config: DatalakeConfig | None = None) -> pa.Table | None:
    return _find_latest("news_articles", "news_articles.parquet", config=config)


@mcp.tool()
def get_rate_history(months: int = 12) -> str:
    """Query silver RBA rates for cash rate target series, returning the last N months of data."""
    table = _find_latest_silver_rba()
    if table is None:
        return json.dumps({"error": "No silver RBA data found"})

    mask = pc.equal(table.column("series_id"), CASH_RATE_SERIES)
    filtered = table.filter(mask)

    cutoff = date.today() - timedelta(days=months * 30)
    date_mask = pc.greater_equal(
        filtered.column("date"), pa.scalar(cutoff, type=pa.date32())
    )
    filtered = filtered.filter(date_mask)
    filtered = filtered.sort_by("date")

    rows = []
    for i in range(filtered.num_rows):
        rows.append(
            {
                "date": str(filtered.column("date")[i].as_py()),
                "value": filtered.column("value")[i].as_py(),
            }
        )
    return json.dumps(
        {"series": "Cash Rate Target", "series_id": CASH_RATE_SERIES, "data": rows}
    )


@mcp.tool()
def get_macro_pulse(date_str: str = "latest") -> str:
    """Return a combined snapshot from all available sources: RBA rates, ABS CPI, AEMO energy prices."""
    result: dict = {}

    rba = _find_latest_silver_rba()
    if rba is not None:
        if date_str == "latest":
            max_date = pc.max(rba.column("date")).as_py()
        else:
            max_date = datetime.strptime(date_str, "%Y-%m-%d").date()
        mask = pc.equal(rba.column("date"), pa.scalar(max_date, type=pa.date32()))
        day_data = rba.filter(mask)
        rba_series = {}
        for i in range(day_data.num_rows):
            rba_series[day_data.column("series_name")[i].as_py()] = day_data.column(
                "value"
            )[i].as_py()
        result["rba"] = {"date": str(max_date), "series": rba_series}

    abs_table = _find_latest_silver_abs()
    if abs_table is not None:
        sorted_t = abs_table.sort_by("period")
        if sorted_t.num_rows > 0:
            latest_period = sorted_t.column("period")[sorted_t.num_rows - 1].as_py()
            mask = pc.equal(sorted_t.column("period"), latest_period)
            latest = sorted_t.filter(mask)
            cpi_data = []
            for i in range(latest.num_rows):
                cpi_data.append(
                    {
                        "indicator": latest.column("indicator_name")[i].as_py(),
                        "value": latest.column("value")[i].as_py(),
                        "unit": latest.column("unit")[i].as_py(),
                    }
                )
            result["abs_cpi"] = {"period": latest_period, "indicators": cpi_data}

    aemo = _find_latest_silver_aemo()
    if aemo is not None:
        sorted_a = aemo.sort_by("timestamp")
        if sorted_a.num_rows > 0:
            latest_ts = sorted_a.column("timestamp")[sorted_a.num_rows - 1].as_py()
            mask = pc.equal(
                sorted_a.column("timestamp"),
                pa.scalar(latest_ts, type=pa.timestamp("us", tz="UTC")),
            )
            latest = sorted_a.filter(mask)
            energy = []
            for i in range(latest.num_rows):
                energy.append(
                    {
                        "region": latest.column("region")[i].as_py(),
                        "price_aud_mwh": latest.column("price_aud_mwh")[i].as_py(),
                        "demand_mw": latest.column("demand_mw")[i].as_py(),
                        "generation_mw": latest.column("generation_mw")[i].as_py(),
                    }
                )
            result["aemo"] = {"timestamp": str(latest_ts), "regions": energy}

    if not result:
        return json.dumps({"error": "No silver data found from any source"})
    return json.dumps(result)


@mcp.tool()
def get_cpi_series(frequency: str = "monthly", periods: int = 24) -> str:
    """Query silver ABS CPI data for the last N periods."""
    table = _find_latest_silver_abs()
    if table is None:
        return json.dumps(
            {
                "error": "Silver CPI data not yet available. Run promote-abs first.",
                "stub": True,
            }
        )

    sorted_t = table.sort_by("period")

    unique_periods = pc.unique(sorted_t.column("period")).to_pylist()
    unique_periods.sort()
    if len(unique_periods) > periods:
        cutoff_period = unique_periods[-periods]
        mask = pc.greater_equal(sorted_t.column("period"), cutoff_period)
        sorted_t = sorted_t.filter(mask)

    rows = []
    for i in range(sorted_t.num_rows):
        rows.append(
            {
                "indicator_id": sorted_t.column("indicator_id")[i].as_py(),
                "indicator_name": sorted_t.column("indicator_name")[i].as_py(),
                "period": sorted_t.column("period")[i].as_py(),
                "value": sorted_t.column("value")[i].as_py(),
                "unit": sorted_t.column("unit")[i].as_py(),
            }
        )
    return json.dumps(
        {"series": "ABS CPI Monthly", "frequency": frequency, "data": rows}
    )


@mcp.tool()
def get_energy(region: str = "NSW1", days: int = 7) -> str:
    """Query silver AEMO NEM prices for a region over the last N days."""
    table = _find_latest_silver_aemo()
    if table is None:
        return json.dumps(
            {
                "error": "Silver AEMO data not yet available. Run promote-aemo first.",
            }
        )

    mask = pc.equal(table.column("region"), region)
    filtered = table.filter(mask)

    cutoff = datetime.now(timezone.utc) - timedelta(days=days)
    ts_mask = pc.greater_equal(
        filtered.column("timestamp"),
        pa.scalar(cutoff, type=pa.timestamp("us", tz="UTC")),
    )
    filtered = filtered.filter(ts_mask)
    filtered = filtered.sort_by("timestamp")

    rows = []
    for i in range(filtered.num_rows):
        rows.append(
            {
                "timestamp": str(filtered.column("timestamp")[i].as_py()),
                "price_aud_mwh": filtered.column("price_aud_mwh")[i].as_py(),
                "demand_mw": filtered.column("demand_mw")[i].as_py(),
                "generation_mw": filtered.column("generation_mw")[i].as_py(),
            }
        )
    return json.dumps({"region": region, "days": days, "data": rows})


@mcp.tool()
def get_news(hours: int = 24, source: str = "") -> str:
    """Get recent news articles from silver layer, optionally filtered by source.

    source: filter by source id (e.g. guardian_au_business, rba_media_releases, rba_speeches). Empty for all.
    hours: how far back to look
    """
    table = _find_latest_silver_news()
    if table is None:
        return json.dumps(
            {
                "error": "No silver news data found. Run rss ingest and promote-rss first."
            }
        )

    cutoff = datetime.now(timezone.utc) - timedelta(hours=hours)
    ts_mask = pc.greater_equal(
        table.column("published_at"),
        pa.scalar(cutoff, type=pa.timestamp("us", tz="UTC")),
    )
    filtered = table.filter(ts_mask)

    if source:
        src_mask = pc.equal(filtered.column("source"), source)
        filtered = filtered.filter(src_mask)

    filtered = filtered.sort_by([("published_at", "descending")])

    rows = []
    for i in range(filtered.num_rows):
        rows.append(
            {
                "published_at": str(filtered.column("published_at")[i].as_py()),
                "source": filtered.column("source")[i].as_py(),
                "title": filtered.column("title")[i].as_py(),
                "url": filtered.column("url")[i].as_py(),
                "summary": filtered.column("summary")[i].as_py(),
            }
        )
    return json.dumps({"count": len(rows), "articles": rows})


@mcp.tool()
def get_narrative_signals() -> str:
    """Return active tracked narratives with their data checks, plus recent news headlines for cross-referencing.

    Use this to identify divergences between media narratives and actual data.
    """
    from dataclasses import asdict

    state = load_state()
    active = [
        asdict(n)
        for n in state.tracked_narratives
        if n.status == "pending_verification"
    ]

    news_table = _find_latest_silver_news()
    recent_headlines = []
    if news_table is not None:
        cutoff = datetime.now(timezone.utc) - timedelta(hours=48)
        ts_mask = pc.greater_equal(
            news_table.column("published_at"),
            pa.scalar(cutoff, type=pa.timestamp("us", tz="UTC")),
        )
        filtered = news_table.filter(ts_mask).sort_by([("published_at", "descending")])
        for i in range(min(filtered.num_rows, 20)):
            recent_headlines.append(
                {
                    "title": filtered.column("title")[i].as_py(),
                    "source": filtered.column("source")[i].as_py(),
                    "published_at": str(filtered.column("published_at")[i].as_py()),
                }
            )

    return json.dumps(
        {
            "active_narratives": active,
            "recent_headlines": recent_headlines,
            "cycle_position": asdict(state.cycle_position),
        }
    )


@mcp.tool()
def get_agent_state() -> str:
    """Return the agent's current persistent state including cycle position, tracked narratives, and key levels."""
    from dataclasses import asdict

    state = load_state()
    return json.dumps(asdict(state))


@mcp.tool()
def set_cycle_position(phase: str, confidence: float, rationale: str) -> str:
    """Update the agent's assessment of where Australia is in the rate cycle.

    phase: one of early_easing, mid_easing, late_easing, neutral, early_tightening, mid_tightening, late_tightening, peak_hold
    confidence: 0.0 to 1.0
    rationale: brief explanation citing data
    """
    state = _update_cycle(phase, confidence, rationale)
    return json.dumps(
        {
            "updated": True,
            "phase": state.cycle_position.phase,
            "confidence": state.cycle_position.confidence,
        }
    )


@mcp.tool()
def add_tracked_narrative(
    claim: str, source: str, data_check: str, agent_prior: str = ""
) -> str:
    """Track a macro narrative or claim that needs verification against data.

    claim: the narrative to track (e.g. "housing market is cooling rapidly")
    source: where the claim came from
    data_check: what data series to check against
    agent_prior: agent's initial assessment before checking data
    """
    narrative = _track_narrative(
        claim=claim,
        source=source,
        data_check=data_check,
        agent_prior=agent_prior,
    )
    return json.dumps(
        {"id": narrative.id, "claim": narrative.claim, "status": narrative.status}
    )


@mcp.tool()
def resolve_narrative(narrative_id: str, status: str, evidence: str = "") -> str:
    """Update a tracked narrative's status after checking against data.

    status: one of pending_verification, confirmed, refuted, partially_confirmed
    evidence: data-backed reasoning for the status change
    """
    narrative = _update_narrative(narrative_id, status, evidence)
    if narrative is None:
        return json.dumps({"error": f"Narrative {narrative_id} not found"})
    return json.dumps(
        {"id": narrative.id, "status": narrative.status, "claim": narrative.claim}
    )


if __name__ == "__main__":
    mcp.run(transport="stdio")
