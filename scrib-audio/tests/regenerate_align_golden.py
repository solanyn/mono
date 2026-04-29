"""Regenerate align golden fixture from the in-file inputs.

Run when you intentionally change align() behaviour. Review the diff before committing.

    uv run python tests/regenerate_align_golden.py
"""
import json
from pathlib import Path

from scrib_audio.align import align
from scrib_audio.diarize import DiarSegment
from scrib_audio.transcribe import Word

FIXTURE = Path(__file__).parent / "fixtures" / "align_golden.json"


def main():
    data = json.loads(FIXTURE.read_text())
    diar = [DiarSegment(s["speaker"], s["start"], s["end"]) for s in data["diar_segments"]]
    words = [Word(w["word"], w["start"], w["end"]) for w in data["words"]]

    result = align(diar, words, data["duration"])
    data["expected"] = [
        {
            "speaker": s.speaker,
            "start": round(s.start, 6),
            "end": round(s.end, 6),
            "text": s.text,
            "uncertain": s.uncertain,
        }
        for s in result
    ]

    FIXTURE.write_text(json.dumps(data, indent=2) + "\n")
    print(f"wrote {len(result)} segments to {FIXTURE}")


if __name__ == "__main__":
    main()
