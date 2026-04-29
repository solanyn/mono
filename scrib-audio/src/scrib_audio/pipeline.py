"""Full audio processing pipeline: audio → diarized transcript.

Orchestrates diarization, transcription, and alignment into a single
call that scrib-server can invoke.
"""

import logging
from dataclasses import asdict, dataclass, field

import numpy as np
import soundfile as sf

from .align import AlignedSegment, align
from .diarize import diarize_array
from .transcribe import Transcript, transcribe_array

log = logging.getLogger(__name__)


@dataclass
class PipelineResult:
    segments: list[AlignedSegment]
    num_speakers: int
    duration_seconds: float
    transcript_text: str
    speaker_embeddings: dict[str, list[float]] = field(default_factory=dict)


def _load_and_normalise(audio_path: str) -> tuple[np.ndarray, int]:
    """Read audio → float32 mono 16kHz. Single normalisation point for the pipeline."""
    data, sr = sf.read(audio_path, dtype="float32")
    if data.ndim > 1 and data.shape[1] > 1:
        data = np.mean(data, axis=1)
    if sr != 16000:
        import librosa
        data = librosa.resample(data, orig_sr=sr, target_sr=16000)
        sr = 16000
    if data.ndim != 1:
        data = data.reshape(-1)
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
    diar_result = diarize_array(
        data, sr,
        threshold=threshold,
        min_duration=min_duration,
        merge_gap=merge_gap,
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

