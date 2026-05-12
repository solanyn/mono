"""Speaker diarization via Senko.

Uses Senko (CAM++ embeddings + spectral/HDBSCAN clustering) with
CoreML acceleration on Apple Silicon. Processes 1 hour in ~8 seconds.

Long recordings (>~10min) are split into overlapping windows so no
single Senko invocation exceeds Metal's per-buffer ceiling. Per-chunk
speaker labels are stitched back together using the CAM++ embeddings
Senko emits per speaker; two chunks' local ``SPEAKER_0``s collapse to
one global label when their embedding cosine exceeds the merge
threshold.
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

# Chunking knobs. An hour of 16kHz float32 is ~230 MB of array; Senko's
# CAM++ embeddings + spectral clustering expand that into buffers that
# cross the ~9.5 GB per-buffer Metal ceiling well before the full file
# fits. 600s chunks stay safely under the ceiling on an M4 Mini; 30s
# overlap gives re-clustering a shared window to match speakers across
# chunk seams.
_DIAR_CHUNK_SECS = float(os.environ.get("SCRIB_AUDIO_DIAR_CHUNK_SECS", 600.0))
_DIAR_OVERLAP_SECS = float(os.environ.get("SCRIB_AUDIO_DIAR_OVERLAP_SECS", 30.0))
# Speakers within the same recording, same mic, same ambient profile
# tend to score high on cosine similarity. 0.70 is looser than the
# 0.75 the Go matcher uses cross-meeting because within-meeting we
# want to merge aggressively.
_DIAR_MERGE_SIM = float(os.environ.get("SCRIB_AUDIO_DIAR_MERGE_SIM", 0.70))


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


def diarize_chunked(
    data: np.ndarray,
    sr: int,
    threshold: float = 0.5,
    min_duration: float = 0.0,
    merge_gap: float = 0.0,
    chunk_secs: float | None = None,
    overlap_secs: float | None = None,
    progress: callable = None,
) -> DiarResult:
    """Run diarization on a long in-memory array by splitting into windows.

    Each window is diarised independently, then the per-chunk ``DiarResult``s
    are stitched into a single coherent result using the CAM++ embeddings
    Senko emits per local speaker.

    If the full array fits in one chunk we short-circuit to
    :func:`diarize_array` to avoid a pointless stitch pass.
    """
    if sr != 16000:
        raise ValueError(f"diarize_chunked expects 16kHz, got {sr}")
    if data.ndim != 1:
        raise ValueError(f"diarize_chunked expects mono, got shape {data.shape}")

    chunk_secs = chunk_secs if chunk_secs is not None else _DIAR_CHUNK_SECS
    overlap_secs = overlap_secs if overlap_secs is not None else _DIAR_OVERLAP_SECS

    duration = len(data) / sr
    if duration <= chunk_secs:
        return diarize_array(data, sr, threshold, min_duration, merge_gap)

    def _emit(detail: str):
        if progress is not None:
            try:
                progress("diarize_chunk", detail)
            except Exception:
                log.exception("progress callback raised")

    chunk_len = int(chunk_secs * sr)
    step = int((chunk_secs - overlap_secs) * sr)
    if step <= 0:
        raise ValueError("diarize_chunked: overlap must be less than chunk")

    chunks: list[DiarResult] = []
    offsets: list[float] = []
    start = 0
    index = 0
    total = max(1, (len(data) - chunk_len) // step + 1 + 1)
    while start < len(data):
        end = min(start + chunk_len, len(data))
        window = data[start:end]
        offset_secs = start / sr
        log.info(
            "diarize_chunked: chunk %d/%d offset=%.1fs len=%.1fs",
            index + 1, total, offset_secs, (end - start) / sr,
        )
        _emit(f"chunk {index + 1}/{total} @ {offset_secs:.0f}s")
        chunk = diarize_array(window, sr, threshold, min_duration, merge_gap)
        chunks.append(chunk)
        offsets.append(offset_secs)
        index += 1
        if end >= len(data):
            break
        start += step

    return _stitch_chunks(chunks, offsets, duration, merge_gap=merge_gap)


def _stitch_chunks(
    chunks: list["DiarResult"],
    offsets: list[float],
    total_duration: float,
    merge_gap: float = 0.5,
    sim_threshold: float | None = None,
) -> "DiarResult":
    """Stitch per-chunk DiarResults into a single coherent result.

    Pure function, no Senko dependency — makes unit testing trivial.

    For each chunk:
      1. Translate segment times into the global timeline.
      2. For each local speaker, match against the running global pool
         by cosine similarity against the pool's running-mean embedding.
      3. If best match >= sim_threshold, reuse that global label and
         blend the embedding (running average). Otherwise allocate a
         new global label.
      4. Rewrite the chunk's segments with the global label.
    Finally dedupe overlapping segments (same speaker, overlapping in
    time) and merge adjacent-same-speaker gaps via ``_merge_segments``.
    """
    if not chunks:
        return DiarResult(segments=[], num_speakers=0, duration_seconds=0.0)
    if len(chunks) == 1:
        only = chunks[0]
        return DiarResult(
            segments=only.segments,
            num_speakers=only.num_speakers,
            duration_seconds=total_duration,
            speaker_embeddings=only.speaker_embeddings,
        )

    sim_threshold = sim_threshold if sim_threshold is not None else _DIAR_MERGE_SIM

    # Global state: next label id, per-global embedding running mean,
    # and a count to weight the mean.
    next_id = 0
    global_emb: dict[str, np.ndarray] = {}
    global_count: dict[str, int] = {}
    out_segments: list[DiarSegment] = []

    def _alloc() -> str:
        nonlocal next_id
        label = f"SPEAKER_{next_id}"
        next_id += 1
        return label

    for chunk, offset in zip(chunks, offsets):
        # local → global label map, resolved per chunk.
        local_to_global: dict[str, str] = {}
        for local_label, emb in chunk.speaker_embeddings.items():
            e = np.asarray(emb, dtype=np.float32)
            n = np.linalg.norm(e)
            if n > 0:
                e = e / n

            best_label: str | None = None
            best_sim = -1.0
            for g_label, g_emb in global_emb.items():
                sim = float(np.dot(e, g_emb))
                if sim > best_sim:
                    best_sim = sim
                    best_label = g_label

            if best_label is not None and best_sim >= sim_threshold:
                # Blend into running mean.
                count = global_count[best_label]
                blended = (global_emb[best_label] * count + e) / (count + 1)
                norm = np.linalg.norm(blended)
                if norm > 0:
                    blended = blended / norm
                global_emb[best_label] = blended
                global_count[best_label] = count + 1
                local_to_global[local_label] = best_label
            else:
                new_label = _alloc()
                global_emb[new_label] = e
                global_count[new_label] = 1
                local_to_global[local_label] = new_label

        # Local speakers with no embedding get their own isolated labels
        # so they don't all collapse onto the same UNKNOWN.
        for seg in chunk.segments:
            if seg.speaker in local_to_global:
                gs = local_to_global[seg.speaker]
            else:
                gs = _alloc()
                local_to_global[seg.speaker] = gs
            out_segments.append(DiarSegment(
                speaker=gs,
                start=round(seg.start + offset, 3),
                end=round(seg.end + offset, 3),
            ))

    out_segments = _dedupe_overlap(out_segments)
    if merge_gap > 0:
        out_segments = _merge_segments(out_segments, merge_gap)

    return DiarResult(
        segments=out_segments,
        num_speakers=len(global_emb) or len({s.speaker for s in out_segments}),
        duration_seconds=round(total_duration, 3),
        speaker_embeddings={k: v.astype(np.float32).tolist() for k, v in global_emb.items()},
    )


def _dedupe_overlap(segments: list[DiarSegment]) -> list[DiarSegment]:
    """Collapse overlapping same-speaker segments that arise from chunk overlap.

    Two chunks' overlap region can emit near-duplicate segments once
    labels are unified. We sort by (start, end) and fold any segment
    that starts inside the previous one (same speaker) into the running
    end.
    """
    if not segments:
        return []
    segments.sort(key=lambda s: (s.start, s.end))
    out = [segments[0]]
    for seg in segments[1:]:
        prev = out[-1]
        if seg.speaker == prev.speaker and seg.start <= prev.end:
            prev.end = max(prev.end, seg.end)
        else:
            out.append(seg)
    return out


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
