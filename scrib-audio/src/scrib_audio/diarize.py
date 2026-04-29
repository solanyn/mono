"""Speaker diarization via Senko.

Uses Senko (CAM++ embeddings + spectral/HDBSCAN clustering) with
CoreML acceleration on Apple Silicon. Processes 1 hour in ~8 seconds.
"""

import logging
import tempfile
from dataclasses import dataclass
from typing import Optional

import numpy as np
import soundfile as sf

log = logging.getLogger(__name__)

_diarizer = None


@dataclass
class DiarSegment:
    speaker: str
    start: float
    end: float


@dataclass
class DiarResult:
    segments: list[DiarSegment]
    num_speakers: int
    duration_seconds: float


def load_model():
    """Load Senko diarizer (lazy singleton)."""
    global _diarizer
    if _diarizer is None:
        import senko
        log.info("loading Senko diarizer (device=auto)")
        _diarizer = senko.Diarizer(device="auto", warmup=True, quiet=True)
    return _diarizer


def diarize(
    audio_path: str,
    threshold: float = 0.5,
    min_duration: float = 0.0,
    merge_gap: float = 0.0,
) -> DiarResult:
    """Run speaker diarization on an audio file.

    Args:
        audio_path: Path to audio file (16kHz mono WAV preferred).
        threshold: Unused (kept for API compat). Senko handles internally.
        min_duration: Minimum segment duration in seconds.
        merge_gap: Merge segments from same speaker within this gap (seconds).

    Returns:
        DiarResult with speaker segments, count, and duration.
    """
    diarizer = load_model()

    # Senko expects 16kHz mono 16-bit WAV
    data, sr = sf.read(audio_path, dtype="float32")
    duration = len(data) / sr

    if len(data.shape) > 1 and data.shape[1] > 1:
        data = np.mean(data, axis=1)

    if sr != 16000:
        import librosa
        data = librosa.resample(data, orig_sr=sr, target_sr=16000)
        sr = 16000

    # Write normalized audio for Senko
    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
        # Senko needs 16-bit PCM
        sf.write(tmp.name, data, sr, subtype="PCM_16")
        tmp_path = tmp.name

    try:
        log.info("diarize: processing %.0fs audio with Senko", duration)
        result = diarizer.diarize(tmp_path, generate_colors=False)

        segments_raw = result.get("merged_segments", [])
        speaker_set: set[str] = set()
        segments: list[DiarSegment] = []

        for seg in segments_raw:
            speaker = seg.get("speaker", "UNKNOWN")
            start = seg.get("start", 0.0)
            end = seg.get("end", 0.0)

            # Apply min_duration filter
            if (end - start) < min_duration:
                continue

            segments.append(DiarSegment(
                speaker=speaker,
                start=round(start, 3),
                end=round(end, 3),
            ))
            speaker_set.add(speaker)

        # Merge adjacent segments from same speaker within gap
        if merge_gap > 0:
            segments = _merge_segments(segments, merge_gap)

        log.info(
            "diarize: %d segments, %d speakers in %.1fs",
            len(segments), len(speaker_set), duration,
        )

        return DiarResult(
            segments=segments,
            num_speakers=len(speaker_set),
            duration_seconds=round(duration, 3),
        )
    finally:
        import os
        os.unlink(tmp_path)


def _merge_segments(
    segments: list[DiarSegment], gap: float = 0.5
) -> list[DiarSegment]:
    """Merge adjacent segments from the same speaker within gap tolerance."""
    if not segments:
        return []

    segments.sort(key=lambda s: s.start)
    merged = [segments[0]]

    for seg in segments[1:]:
        prev = merged[-1]
        if seg.speaker == prev.speaker and (seg.start - prev.end) <= gap:
            prev.end = max(prev.end, seg.end)
        else:
            merged.append(seg)

    return merged
