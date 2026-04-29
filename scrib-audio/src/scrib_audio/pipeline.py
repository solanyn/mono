"""Full audio processing pipeline: audio → diarized transcript.

Orchestrates diarization, transcription, and alignment into a single
call that scrib-server can invoke.
"""

import logging
import tempfile
from dataclasses import asdict, dataclass

import numpy as np
import soundfile as sf

from .align import AlignedSegment, align
from .diarize import DiarResult, diarize
from .transcribe import Transcript, transcribe

log = logging.getLogger(__name__)


@dataclass
class PipelineResult:
    segments: list[AlignedSegment]
    num_speakers: int
    duration_seconds: float
    transcript_text: str


def process(
    audio_path: str,
    threshold: float = 0.5,
    min_duration: float = 0.0,
    merge_gap: float = 0.5,
) -> PipelineResult:
    """Run the full audio pipeline on a file.

    Sequential execution: diarize first, then transcribe, then align.
    mlx-audio models are single-worker so parallel would cause contention.

    Args:
        audio_path: Path to audio file (WAV preferred, any format soundfile supports).
        threshold: Diarization speaker activity threshold.
        min_duration: Minimum diarization segment duration.
        merge_gap: Merge same-speaker segments within this gap (seconds).

    Returns:
        PipelineResult with aligned speaker segments and metadata.
    """
    log.info("pipeline: starting on %s", audio_path)

    # Normalize audio to 16kHz mono WAV for both models
    data, sr = sf.read(audio_path, dtype="float32")
    if len(data.shape) > 1 and data.shape[1] > 1:
        data = np.mean(data, axis=1)
    if sr != 16000:
        import librosa
        data = librosa.resample(data, orig_sr=sr, target_sr=16000)
        sr = 16000

    duration = len(data) / sr
    log.info("pipeline: %.0fs audio, %d samples", duration, len(data))

    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
        sf.write(tmp.name, data, sr)
        norm_path = tmp.name

    try:
        # Step 1: Diarize
        diar_result = diarize(
            norm_path,
            threshold=threshold,
            min_duration=min_duration,
            merge_gap=merge_gap,
        )
        log.info(
            "pipeline: diarization done — %d segments, %d speakers",
            len(diar_result.segments),
            diar_result.num_speakers,
        )

        # Step 2: Transcribe
        transcript = transcribe(norm_path)
        log.info(
            "pipeline: transcription done — %d words",
            len(transcript.words),
        )

        # Step 3: Align
        segments = align(
            diar_result.segments,
            transcript.words,
            diar_result.duration_seconds,
        )
        log.info("pipeline: alignment done — %d segments", len(segments))

        return PipelineResult(
            segments=segments,
            num_speakers=diar_result.num_speakers,
            duration_seconds=diar_result.duration_seconds,
            transcript_text=transcript.text,
        )
    finally:
        import os
        os.unlink(norm_path)


def result_to_dict(result: PipelineResult) -> dict:
    """Serialize PipelineResult to JSON-friendly dict."""
    return {
        "segments": [asdict(s) for s in result.segments],
        "num_speakers": result.num_speakers,
        "duration_seconds": result.duration_seconds,
        "transcript_text": result.transcript_text,
    }
