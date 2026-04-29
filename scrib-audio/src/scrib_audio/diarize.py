"""Speaker diarization via Senko.

Uses Senko (CAM++ embeddings + spectral/HDBSCAN clustering) with
CoreML acceleration on Apple Silicon. Processes 1 hour in ~8 seconds.
"""

import logging
import os
import tempfile
from collections import defaultdict
from dataclasses import dataclass, field

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
    speaker_embeddings: dict[str, list[float]] = field(default_factory=dict)


def load_model():
    """Load Senko diarizer (lazy singleton)."""
    global _diarizer
    if _diarizer is None:
        import senko
        log.info("loading Senko diarizer (device=auto)")
        _diarizer = senko.Diarizer(device="auto", warmup=True, quiet=True)
    return _diarizer


def _extract_speaker_embeddings(result: dict) -> dict[str, list[float]]:
    """Best-effort per-speaker embedding extraction from Senko output.

    Senko's result shape varies by version. We try the most common keys
    and fall back to an empty dict; the Go matcher then treats this
    meeting as label-only.
    """
    # Direct speaker→embedding map
    if "speaker_embeddings" in result:
        raw = result["speaker_embeddings"]
        return {str(k): list(map(float, v)) for k, v in raw.items()}

    # Per-segment embeddings; average per speaker
    segments = result.get("segments") or result.get("merged_segments") or []
    by_speaker: dict[str, list[np.ndarray]] = defaultdict(list)
    for seg in segments:
        emb = seg.get("embedding") if isinstance(seg, dict) else None
        if emb is None:
            continue
        speaker = str(seg.get("speaker", "UNKNOWN"))
        by_speaker[speaker].append(np.asarray(emb, dtype=np.float32))

    out: dict[str, list[float]] = {}
    for speaker, embs in by_speaker.items():
        mean = np.mean(np.stack(embs), axis=0)
        norm = np.linalg.norm(mean)
        if norm > 0:
            mean = mean / norm
        out[speaker] = mean.astype(np.float32).tolist()
    return out


def diarize_array(
    data: np.ndarray,
    sr: int,
    threshold: float = 0.5,
    min_duration: float = 0.0,
    merge_gap: float = 0.0,
) -> DiarResult:
    """Run diarization on an in-memory float32 mono 16kHz array.

    Senko requires a file path, so we write a single short-lived tempfile.
    Caller owns normalisation; this function does not resample.
    """
    if sr != 16000:
        raise ValueError(f"diarize_array expects 16kHz, got {sr}")
    if data.ndim != 1:
        raise ValueError(f"diarize_array expects mono, got shape {data.shape}")

    diarizer = load_model()
    duration = len(data) / sr

    tmp_fd, tmp_path = tempfile.mkstemp(suffix=".wav")
    os.close(tmp_fd)
    try:
        sf.write(tmp_path, data, sr, subtype="PCM_16")
        log.info("diarize: processing %.0fs audio with Senko", duration)
        result = diarizer.diarize(tmp_path, generate_colors=False)
    finally:
        try:
            os.unlink(tmp_path)
        except OSError:
            pass

    segments_raw = result.get("merged_segments", [])
    speaker_set: set[str] = set()
    segments: list[DiarSegment] = []

    for seg in segments_raw:
        speaker = seg.get("speaker", "UNKNOWN")
        start = seg.get("start", 0.0)
        end = seg.get("end", 0.0)
        if (end - start) < min_duration:
            continue
        segments.append(DiarSegment(
            speaker=speaker,
            start=round(start, 3),
            end=round(end, 3),
        ))
        speaker_set.add(speaker)

    if merge_gap > 0:
        segments = _merge_segments(segments, merge_gap)

    embeddings = _extract_speaker_embeddings(result)
    if not embeddings:
        log.info("diarize: no per-speaker embeddings available from diarizer output")

    log.info(
        "diarize: %d segments, %d speakers in %.1fs (%d embeddings)",
        len(segments), len(speaker_set), duration, len(embeddings),
    )

    return DiarResult(
        segments=segments,
        num_speakers=len(speaker_set),
        duration_seconds=round(duration, 3),
        speaker_embeddings=embeddings,
    )


def diarize(
    audio_path: str,
    threshold: float = 0.5,
    min_duration: float = 0.0,
    merge_gap: float = 0.0,
) -> DiarResult:
    """Run diarization on a file path. Handles its own normalisation.

    Prefer :func:`diarize_array` when caller has already normalised.
    """
    data, sr = sf.read(audio_path, dtype="float32")
    if data.ndim > 1 and data.shape[1] > 1:
        data = np.mean(data, axis=1)
    if sr != 16000:
        import librosa
        data = librosa.resample(data, orig_sr=sr, target_sr=16000)
        sr = 16000
    return diarize_array(data, sr, threshold, min_duration, merge_gap)


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
