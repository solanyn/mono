"""Full audio processing pipeline: audio → diarized transcript.

Orchestrates diarization, transcription, and alignment into a single
call that scrib-server can invoke.
"""

import logging
from dataclasses import asdict, dataclass, field

import noisereduce as nr
import numpy as np
import soundfile as sf

from .align import AlignedSegment, align
from .diarize import diarize_chunked
from .transcribe import Transcript, transcribe_array

log = logging.getLogger(__name__)


@dataclass
class PipelineResult:
    segments: list[AlignedSegment]
    num_speakers: int
    duration_seconds: float
    transcript_text: str
    speaker_embeddings: dict[str, list[float]] = field(default_factory=dict)


_TARGET_DBFS = -20.0
_MAX_GAIN = 20.0
_MIN_GAIN = 0.1
_SILENCE_FLOOR_RMS = 1e-6
_TRIM_THRESHOLD = 0.005
_TRIM_MIN_SILENCE_SECS = 1.0


def _trim_silence(data: np.ndarray, sr: int) -> tuple[np.ndarray, float]:
    """Trim leading and trailing silence. Returns trimmed data and offset in seconds.

    Only trims silence regions longer than _TRIM_MIN_SILENCE_SECS.
    Internal silence is preserved for timestamp alignment.
    """
    frame_len = int(0.02 * sr)
    energy = np.array([
        np.sqrt(np.mean(data[i:i + frame_len] ** 2))
        for i in range(0, len(data) - frame_len, frame_len)
    ])

    above = np.where(energy > _TRIM_THRESHOLD)[0]
    if len(above) == 0:
        return data, 0.0

    first_voice = above[0] * frame_len
    last_voice = (above[-1] + 1) * frame_len

    min_samples = int(_TRIM_MIN_SILENCE_SECS * sr)
    start = first_voice if first_voice > min_samples else 0
    end = last_voice if (len(data) - last_voice) > min_samples else len(data)

    offset = start / sr
    return data[start:end], offset


def _rms_normalize(data: np.ndarray, target_dbfs: float = _TARGET_DBFS) -> np.ndarray:
    """Normalize audio RMS to target dBFS. Gain is capped to avoid amplifying noise."""
    rms = float(np.sqrt(np.mean(data**2)))
    if rms < _SILENCE_FLOOR_RMS:
        return data
    target_rms = 10.0 ** (target_dbfs / 20.0)
    gain = target_rms / rms
    gain = max(_MIN_GAIN, min(_MAX_GAIN, gain))
    normalized = data * gain
    peak = float(np.max(np.abs(normalized)))
    if peak > 1.0:
        normalized = normalized / peak
    return normalized


def _reduce_noise(data: np.ndarray, sr: int) -> np.ndarray:
    """Apply stationary noise reduction (fan, HVAC, hum). Conservative to preserve speech."""
    return nr.reduce_noise(
        y=data,
        sr=sr,
        stationary=True,
        prop_decrease=0.6,
        n_fft=512,
        freq_mask_smooth_hz=200,
    )


def _load_and_normalise(audio_path: str) -> tuple[np.ndarray, int]:
    """Read audio → float32 mono 16kHz, trimmed, noise-reduced, RMS-normalized."""
    data, sr = sf.read(audio_path, dtype="float32")
    if data.ndim > 1 and data.shape[1] > 1:
        data = np.mean(data, axis=1)
    if sr != 16000:
        import librosa
        data = librosa.resample(data, orig_sr=sr, target_sr=16000)
        sr = 16000
    if data.ndim != 1:
        data = data.reshape(-1)
    original_dur = len(data) / sr
    data, offset = _trim_silence(data, sr)
    trimmed_dur = len(data) / sr
    if offset > 0 or trimmed_dur < original_dur:
        log.info(
            "pipeline: trimmed %.1fs silence (offset=%.1fs, %.1fs → %.1fs)",
            original_dur - trimmed_dur, offset, original_dur, trimmed_dur,
        )
    data = _reduce_noise(data, sr)
    data = _rms_normalize(data)
    return data, sr


def process(
    audio_path: str,
    threshold: float = 0.5,
    min_duration: float = 0.0,
    merge_gap: float = 0.5,
    progress: callable = None,
) -> PipelineResult:
    """Run the full audio pipeline on a file.

    Sequential execution: diarize first, then transcribe, then align.
    mlx-audio models are single-worker so parallel would cause contention.

    Normalisation happens once here; diarize/transcribe receive the array.
    Each model library still needs a file path internally, so exactly two
    tempfiles are written (one per model invocation) rather than four.

    If ``progress`` is supplied, it is invoked as ``progress(stage, detail)``
    after each pipeline stage so the caller (e.g. the SSE endpoint) can
    push updates to the client.
    """
    log.info("pipeline: starting on %s", audio_path)

    def _emit(stage: str, detail: str = ""):
        if progress is not None:
            try:
                progress(stage, detail)
            except Exception:
                log.exception("progress callback raised")

    _emit("load", audio_path)
    data, sr = _load_and_normalise(audio_path)
    duration = len(data) / sr
    log.info("pipeline: %.0fs audio, %d samples", duration, len(data))

    _emit("diarize", f"{duration:.0f}s audio")
    diar_result = diarize_chunked(
        data, sr,
        threshold=threshold,
        min_duration=min_duration,
        merge_gap=merge_gap,
        progress=progress,
    )
    log.info(
        "pipeline: diarization done — %d segments, %d speakers",
        len(diar_result.segments),
        diar_result.num_speakers,
    )
    _emit("diarize_done", f"{diar_result.num_speakers} speakers")

    _emit("transcribe", f"{duration:.0f}s audio")
    transcript = transcribe_array(data, sr)
    log.info("pipeline: transcription done — %d words", len(transcript.words))
    _emit("transcribe_done", f"{len(transcript.words)} words")

    _emit("align", "")
    segments = align(
        diar_result.segments,
        transcript.words,
        diar_result.duration_seconds,
    )
    log.info("pipeline: alignment done — %d segments", len(segments))
    _emit("align_done", f"{len(segments)} segments")

    return PipelineResult(
        segments=segments,
        num_speakers=diar_result.num_speakers,
        duration_seconds=diar_result.duration_seconds,
        transcript_text=transcript.text,
        speaker_embeddings=diar_result.speaker_embeddings,
    )


def result_to_dict(result: PipelineResult, include_embeddings: bool = False) -> dict:
    """Serialize PipelineResult to JSON-friendly dict."""
    out = {
        "segments": [asdict(s) for s in result.segments],
        "num_speakers": result.num_speakers,
        "duration_seconds": result.duration_seconds,
        "transcript_text": result.transcript_text,
    }
    if include_embeddings and result.speaker_embeddings:
        out["speaker_embeddings"] = [
            {"speaker": speaker, "embedding": embedding}
            for speaker, embedding in sorted(result.speaker_embeddings.items())
        ]
    return out

