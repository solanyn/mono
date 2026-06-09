from __future__ import annotations

import argparse
import os
import tempfile
from datetime import datetime, timezone
from pathlib import Path


def run_pipeline(period: str | None = None, output_dir: str | None = None, skip_publish: bool = False, narrative: bool = False):
    from sources.smogon.ingest import ingest_period, list_available_periods
    from sources.pokepaste.limitless import ingest_tournaments
    from sources.pokeapi.sync import sync_all
    from publish.export import export_report, upload_to_r2
    from schema.storage import write_parquet_to_lake

    if output_dir is None:
        output_dir = tempfile.mkdtemp(prefix="vgc-pipeline-")
    Path(output_dir).mkdir(parents=True, exist_ok=True)

    if period is None:
        periods = list_available_periods()
        period = periods[-1] if periods else datetime.now(timezone.utc).strftime("%Y-%m")

    print(f"[vgc] Pipeline start — period={period}, output={output_dir}")

    print(f"[vgc] Stage 1: Smogon usage stats ({period})")
    usage_path = ingest_period(period, output_dir)
    if usage_path:
        print(f"[vgc]   -> {usage_path}")
        write_parquet_to_lake(usage_path, "usage_stats")
        print("[vgc]   -> written to lake: vgc.usage_stats")
    else:
        print("[vgc]   -> No data available")

    print("[vgc] Stage 2: Limitless team sheets")
    teams_path = ingest_tournaments(output_dir, min_players=16)
    if teams_path:
        print(f"[vgc]   -> {teams_path}")
        write_parquet_to_lake(teams_path, "team_sheets")
        print("[vgc]   -> written to lake: vgc.team_sheets")
    else:
        print("[vgc]   -> No tournaments found")

    print("[vgc] Stage 3: PokeAPI sync (types, natures, pokemon)")
    pokemon_names = []
    if usage_path:
        import pyarrow.parquet as pq
        _usage_table = pq.read_table(usage_path)
        pokemon_names = list(set(_usage_table.column("pokemon").to_pylist()))
    game_paths = sync_all(output_dir, ["type", "nature", "pokemon"], pokemon_filter=pokemon_names)
    for entity_type, path in game_paths.items():
        print(f"[vgc]   -> {entity_type}: {path}")

    print("[vgc] Stage 4: Generate meta report")
    report_path = str(Path(__file__).parent / "reports" / "meta_report.py")
    html_path = f"{output_dir}/meta_report_{period}.html"
    try:
        export_report(report_path, html_path, data_dir=output_dir)
        print(f"[vgc]   -> {html_path}")
    except Exception as e:
        print(f"[vgc]   -> Export failed: {e}")
        html_path = None

    narrative_path = None
    if narrative and usage_path:
        print("[vgc] Stage 5: Generate LLM narrative")
        try:
            narrative_path = generate_weekly_narrative(usage_path, output_dir, period, game_paths.get("pokemon"))
            print(f"[vgc]   -> {narrative_path}")
        except Exception as e:
            print(f"[vgc]   -> Narrative failed: {e}")

    if not skip_publish:
        print("[vgc] Stage 6: Publish to R2")
        if html_path:
            try:
                url = upload_to_r2(html_path, "assets", f"vgc/reports/meta-report-{period}.html")
                print(f"[vgc]   -> report: {url}")
            except Exception as e:
                print(f"[vgc]   -> Report publish failed: {e}")
        if narrative_path:
            try:
                url = upload_to_r2(narrative_path, "assets", f"vgc/reports/weekly-{period}.html")
                print(f"[vgc]   -> narrative: {url}")
            except Exception as e:
                print(f"[vgc]   -> Narrative publish failed: {e}")

    print(f"[vgc] Pipeline complete — {period}")


def generate_weekly_narrative(usage_path: str, output_dir: str, period: str, pokemon_path: str | None) -> str:
    import polars as pl
    from reports.helpers import load_base_stats
    from analysis import build_stats_summary, generate_narrative

    usage_df = pl.read_parquet(usage_path)
    base_stats = {}
    if pokemon_path:
        os.environ["VGC_DATA_DIR"] = output_dir
        base_stats = load_base_stats()

    summary = build_stats_summary(usage_df, base_stats)
    text = generate_narrative(summary)

    html = f"""<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>VGC Weekly Meta Pulse — {period}</title>
<style>
body {{ font-family: system-ui, sans-serif; max-width: 720px; margin: 2rem auto; padding: 0 1rem; line-height: 1.6; }}
h1 {{ color: #6366f1; }}
</style></head>
<body>
<h1>VGC Weekly Meta Pulse — {period}</h1>
{_markdown_to_html(text)}
</body></html>"""

    path = f"{output_dir}/weekly_{period}.html"
    Path(path).write_text(html)
    return path


def _markdown_to_html(text: str) -> str:
    paragraphs = text.strip().split("\n\n")
    return "\n".join(f"<p>{p.strip()}</p>" for p in paragraphs if p.strip())


def main():
    parser = argparse.ArgumentParser(description="VGC meta analysis pipeline")
    parser.add_argument("--period", help="Stats period (YYYY-MM). Defaults to latest.")
    parser.add_argument("--output", help="Output directory for parquet/html files")
    parser.add_argument("--skip-publish", action="store_true", help="Skip R2 upload")
    parser.add_argument("--narrative", action="store_true", help="Generate LLM weekly narrative")
    args = parser.parse_args()

    run_pipeline(period=args.period, output_dir=args.output, skip_publish=args.skip_publish, narrative=args.narrative)


if __name__ == "__main__":
    main()
