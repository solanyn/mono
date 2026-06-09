from __future__ import annotations

import json
import os
import subprocess
from datetime import datetime, timezone
from pathlib import Path

import boto3


def _get_s3_client():
    endpoint = os.getenv("R2_ENDPOINT")
    access_key = os.getenv("CLOUDFLARE_R2_ACCESS_KEY_ID")
    secret_key = os.getenv("CLOUDFLARE_R2_SECRET_ACCESS_KEY")

    if not endpoint or not access_key:
        raise RuntimeError("R2 credentials not configured (R2_ENDPOINT, CLOUDFLARE_R2_ACCESS_KEY_ID)")

    return boto3.client(
        "s3",
        endpoint_url=endpoint,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
        region_name="auto",
    )


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
    s3 = _get_s3_client()
    s3.upload_file(
        local_path,
        bucket,
        key,
        ExtraArgs={"ContentType": "text/html", "CacheControl": "public, max-age=3600"},
    )
    return f"https://assets.goyangi.io/{key}"


def update_report_manifest(bucket: str, period: str, report_type: str = "meta-report"):
    s3 = _get_s3_client()
    manifest_key = "vgc/reports/index.json"

    try:
        obj = s3.get_object(Bucket=bucket, Key=manifest_key)
        manifest = json.loads(obj["Body"].read())
    except Exception:
        manifest = {"reports": []}

    entry = {
        "period": period,
        "type": report_type,
        "key": f"vgc/reports/{report_type}-{period}.html",
        "url": f"/vgc/report?period={period}",
        "published_at": datetime.now(timezone.utc).isoformat(),
    }

    existing = [r for r in manifest["reports"] if r["period"] != period or r["type"] != report_type]
    existing.append(entry)
    existing.sort(key=lambda r: r["period"], reverse=True)
    manifest["reports"] = existing

    s3.put_object(
        Bucket=bucket,
        Key=manifest_key,
        Body=json.dumps(manifest, indent=2),
        ContentType="application/json",
        CacheControl="public, max-age=300",
    )


def publish_meta_report(period: str, report_dir: str = "reports") -> str:
    report_path = Path(report_dir) / "meta_report.py"
    output_path = f"/tmp/vgc_meta_report_{period}.html"

    export_report(str(report_path), output_path)
    url = upload_to_r2(output_path, "assets", f"vgc/reports/meta-report-{period}.html")
    upload_to_r2(output_path, "assets", "vgc/reports/meta-report-latest.html")
    update_report_manifest("assets", period, "meta-report")
    return url


if __name__ == "__main__":
    import sys
    period = sys.argv[1] if len(sys.argv) > 1 else "latest"
    url = publish_meta_report(period)
    print(f"Published: {url}")
