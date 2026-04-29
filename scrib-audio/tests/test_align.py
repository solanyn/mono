"""Tests for word-to-speaker alignment."""
import pytest

from scrib_audio.align import AlignedSegment, align
from scrib_audio.diarize import DiarSegment
from scrib_audio.transcribe import Word


def test_empty_segments_returns_single():
    words = [Word("hello", 0.0, 0.5), Word("world", 0.5, 1.0)]
    result = align([], words, 1.0)
    assert len(result) == 1
    assert result[0].speaker == "SPEAKER_0"
    assert result[0].text == "hello world"


def test_empty_words_returns_single():
    segs = [DiarSegment("SPEAKER_0", 0.0, 5.0)]
    result = align(segs, [], 5.0)
    assert len(result) == 1
    assert result[0].text == ""


def test_single_speaker():
    segs = [DiarSegment("SPEAKER_0", 0.0, 3.0)]
    words = [
        Word("hello", 0.0, 0.5),
        Word("world", 0.5, 1.0),
        Word("foo", 1.0, 1.5),
    ]
    result = align(segs, words, 3.0)
    assert len(result) == 1
    assert result[0].speaker == "SPEAKER_0"
    assert result[0].text == "hello world foo"


def test_two_speakers():
    segs = [
        DiarSegment("SPEAKER_0", 0.0, 2.0),
        DiarSegment("SPEAKER_1", 2.0, 4.0),
    ]
    words = [
        Word("hello", 0.0, 0.5),
        Word("world", 0.5, 1.0),
        Word("how", 2.0, 2.5),
        Word("are", 2.5, 3.0),
        Word("you", 3.0, 3.5),
    ]
    result = align(segs, words, 4.0)
    assert len(result) == 2
    assert result[0].speaker == "SPEAKER_0"
    assert result[0].text == "hello world"
    assert result[1].speaker == "SPEAKER_1"
    assert result[1].text == "how are you"


def test_speaker_change_mid_sentence():
    segs = [
        DiarSegment("SPEAKER_0", 0.0, 1.5),
        DiarSegment("SPEAKER_1", 1.5, 3.0),
        DiarSegment("SPEAKER_0", 3.0, 5.0),
    ]
    words = [
        Word("I", 0.0, 0.3),
        Word("think", 0.3, 0.8),
        Word("yes", 1.5, 2.0),
        Word("exactly", 2.0, 2.8),
        Word("so", 3.0, 3.3),
        Word("anyway", 3.3, 4.0),
    ]
    result = align(segs, words, 5.0)
    assert len(result) == 3
    assert result[0].speaker == "SPEAKER_0"
    assert result[1].speaker == "SPEAKER_1"
    assert result[2].speaker == "SPEAKER_0"


def test_word_in_gap_uses_nearest():
    """Word between two segments should be assigned to nearest speaker."""
    segs = [
        DiarSegment("SPEAKER_0", 0.0, 1.0),
        DiarSegment("SPEAKER_1", 3.0, 5.0),
    ]
    words = [
        Word("hello", 0.0, 0.5),
        Word("um", 1.5, 2.0),  # in gap, closer to SPEAKER_0
        Word("yes", 3.0, 3.5),
    ]
    result = align(segs, words, 5.0)
    assert result[0].speaker == "SPEAKER_0"
    assert "um" in result[0].text  # merged with SPEAKER_0
    assert result[-1].speaker == "SPEAKER_1"
