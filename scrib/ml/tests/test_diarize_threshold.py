"""Tests for diarize_array threshold filtering."""

from unittest.mock import patch, MagicMock

import numpy as np
import pytest

from scrib_ml.diarize import diarize_array, DiarSegment


def _make_audio_with_loud_and_quiet(sr=16000):
    """Create 10s audio: loud speech 0-5s, near-silence 5-10s."""
    loud = 0.1 * np.random.default_rng(42).normal(0, 1, sr * 5).astype(np.float32)
    quiet = 0.001 * np.random.default_rng(42).normal(0, 1, sr * 5).astype(np.float32)
    return np.concatenate([loud, quiet])


def _mock_diarizer_result():
    """Fake Senko output with two segments: one loud, one quiet."""
    return {
        "merged_segments": [
            {"speaker": "SPEAKER_0", "start": 0.0, "end": 5.0},
            {"speaker": "SPEAKER_1", "start": 5.0, "end": 10.0},
        ],
    }


class TestThresholdFilter:
    @patch("scrib_ml.diarize.load_model")
    def test_threshold_filters_quiet_segments(self, mock_load):
        mock_diarizer = MagicMock()
        mock_diarizer.diarize.return_value = _mock_diarizer_result()
        mock_load.return_value = mock_diarizer

        data = _make_audio_with_loud_and_quiet()
        result = diarize_array(data, 16000, threshold=0.5)

        speakers = [s.speaker for s in result.segments]
        assert "SPEAKER_0" in speakers
        assert "SPEAKER_1" not in speakers

    @patch("scrib_ml.diarize.load_model")
    def test_zero_threshold_keeps_all(self, mock_load):
        mock_diarizer = MagicMock()
        mock_diarizer.diarize.return_value = _mock_diarizer_result()
        mock_load.return_value = mock_diarizer

        data = _make_audio_with_loud_and_quiet()
        result = diarize_array(data, 16000, threshold=0.0)

        speakers = [s.speaker for s in result.segments]
        assert "SPEAKER_0" in speakers
        assert "SPEAKER_1" in speakers

    @patch("scrib_ml.diarize.load_model")
    def test_high_threshold_filters_more(self, mock_load):
        mock_diarizer = MagicMock()
        mock_diarizer.diarize.return_value = _mock_diarizer_result()
        mock_load.return_value = mock_diarizer

        data = _make_audio_with_loud_and_quiet()
        result_low = diarize_array(data, 16000, threshold=0.1)
        result_high = diarize_array(data, 16000, threshold=0.9)

        assert len(result_high.segments) <= len(result_low.segments)

    @patch("scrib_ml.diarize.load_model")
    def test_threshold_with_uniform_audio_keeps_all(self, mock_load):
        """When all segments have similar energy, none get filtered."""
        mock_diarizer = MagicMock()
        mock_diarizer.diarize.return_value = {
            "merged_segments": [
                {"speaker": "SPEAKER_0", "start": 0.0, "end": 3.0},
                {"speaker": "SPEAKER_1", "start": 3.0, "end": 6.0},
            ],
        }
        mock_load.return_value = mock_diarizer

        rng = np.random.default_rng(42)
        data = 0.1 * rng.normal(0, 1, 16000 * 6).astype(np.float32)
        result = diarize_array(data, 16000, threshold=0.5)

        assert len(result.segments) == 2

    @patch("scrib_ml.diarize.load_model")
    def test_threshold_does_not_affect_timestamps(self, mock_load):
        mock_diarizer = MagicMock()
        mock_diarizer.diarize.return_value = _mock_diarizer_result()
        mock_load.return_value = mock_diarizer

        data = _make_audio_with_loud_and_quiet()
        result = diarize_array(data, 16000, threshold=0.5)

        for seg in result.segments:
            if seg.speaker == "SPEAKER_0":
                assert seg.start == 0.0
                assert seg.end == 5.0
