import marimo

__generated_with = "0.23.9"
app = marimo.App(width="medium")


@app.cell
def _():
    import marimo as mo
    import polars as pl
    import pandas as pd
    import altair as alt
    import json
    import sys
    from pathlib import Path

    sys.path.insert(0, str(Path(__file__).parent.parent))
    from reports.helpers import (
        load_usage_stats,
        load_team_sheets,
        load_base_stats,
        parse_spreads_with_speed,
        parse_teammates,
        parse_items_abilities,
        tournament_usage,
    )
    return mo, pl, pd, alt, json, load_usage_stats, load_team_sheets, load_base_stats, parse_spreads_with_speed, parse_teammates, parse_items_abilities, tournament_usage


@app.cell
def _(load_usage_stats, load_team_sheets, load_base_stats, pl):
    df = load_usage_stats()
    teams_df = load_team_sheets()
    base_stats = load_base_stats()
    period = df["period"].unique().to_list()[0] if len(df) > 0 else "unknown"
    elo_1500 = df.filter(pl.col("elo_bracket") == 1500).sort("rank")
    return df, teams_df, base_stats, period, elo_1500


@app.cell
def _(mo, period):
    mo.md(f"""
    # VGC Meta Report — Champions Regulation M-A ({period})

    Monthly analysis of the competitive Pokemon VGC ladder and tournament scene.
    Data sourced from Smogon (`gen9championsvgc2026regma`) and Limitless TCG tournaments.
    """)
    return


@app.cell
def _(mo, elo_1500, period, alt):
    _top_20 = elo_1500.head(20).select("pokemon", "usage_pct", "raw_count").to_pandas()
    _total = _top_20["raw_count"].sum()

    _chart = (
        alt.Chart(_top_20)
        .mark_bar(cornerRadiusEnd=4, color="#6366f1")
        .encode(
            x=alt.X("usage_pct:Q", title="Usage %"),
            y=alt.Y("pokemon:N", sort=list(_top_20["pokemon"]), title=""),
            tooltip=["pokemon:N", alt.Tooltip("usage_pct:Q", format=".1f"), alt.Tooltip("raw_count:Q", format=",")],
        )
        .properties(width=550, height=450)
    )

    mo.md(f"""## Top 20 Pokemon — Ladder (ELO ≥1500)

Usage % = team inclusion rate. Total battles sampled: **{_total:,}**.

{mo.as_html(_chart)}
""")
    return


@app.cell
def _(mo, elo_1500, parse_teammates, alt, pd):
    _cores_df = parse_teammates(elo_1500)

    _chart = (
        alt.Chart(_cores_df)
        .mark_bar(cornerRadiusEnd=4, color="#10b981")
        .encode(
            x=alt.X("Co-usage %:Q", title="Co-usage %"),
            y=alt.Y("Pokemon 1:N", sort="-x", title=""),
            color=alt.Color("Pokemon 2:N", legend=alt.Legend(title="Partner")),
            tooltip=["Pokemon 1:N", "Pokemon 2:N", alt.Tooltip("Co-usage %:Q", format=".1f")],
        )
        .properties(width=550, height=350)
    )

    mo.md(f"""## Common Cores

Pokemon that appear together on teams. High co-usage = format-defining core.

{mo.as_html(_chart)}

{mo.as_html(mo.ui.table(_cores_df))}
""")
    return


@app.cell
def _(mo, elo_1500, base_stats, parse_spreads_with_speed, alt, pd):
    _spread_df = parse_spreads_with_speed(elo_1500, base_stats)

    if _spread_df.empty:
        mo.md("## Speed Tiers\n\n*No base stat data available — run with pokemon sync enabled.*")
    else:
        _total = len(_spread_df)
        _sum_pct = _spread_df["Spread %"].sum()
        _avg_speed = (_spread_df["Actual Speed"] * _spread_df["Spread %"]).sum() / _sum_pct
        _avg_speed_ev = (_spread_df["Speed EVs"] * _spread_df["Spread %"]).sum() / _sum_pct
        _avg_bulk = (_spread_df["Bulk EVs"] * _spread_df["Spread %"]).sum() / _sum_pct
        _avg_offense = (_spread_df["Offense EVs"] * _spread_df["Spread %"]).sum() / _sum_pct
        _max_spe = (_spread_df["Speed EVs"] >= 252).sum()
        _min_spe = (_spread_df["Speed EVs"] == 0).sum()
        _lean = "fast and aggressive" if _avg_speed_ev > _avg_bulk else "bulky and defensive"

        _speed_chart = (
            alt.Chart(_spread_df)
            .mark_circle(opacity=0.7)
            .encode(
                x=alt.X("Actual Speed:Q", title="Speed Stat (Lv50)"),
                y=alt.Y("Pokemon:N", sort=alt.EncodingSortField(field="Usage %", order="descending"), title=""),
                color=alt.Color("Nature Mod:N", scale=alt.Scale(domain=["+", "-", ""], range=["#ef4444", "#3b82f6", "#9ca3af"]), title="Speed Nature"),
                size=alt.Size("Spread %:Q", legend=None, scale=alt.Scale(range=[40, 200])),
                tooltip=["Pokemon", "Actual Speed", "Base Speed", "Speed EVs", "Nature", "Spread %"],
            )
            .properties(width=550, height=500, title="Speed Tier Map — Actual Stats at Lv50 (31 IVs)")
        )

        _tier_bins = pd.DataFrame([
            {"Tier": "200+", "Count": int((_spread_df["Actual Speed"] >= 200).sum())},
            {"Tier": "150-199", "Count": int(((_spread_df["Actual Speed"] >= 150) & (_spread_df["Actual Speed"] < 200)).sum())},
            {"Tier": "100-149", "Count": int(((_spread_df["Actual Speed"] >= 100) & (_spread_df["Actual Speed"] < 150)).sum())},
            {"Tier": "50-99", "Count": int(((_spread_df["Actual Speed"] >= 50) & (_spread_df["Actual Speed"] < 100)).sum())},
            {"Tier": "<50", "Count": int((_spread_df["Actual Speed"] < 50).sum())},
        ])
        _tier_chart = (
            alt.Chart(_tier_bins)
            .mark_bar(cornerRadiusEnd=4)
            .encode(
                x=alt.X("Count:Q", title="# spread variants"),
                y=alt.Y("Tier:N", sort=["200+", "150-199", "100-149", "50-99", "<50"], title=""),
                color=alt.Color("Tier:N", legend=None, scale=alt.Scale(range=["#dc2626", "#f97316", "#eab308", "#22c55e", "#3b82f6"])),
            )
            .properties(width=400, height=150)
        )

        _invest = pd.DataFrame([
            {"Category": "Speed", "Avg EVs": round(_avg_speed_ev, 1)},
            {"Category": "Bulk (HP+Def+SpD)", "Avg EVs": round(_avg_bulk, 1)},
            {"Category": "Offense (Atk+SpA)", "Avg EVs": round(_avg_offense, 1)},
        ])
        _invest_chart = (
            alt.Chart(_invest)
            .mark_bar(cornerRadiusEnd=4)
            .encode(
                x=alt.X("Avg EVs:Q", title="Weighted Avg EVs"),
                y=alt.Y("Category:N", sort="-x", title=""),
                color=alt.Color("Category:N", legend=None, scale=alt.Scale(range=["#6366f1", "#10b981", "#f59e0b"])),
            )
            .properties(width=400, height=120)
        )

        mo.md(f"""## Speed Tiers & Meta Trend

The meta leans **{_lean}**. Weighted avg speed stat: **{_avg_speed:.0f}**.

| Metric | Value |
|--------|-------|
| Avg Speed Stat | {_avg_speed:.0f} |
| Avg Speed EVs | {_avg_speed_ev:.0f} |
| Avg Bulk EVs | {_avg_bulk:.0f} |
| Avg Offense EVs | {_avg_offense:.0f} |
| Max Speed spreads | {_max_spe}/{_total} ({_max_spe/_total*100:.0f}%) |
| Min Speed spreads | {_min_spe}/{_total} ({_min_spe/_total*100:.0f}%) |

{mo.as_html(_speed_chart)}

### Speed Tier Distribution

{mo.as_html(_tier_chart)}

### EV Allocation

{mo.as_html(_invest_chart)}
""")
    return


@app.cell
def _(mo, df, pl, pd, alt):
    _elo_high = df.filter(pl.col("elo_bracket") == 1760).sort("rank")
    _elo_low = df.filter(pl.col("elo_bracket") == 1500).sort("rank")
    _low_top50 = set(_elo_low.head(50)["pokemon"].to_list())

    _anti = [
        {"Pokemon": r["pokemon"], "Usage % (1760)": round(r["usage_pct"], 2), "Rank": r["rank"]}
        for r in _elo_high.iter_rows(named=True)
        if r["pokemon"] not in _low_top50 and r["usage_pct"] > 1.0
    ][:10]

    _rising = []
    for _r in _elo_high.head(30).iter_rows(named=True):
        _match = _elo_low.filter(pl.col("pokemon") == _r["pokemon"])
        if len(_match) > 0 and _match["rank"][0] - _r["rank"] >= 5:
            _rising.append({"Pokemon": _r["pokemon"], "Rank 1500": _match["rank"][0], "Rank 1760": _r["rank"], "Jump": _match["rank"][0] - _r["rank"]})
    _rising = sorted(_rising, key=lambda x: x["Jump"], reverse=True)[:10]
    _rising_df = pd.DataFrame(_rising)

    _chart = (
        alt.Chart(_rising_df)
        .mark_bar(cornerRadiusEnd=4, color="#10b981")
        .encode(
            x=alt.X("Jump:Q", title="Positions gained"),
            y=alt.Y("Pokemon:N", sort=list(_rising_df["Pokemon"]), title=""),
            tooltip=["Pokemon:N", "Rank 1500:Q", "Rank 1760:Q", "Jump:Q"],
        )
        .properties(width=500, height=280)
    ) if len(_rising_df) > 0 else None

    mo.md(f"""## Anti-Meta & Rising Stars

Pokemon that see disproportionate play at high ELO — tech picks and skill-intensive strategies.

### High-ELO Exclusives (≥1760, absent from top 50 at 1500)

{mo.as_html(mo.ui.table(pd.DataFrame(_anti)))}

### Rising at High ELO

{mo.as_html(_chart) if _chart else "*No significant risers this period.*"}
""")
    return


@app.cell
def _(mo, elo_1500, teams_df, tournament_usage, pl, pd, alt):
    _ladder_top = elo_1500.head(30).select("pokemon", "usage_pct").to_pandas()
    _ladder_top = _ladder_top.rename(columns={"pokemon": "Pokemon", "usage_pct": "Ladder %"})

    _tourney = tournament_usage(teams_df)

    if _tourney.empty:
        mo.md("## Ladder vs Tournament\n\n*No tournament data available.*")
    else:
        _merged = _ladder_top.merge(_tourney[["Pokemon", "Tournament Usage %"]], on="Pokemon", how="outer").fillna(0)
        _merged["Diff"] = round(_merged["Tournament Usage %"] - _merged["Ladder %"], 1)
        _merged = _merged.sort_values("Diff", ascending=False)

        _tourney_fav = _merged[_merged["Diff"] > 3].head(10)
        _ladder_fav = _merged[_merged["Diff"] < -3].sort_values("Diff").head(10)

        _chart_data = pd.concat([
            _tourney_fav.assign(Category="Tournament Favoured"),
            _ladder_fav.assign(Category="Ladder Favoured"),
        ])

        _chart = (
            alt.Chart(_chart_data)
            .mark_bar(cornerRadiusEnd=4)
            .encode(
                x=alt.X("Diff:Q", title="Usage Difference (Tournament - Ladder)"),
                y=alt.Y("Pokemon:N", sort=alt.EncodingSortField(field="Diff", order="descending"), title=""),
                color=alt.Color("Category:N", scale=alt.Scale(domain=["Tournament Favoured", "Ladder Favoured"], range=["#8b5cf6", "#f59e0b"]), legend=alt.Legend(title="")),
                tooltip=["Pokemon:N", alt.Tooltip("Ladder %:Q", format=".1f"), alt.Tooltip("Tournament Usage %:Q", format=".1f"), alt.Tooltip("Diff:Q", format=".1f")],
            )
            .properties(width=550, height=350, title="Ladder vs Tournament Usage Gap")
        ) if len(_chart_data) > 0 else None

        mo.md(f"""## Ladder vs Tournament

Comparing ladder usage (Smogon 1500+ ELO) against tournament results (Limitless TCG).
Positive diff = more popular in tournaments; negative = ladder-only favourite.

{mo.as_html(_chart) if _chart else ""}

{mo.as_html(mo.ui.table(_merged.head(20).round(1)))}
""")
    return


@app.cell
def _(mo, elo_1500, parse_items_abilities, alt, pd):
    _items_df = parse_items_abilities(elo_1500)

    _chart = (
        alt.Chart(_items_df)
        .mark_bar(cornerRadiusEnd=4)
        .encode(
            x=alt.X("Item %:Q", title="Item Usage %"),
            y=alt.Y("Pokemon:N", sort=list(_items_df["Pokemon"]), title=""),
            color=alt.Color("Item:N", legend=alt.Legend(title="Item")),
            tooltip=["Pokemon:N", "Item:N", alt.Tooltip("Item %:Q", format=".1f"), "Ability:N", alt.Tooltip("Ability %:Q", format=".1f")],
        )
        .properties(width=550, height=350, title="Top Item by Pokemon")
    )

    mo.md(f"""## Items & Abilities

Most popular item and ability for the top 15 Pokemon. High % = solved slot; low % = flex pick.

{mo.as_html(_chart)}

{mo.as_html(mo.ui.table(_items_df))}
""")
    return


if __name__ == "__main__":
    app.run()
