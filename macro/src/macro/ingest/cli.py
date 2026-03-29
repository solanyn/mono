import sys

from macro.ingest import rba_csv, abs_sdmx, aemo, rss
from macro.promote.bronze_to_silver import promote_rba
from macro.promote.abs_to_silver import promote_abs
from macro.promote.aemo_to_silver import promote_aemo
from macro.promote.rss_to_silver import promote_rss

COMMANDS = {
    "rba": rba_csv.ingest,
    "abs": abs_sdmx.ingest,
    "aemo": aemo.ingest,
    "rss": rss.ingest,
    "promote-rba": promote_rba,
    "promote-abs": promote_abs,
    "promote-aemo": promote_aemo,
    "promote-rss": promote_rss,
}


def main():
    if len(sys.argv) < 2 or sys.argv[1] not in COMMANDS:
        print(f"Usage: python -m macro.ingest.cli <{'|'.join(COMMANDS)}>")
        sys.exit(1)
    cmd = sys.argv[1]
    COMMANDS[cmd]()


if __name__ == "__main__":
    main()
