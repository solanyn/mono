from __future__ import annotations

import json
import subprocess
import hashlib
from datetime import datetime, timezone, date
from pathlib import Path

import pyarrow as pa
import pyarrow.parquet as pq

from schema.tables import KNOWLEDGE_SCHEMA

VGC_CHANNELS = [
    "https://www.youtube.com/@CybertronVGC",
    "https://www.youtube.com/@WolfeyVGC",
    "https://www.youtube.com/@JamesBaschVGC",
    "https://www.youtube.com/@AaronZhengVGC",
]


def download_transcript(url: str, output_dir: str) -> dict | None:
    meta = subprocess.run(
        ["yt-dlp", "--skip-download", "--print", "%(id)s\t%(title)s\t%(upload_date)s\t%(channel)s\t%(duration)s", url],
        capture_output=True,
        text=True,
        timeout=60,
    )
    if meta.returncode != 0:
        return None

    line = meta.stdout.strip().split("\n")[0]
    parts = line.split("\t")
    if len(parts) < 5:
        return None

    video_id, title, upload_date, channel, duration = parts[:5]

    subprocess.run(
        [
            "yt-dlp",
            "--skip-download",
            "--write-subs",
            "--write-auto-subs",
            "--sub-langs", "en.*",
            "--sub-format", "json3",
            "--output", f"{output_dir}/%(id)s.%(ext)s",
            url,
        ],
        capture_output=True,
        text=True,
        timeout=60,
    )

    sub_file = Path(output_dir) / f"{video_id}.en.json3"
    if not sub_file.exists():
        candidates = list(Path(output_dir).glob(f"{video_id}.en*.json3"))
        sub_file = candidates[0] if candidates else sub_file

    transcript = ""
    if sub_file.exists():
        data = json.loads(sub_file.read_text())
        segments = data.get("events", [])
        transcript = " ".join(
            seg.get("segs", [{}])[0].get("utf8", "")
            for seg in segments
            if seg.get("segs")
        )

    return {
        "video_id": video_id,
        "title": title,
        "upload_date": upload_date,
        "channel": channel,
        "duration": int(duration) if duration.isdigit() else 0,
        "transcript": transcript,
        "url": f"https://www.youtube.com/watch?v={video_id}",
    }


def extract_facts_from_transcript(video: dict) -> list[dict]:
    now = datetime.now(timezone.utc)
    facts = []

    facts.append({
        "fact_id": hashlib.sha256(f"transcript:{video['video_id']}".encode()).hexdigest()[:16],
        "fact_type": "team_report",
        "game": "champions",
        "regulation": "M-A",
        "content": json.dumps({
            "title": video["title"],
            "transcript_preview": video["transcript"][:2000],
            "full_length": len(video["transcript"]),
        }),
        "confidence": "medium",
        "source_type": "youtube",
        "source_url": video["url"],
        "source_timestamp": None,
        "source_channel": video["channel"],
        "valid_until": None,
        "extracted_at": now,
    })

    return facts


def ingest_channel_latest(channel_url: str, output_dir: str, max_videos: int = 5) -> list[dict]:
    result = subprocess.run(
        [
            "yt-dlp",
            "--flat-playlist",
            "--playlist-end", str(max_videos),
            "--print", "%(url)s",
            f"{channel_url}/videos",
        ],
        capture_output=True,
        text=True,
        timeout=30,
    )
    if result.returncode != 0:
        return []

    urls = [u.strip() for u in result.stdout.strip().split("\n") if u.strip()]
    all_facts = []

    for url in urls:
        video = download_transcript(url, output_dir)
        if video and video["transcript"]:
            facts = extract_facts_from_transcript(video)
            all_facts.extend(facts)

    return all_facts


def ingest_all_channels(output_dir: str, max_per_channel: int = 3) -> str | None:
    all_facts = []
    for channel in VGC_CHANNELS:
        facts = ingest_channel_latest(channel, output_dir, max_videos=max_per_channel)
        all_facts.extend(facts)

    if not all_facts:
        return None

    table = pa.Table.from_pylist(all_facts, schema=KNOWLEDGE_SCHEMA)
    path = f"{output_dir}/knowledge_youtube.parquet"
    pq.write_table(table, path, compression="zstd")
    return path


if __name__ == "__main__":
    import sys

    output_dir = sys.argv[1] if len(sys.argv) > 1 else "/tmp/vgc-yt"
    Path(output_dir).mkdir(parents=True, exist_ok=True)

    video_url = sys.argv[2] if len(sys.argv) > 2 else None
    if video_url:
        video = download_transcript(video_url, output_dir)
        if video:
            print(f"Title: {video['title']}")
            print(f"Channel: {video['channel']}")
            print(f"Transcript length: {len(video['transcript'])} chars")
            print(f"Preview: {video['transcript'][:200]}")
        else:
            print("Failed to download transcript")
    else:
        path = ingest_all_channels(output_dir, max_per_channel=2)
        if path:
            table = pq.read_table(path)
            print(f"Wrote {table.num_rows} knowledge facts to {path}")
        else:
            print("No transcripts extracted")
