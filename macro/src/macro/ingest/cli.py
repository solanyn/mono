import sys

from macro.ingest import rba_csv, abs_sdmx, aemo, rss, reddit, domain
from macro.promote.bronze_to_silver import promote_rba
from macro.promote.abs_to_silver import promote_abs
from macro.promote.aemo_to_silver import promote_aemo
from macro.promote.rss_to_silver import promote_rss
from macro.promote.reddit_to_silver import promote_reddit
from macro.promote.domain_to_silver import promote_domain

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
        print(f"Usage: python -m macro.ingest.cli <{'|'.join(COMMANDS)}>")
        sys.exit(1)
    cmd = sys.argv[1]
    COMMANDS[cmd]()


if __name__ == "__main__":
    main()
