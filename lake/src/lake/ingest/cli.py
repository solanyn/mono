import sys

from lake.ingest import rba_csv, abs_sdmx, aemo, rss, reddit, domain
from lake.promote.bronze_to_silver import promote_rba
from lake.promote.abs_to_silver import promote_abs
from lake.promote.aemo_to_silver import promote_aemo
from lake.promote.rss_to_silver import promote_rss
from lake.promote.reddit_to_silver import promote_reddit
from lake.promote.domain_to_silver import promote_domain

INGESTS = [
    ("rba", rba_csv.ingest),
    ("abs", abs_sdmx.ingest),
    ("aemo", aemo.ingest),
    ("rss", rss.ingest),
    ("reddit", reddit.ingest),
    ("domain", domain.ingest),
]

PROMOTIONS = [
    ("promote-rba", promote_rba),
    ("promote-abs", promote_abs),
    ("promote-aemo", promote_aemo),
    ("promote-rss", promote_rss),
    ("promote-reddit", promote_reddit),
    ("promote-domain", promote_domain),
]

COMMANDS = {name: fn for name, fn in INGESTS + PROMOTIONS}


def run_all():
    for name, fn in INGESTS:
        print(f"--- ingest: {name} ---")
        try:
            fn()
        except Exception as e:
            print(f"Warning: {name} failed: {e}")
    for name, fn in PROMOTIONS:
        print(f"--- {name} ---")
        try:
            fn()
        except Exception as e:
            print(f"Warning: {name} failed: {e}")


COMMANDS["all"] = run_all


def main():
    if len(sys.argv) < 2 or sys.argv[1] not in COMMANDS:
        print(f"Usage: python -m lake.ingest.cli <{'|'.join(COMMANDS)}>")
        sys.exit(1)
    cmd = sys.argv[1]
    COMMANDS[cmd]()


if __name__ == "__main__":
    main()
