import json
from datetime import datetime, timezone

import pyarrow as pa

from datalake import DatalakeConfig, read_parquet, write_silver

SILVER_RBA_SCHEMA = pa.schema(
    [
        pa.field("date", pa.date32()),
        pa.field("series_id", pa.string()),
        pa.field("series_name", pa.string()),
        pa.field("value", pa.float64()),
    ]
)

SERIES_NAMES = {
    "FIRMMCRTD": "Cash Rate Target",
    "FIRMMCCRT": "Change in the Cash Rate Target",
    "FIRMMCRID": "Interbank Overnight Cash Rate",
    "FIRMMCRIH": "Highest Interbank Overnight Cash Rate",
    "FIRMMCRIL": "Lowest Interbank Overnight Cash Rate",
    "FIRMMCRIV": "Volume of Cash Market Transactions",
    "FIRMMCRIN": "Number of Cash Market Transactions",
    "FIRMMCTRI": "Cash Rate Total Return Index",
    "FIRMMBAB30D": "Bank Accepted Bills 30 Day",
    "FIRMMBAB90D": "Bank Accepted Bills 90 Day",
    "FIRMMBAB180D": "Bank Accepted Bills 180 Day",
    "FIRMMOIS1D": "OIS 1 Month",
    "FIRMMOIS3D": "OIS 3 Month",
    "FIRMMOIS6D": "OIS 6 Month",
    "FIRMMTN1D": "Treasury Note 1 Month",
    "FIRMMTN3D": "Treasury Note 3 Month",
    "FIRMMTN6D": "Treasury Note 6 Month",
}


def promote_rba(
    bronze_path: str | None = None,
    config: DatalakeConfig | None = None,
) -> str:
    config = config or DatalakeConfig()

    if bronze_path is None:
        now = datetime.now(timezone.utc)
        bronze_path = (
            f"bronze/rba/{now.year:04d}/{now.month:02d}/{now.day:02d}/f1-data.parquet"
        )

    bronze = read_parquet(bronze_path, config)
    payloads = bronze.column("_raw_payload").to_pylist()

    if not payloads:
        raise ValueError(f"No rows in {bronze_path}")

    first_row = json.loads(payloads[0])
    series_ids = [k for k in first_row.keys() if k != "Series ID"]

    dates = []
    s_ids = []
    s_names = []
    values = []

    for row_json in payloads:
        row = json.loads(row_json)
        date_str = row.get("Series ID", "")
        if not date_str:
            continue

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

    table = pa.table(
        {
            "date": pa.array(dates, type=pa.date32()),
            "series_id": s_ids,
            "series_name": s_names,
            "value": values,
        },
        schema=SILVER_RBA_SCHEMA,
    )

    path = write_silver(table, "rba_rates", "rba_rates.parquet", config=config)
    print(f"Promoted {table.num_rows} rows to {path}")
    return path
