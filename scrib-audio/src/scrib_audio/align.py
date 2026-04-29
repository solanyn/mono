"""Align STT words to diarization speaker segments.

Ported from scrib/server/process.go alignSpeakersToWords().
Maps each word to the speaker with the best temporal overlap.
"""

from dataclasses import dataclass

from .diarize import DiarSegment
from .transcribe import Word


@dataclass
class AlignedSegment:
    speaker: str
    start: float
    end: float
    text: str


def align(
    diar_segments: list[DiarSegment],
    words: list[Word],
    duration: float,
) -> list[AlignedSegment]:
    """Align transcribed words to speaker diarization segments.

    For each word, finds the diarization segment with the best temporal
    overlap. Adjacent words from the same speaker are merged into
    contiguous segments.

    Args:
        diar_segments: Speaker diarization segments with timestamps.
        words: Transcribed words with timestamps.
        duration: Total audio duration in seconds.

    Returns:
        List of aligned segments, each with speaker, timestamps, and text.
    """
    if not diar_segments or not words:
        text = " ".join(w.word for w in words) if words else ""
        return [AlignedSegment(
            speaker="SPEAKER_0",
            start=0.0,
            end=duration,
            text=text,
        )]

    # Tag each word with best-matching speaker
    tagged: list[tuple[str, Word]] = []
    for w in words:
        best_overlap = 0.0
        best_speaker = "UNKNOWN"

        for seg in diar_segments:
            overlap_start = max(w.start, seg.start)
            overlap_end = min(w.end, seg.end)
            if overlap_end > overlap_start:
                overlap = overlap_end - overlap_start
                if overlap > best_overlap:
                    best_overlap = overlap
                    best_speaker = seg.speaker

        # Fallback: nearest segment by midpoint
        if best_speaker == "UNKNOWN":
            mid = (w.start + w.end) / 2
            min_dist = float("inf")
            for seg in diar_segments:
                d = min(abs(mid - seg.start), abs(mid - seg.end))
                if d < min_dist:
                    min_dist = d
                    best_speaker = seg.speaker

        tagged.append((best_speaker, w))

    # Merge adjacent words from same speaker
    segments: list[AlignedSegment] = []
    if not tagged:
        return segments

    cur_speaker, cur_word = tagged[0]
    cur = AlignedSegment(
        speaker=cur_speaker,
        start=cur_word.start,
        end=cur_word.end,
        text=cur_word.word,
    )

    for speaker, word in tagged[1:]:
        if speaker == cur.speaker:
            cur.end = word.end
            cur.text += " " + word.word
        else:
            segments.append(cur)
            cur = AlignedSegment(
                speaker=speaker,
                start=word.start,
                end=word.end,
                text=word.word,
            )
    segments.append(cur)

    segments.sort(key=lambda s: s.start)
    return segments
