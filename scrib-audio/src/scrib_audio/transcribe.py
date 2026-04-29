"""Speech-to-text via Parakeet TDT.

Wraps mlx_audio STT to produce word-level timestamps for alignment
with diarization segments.

Parakeet returns AlignedResult with sentences, each containing sub-word
AlignedTokens. We merge tokens into words using space-prefix boundaries.
"""

import logging
import os
import tempfile
from dataclasses import dataclass

import numpy as np
import soundfile as sf

log = logging.getLogger(__name__)

_model = None
_model_name: str = "mlx-community/parakeet-tdt-0.6b-v3"


@dataclass
class Word:
    word: str
    start: float
    end: float


@dataclass
class Transcript:
    text: str
    words: list[Word]


def load_model(model_name: str | None = None):
    """Load STT model (lazy singleton)."""
    global _model, _model_name
    if model_name:
        _model_name = model_name
    if _model is None:
        from mlx_audio.stt import load
        log.info("loading STT model: %s", _model_name)
        _model = load(_model_name)
    return _model


def _tokens_to_words(sentences) -> list[Word]:
    """Merge sub-word AlignedTokens into full words."""
    words: list[Word] = []
    current_text = ""
    current_start = 0.0
    current_end = 0.0

    for sentence in sentences:
        for token in sentence.tokens:
            text = token.text
            if text.startswith(" ") and current_text:
                words.append(Word(
                    word=current_text.strip(),
                    start=round(current_start, 3),
                    end=round(current_end, 3),
                ))
                current_text = text
                current_start = token.start
                current_end = token.end
            elif not current_text:
                current_text = text
                current_start = token.start
                current_end = token.end
            else:
                current_text += text
                current_end = token.end

    if current_text.strip():
        words.append(Word(
            word=current_text.strip(),
            start=round(current_start, 3),
            end=round(current_end, 3),
        ))

    return words


def transcribe_array(data: np.ndarray, sr: int) -> Transcript:
    """Transcribe a normalised float32 mono 16kHz array.

    mlx_audio's model.generate expects a file path, so we write a single
    short-lived tempfile. Caller owns normalisation.
    """
    if sr != 16000:
        raise ValueError(f"transcribe_array expects 16kHz, got {sr}")
    if data.ndim != 1:
        raise ValueError(f"transcribe_array expects mono, got shape {data.shape}")

    model = load_model()

    tmp_fd, tmp_path = tempfile.mkstemp(suffix=".wav")
    os.close(tmp_fd)
    try:
        sf.write(tmp_path, data, sr)
        log.info("transcribing %.0fs of audio", len(data) / sr)
        result = model.generate(tmp_path, verbose=True)
    finally:
        try:
            os.unlink(tmp_path)
        except OSError:
            pass

    words = _tokens_to_words(result.sentences)
    log.info("transcribed %d words from %d sentences", len(words), len(result.sentences))
    return Transcript(text=result.text, words=words)


def transcribe(audio_path: str) -> Transcript:
    """Transcribe an audio file. Handles its own normalisation.

    Prefer :func:`transcribe_array` when the caller has already normalised.
    """
    data, sr = sf.read(audio_path, dtype="float32")
    if data.ndim > 1 and data.shape[1] > 1:
        data = np.mean(data, axis=1)
    if sr != 16000:
        import librosa
        data = librosa.resample(data, orig_sr=sr, target_sr=16000)
        sr = 16000
    return transcribe_array(data, sr)
