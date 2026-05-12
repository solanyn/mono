"""Align STT words to diarization speaker segments.

Ported from scrib/server/process.go alignSpeakersToWords().
Maps each word to the speaker with the best temporal overlap.
"""

from dataclasses import dataclass, field

from .diarize import DiarSegment
from .transcribe import Word


@dataclass
class AlignedSegment:
    speaker: str
    start: float
    end: float
    text: str
    uncertain: bool = False


def align(
    diar_segments: list[DiarSegment],
    words: list[Word],
    duration: float,
) -> list[AlignedSegment]:
    """Align transcribed words to speaker diarization segments.

    For each word, finds the diarization segment with the best temporal
    overlap. Adjacent words from the same speaker are merged into
    contiguous segments.

    A word/segment is flagged ``uncertain`` when the best speaker was
    assigned via nearest-midpoint fallback or when the temporal overlap
    covered less than half the word's duration. This matches the Go
    original in scrib/server/process.go.
    """
    if not diar_segments or not words:
        text = " ".join(w.word for w in words) if words else ""
        return [AlignedSegment(
            speaker="SPEAKER_0",
            start=0.0,
            end=duration,
            text=text,
        )]

    tagged: list[tuple[str, Word, bool]] = []
    for w in words:
        word_dur = w.end - w.start
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

        uncertain = False
        if best_speaker == "UNKNOWN":
            mid = (w.start + w.end) / 2
            min_dist = float("inf")
            for seg in diar_segments:
                d = min(abs(mid - seg.start), abs(mid - seg.end))
                if d < min_dist:
                    min_dist = d
                    best_speaker = seg.speaker
            uncertain = True
        elif word_dur > 0 and (best_overlap / word_dur) < 0.5:
            uncertain = True

        tagged.append((best_speaker, w, uncertain))

    segments: list[AlignedSegment] = []
    if not tagged:
        return segments

    cur_speaker, cur_word, cur_uncertain = tagged[0]
    cur = AlignedSegment(
        speaker=cur_speaker,
        start=cur_word.start,
        end=cur_word.end,
        text=cur_word.word,
        uncertain=cur_uncertain,
    )

    for speaker, word, uncertain in tagged[1:]:
        if speaker == cur.speaker:
            cur.end = word.end
            cur.text += " " + word.word
            if uncertain:
                cur.uncertain = True
        else:
            segments.append(cur)
            cur = AlignedSegment(
                speaker=speaker,
                start=word.start,
                end=word.end,
                text=word.word,
                uncertain=uncertain,
            )
    segments.append(cur)

    segments.sort(key=lambda s: s.start)
    return segments
