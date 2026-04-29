"""Speech-to-text via Parakeet TDT.

Wraps mlx_audio STT to produce word-level timestamps for alignment
with diarization segments.

Parakeet returns AlignedResult with sentences, each containing sub-word
AlignedTokens. We merge tokens into words using space-prefix boundaries.
"""

import logging
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
    """Merge sub-word AlignedTokens into full words.

    Parakeet tokens use space prefix to indicate word boundaries:
        [" U", "h", " no", ",", " not", " y", "et", "."]
    becomes:
        ["Uh", "no,", "not", "yet."]
    """
    words: list[Word] = []
    current_text = ""
    current_start = 0.0
    current_end = 0.0

    for sentence in sentences:
        for token in sentence.tokens:
            text = token.text
            # Space prefix = new word boundary
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
                # First token
                current_text = text
                current_start = token.start
                current_end = token.end
            else:
                # Continuation of current word
                current_text += text
                current_end = token.end

    # Flush last word
    if current_text.strip():
        words.append(Word(
            word=current_text.strip(),
            start=round(current_start, 3),
            end=round(current_end, 3),
        ))

    return words


def transcribe(audio_path: str) -> Transcript:
    """Transcribe audio file to text with word-level timestamps.

    Args:
        audio_path: Path to WAV file (16kHz mono preferred).

    Returns:
        Transcript with full text and per-word timestamps.
    """
    model = load_model()

    data, sr = sf.read(audio_path, dtype="float32")

    # Mono + 16kHz
    if len(data.shape) > 1 and data.shape[1] > 1:
        data = np.mean(data, axis=1)
    if sr != 16000:
        import librosa
        data = librosa.resample(data, orig_sr=sr, target_sr=16000)
        sr = 16000

    with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
        sf.write(tmp.name, data, sr)
        tmp_path = tmp.name

    try:
        log.info("transcribing %.0fs of audio", len(data) / sr)
        result = model.generate(tmp_path, verbose=True)

        words = _tokens_to_words(result.sentences)
        log.info("transcribed %d words from %d sentences", len(words), len(result.sentences))

        return Transcript(text=result.text, words=words)
    finally:
        import os
        os.unlink(tmp_path)
