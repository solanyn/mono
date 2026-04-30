"""Unit tests for _stitch_chunks.

These tests exercise the pure-Python stitching logic without loading
Senko, so they run on any platform and in CI.
"""

import numpy as np

from scrib_audio.diarize import DiarResult, DiarSegment, _stitch_chunks


def _emb(*vals):
    arr = np.asarray(vals, dtype=np.float32)
    n = np.linalg.norm(arr)
    return (arr / n).tolist() if n > 0 else arr.tolist()


def test_stitch_empty():
    r = _stitch_chunks([], [], 0.0)
    assert r.segments == []
    assert r.num_speakers == 0


def test_stitch_single_chunk_passthrough():
    chunk = DiarResult(
        segments=[
            DiarSegment("SPEAKER_0", 0.0, 2.0),
            DiarSegment("SPEAKER_1", 2.0, 4.0),
        ],
        num_speakers=2,
        duration_seconds=4.0,
        speaker_embeddings={"SPEAKER_0": _emb(1, 0), "SPEAKER_1": _emb(0, 1)},
    )
    r = _stitch_chunks([chunk], [0.0], 4.0)
    assert r.num_speakers == 2
    assert r.segments == chunk.segments


def test_stitch_same_speaker_across_chunks_merges():
    """Two chunks each have SPEAKER_0, with near-identical embeddings.

    They should collapse into a single global SPEAKER_0 and the
    segments from chunk 1 should be offset into the global timeline.
    """
    e = _emb(1, 0, 0)
    c0 = DiarResult(
        segments=[DiarSegment("SPEAKER_0", 0.0, 5.0)],
        num_speakers=1,
        duration_seconds=10.0,
        speaker_embeddings={"SPEAKER_0": e},
    )
    c1 = DiarResult(
        segments=[DiarSegment("SPEAKER_0", 2.0, 6.0)],
        num_speakers=1,
        duration_seconds=10.0,
        speaker_embeddings={"SPEAKER_0": e},
    )
    # Chunk 1 starts at 8.0s in the global timeline.
    r = _stitch_chunks([c0, c1], [0.0, 8.0], 14.0)

    assert r.num_speakers == 1
    # chunk 1's 2.0-6.0 is now 10.0-14.0 globally.
    speakers = {s.speaker for s in r.segments}
    assert speakers == {"SPEAKER_0"}
    ends = [s.end for s in r.segments]
    assert max(ends) == 14.0


def test_stitch_distinct_speakers_allocate_new_labels():
    """Two chunks with orthogonal speaker embeddings get unique global labels."""
    c0 = DiarResult(
        segments=[DiarSegment("SPEAKER_0", 0.0, 2.0)],
        num_speakers=1,
        duration_seconds=5.0,
        speaker_embeddings={"SPEAKER_0": _emb(1, 0, 0)},
    )
    c1 = DiarResult(
        segments=[DiarSegment("SPEAKER_0", 0.0, 2.0)],
        num_speakers=1,
        duration_seconds=5.0,
        speaker_embeddings={"SPEAKER_0": _emb(0, 1, 0)},
    )
    r = _stitch_chunks([c0, c1], [0.0, 5.0], 10.0)

    assert r.num_speakers == 2
    labels = sorted({s.speaker for s in r.segments})
    assert labels == ["SPEAKER_0", "SPEAKER_1"]


def test_stitch_overlap_dedup():
    """Same-speaker segments that overlap at the chunk seam collapse."""
    e = _emb(1, 0)
    c0 = DiarResult(
        segments=[DiarSegment("SPEAKER_0", 0.0, 10.0)],
        num_speakers=1,
        duration_seconds=10.0,
        speaker_embeddings={"SPEAKER_0": e},
    )
    c1 = DiarResult(
        segments=[DiarSegment("SPEAKER_0", 0.0, 5.0)],
        num_speakers=1,
        duration_seconds=10.0,
        speaker_embeddings={"SPEAKER_0": e},
    )
    # Chunk 1 starts at 8.0 globally, so its 0-5 → 8-13. Overlaps chunk 0's 0-10.
    r = _stitch_chunks([c0, c1], [0.0, 8.0], 15.0, merge_gap=0.0)

    assert len(r.segments) == 1
    assert r.segments[0].speaker == "SPEAKER_0"
    assert r.segments[0].start == 0.0
    assert r.segments[0].end == 13.0


def test_stitch_three_speakers_across_three_chunks():
    """A plausible meeting: alice appears in chunks 0+1, bob in chunks 1+2, carol only in chunk 2."""
    alice = _emb(1, 0, 0)
    bob = _emb(0, 1, 0)
    carol = _emb(0, 0, 1)

    c0 = DiarResult(
        segments=[DiarSegment("SPEAKER_0", 0.0, 4.0)],
        num_speakers=1,
        duration_seconds=5.0,
        speaker_embeddings={"SPEAKER_0": alice},
    )
    c1 = DiarResult(
        segments=[
            DiarSegment("SPEAKER_0", 0.0, 2.0),
            DiarSegment("SPEAKER_1", 2.0, 5.0),
        ],
        num_speakers=2,
        duration_seconds=5.0,
        speaker_embeddings={"SPEAKER_0": alice, "SPEAKER_1": bob},
    )
    c2 = DiarResult(
        segments=[
            DiarSegment("SPEAKER_0", 0.0, 2.0),
            DiarSegment("SPEAKER_1", 2.0, 5.0),
        ],
        num_speakers=2,
        duration_seconds=5.0,
        speaker_embeddings={"SPEAKER_0": bob, "SPEAKER_1": carol},
    )

    r = _stitch_chunks([c0, c1, c2], [0.0, 4.0, 8.0], 13.0)
    assert r.num_speakers == 3
    # All alice segments share a global label, all bob segments share another.
    segs_by_global: dict[str, list[DiarSegment]] = {}
    for s in r.segments:
        segs_by_global.setdefault(s.speaker, []).append(s)
    # There should be exactly 3 distinct labels.
    assert len(segs_by_global) == 3
