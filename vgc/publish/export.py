from __future__ import annotations

import os
import subprocess
from pathlib import Path

import boto3


def export_report(report_path: str, output_path: str, data_dir: str | None = None) -> str:
    env = os.environ.copy()
    if data_dir:
        env["VGC_DATA_DIR"] = data_dir
    result = subprocess.run(
        ["marimo", "export", "html", report_path, "-o", output_path, "--no-include-code"],
        capture_output=True,
        text=True,
        timeout=120,
        env=env,
    )
    if result.returncode != 0:
        raise RuntimeError(f"marimo export failed: {result.stderr}")
    return output_path


def upload_to_r2(local_path: str, bucket: str, key: str) -> str:
    endpoint = os.getenv("R2_ENDPOINT")
    access_key = os.getenv("CLOUDFLARE_R2_ACCESS_KEY_ID")
    secret_key = os.getenv("CLOUDFLARE_R2_SECRET_ACCESS_KEY")

    if not endpoint or not access_key:
        raise RuntimeError("R2 credentials not configured (R2_ENDPOINT, CLOUDFLARE_R2_ACCESS_KEY_ID)")

    s3 = boto3.client(
        "s3",
        endpoint_url=endpoint,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
        region_name="auto",
    )

    s3.upload_file(
        local_path,
        bucket,
        key,
        ExtraArgs={"ContentType": "text/html", "CacheControl": "public, max-age=3600"},
    )

    return f"https://assets.goyangi.io/{key}"


def publish_meta_report(period: str, report_dir: str = "reports") -> str:
    report_path = Path(report_dir) / "meta_report.py"
    output_path = f"/tmp/vgc_meta_report_{period}.html"

    export_report(str(report_path), output_path)
    url = upload_to_r2(output_path, "assets", f"vgc/reports/meta-report-{period}.html")
    return url


if __name__ == "__main__":
    import sys
    period = sys.argv[1] if len(sys.argv) > 1 else "latest"
    url = publish_meta_report(period)
    print(f"Published: {url}")
