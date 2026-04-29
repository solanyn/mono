"""Golden fixture parity test for align().

Locks in the word-to-speaker alignment output against a hand-audited fixture.
If you change align() semantics intentionally, regenerate the fixture with:

    uv run python tests/regenerate_align_golden.py

and eyeball the diff before committing.
"""
import json
from pathlib import Path

from scrib_audio.align import align
from scrib_audio.diarize import DiarSegment
from scrib_audio.transcribe import Word

FIXTURE = Path(__file__).parent / "fixtures" / "align_golden.json"


def _load_fixture():
    data = json.loads(FIXTURE.read_text())
    diar = [DiarSegment(s["speaker"], s["start"], s["end"]) for s in data["diar_segments"]]
    words = [Word(w["word"], w["start"], w["end"]) for w in data["words"]]
    return data, diar, words


def test_align_golden_matches_fixture():
    data, diar, words = _load_fixture()
    result = align(diar, words, data["duration"])

    expected = data["expected"]
    assert len(result) == len(expected), (
        f"segment count mismatch: got {len(result)}, want {len(expected)}\n"
        f"got: {[(s.speaker, s.text) for s in result]}"
    )
    for i, (got, want) in enumerate(zip(result, expected)):
        assert got.speaker == want["speaker"], f"seg {i}: speaker {got.speaker!r} != {want['speaker']!r}"
        assert got.text == want["text"], f"seg {i}: text {got.text!r} != {want['text']!r}"
        assert abs(got.start - want["start"]) < 1e-6, f"seg {i}: start {got.start} != {want['start']}"
        assert abs(got.end - want["end"]) < 1e-6, f"seg {i}: end {got.end} != {want['end']}"
        assert got.uncertain == want["uncertain"], (
            f"seg {i}: uncertain {got.uncertain} != {want['uncertain']}"
        )
