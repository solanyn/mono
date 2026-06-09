"""Integration tests for pipeline audio normalization."""

import math
import tempfile
import wave
import struct

import numpy as np
import pytest

from scrib_ml.pipeline import _rms_normalize, _reduce_noise, _trim_silence, _load_and_normalise


def _rms_dbfs(data: np.ndarray) -> float:
    rms = float(np.sqrt(np.mean(data**2)))
    if rms < 1e-10:
        return -100.0
    return 20.0 * math.log10(rms)


def _write_wav(path: str, samples: np.ndarray, sr: int = 16000, channels: int = 1):
    int_samples = np.clip(samples * 32767, -32768, 32767).astype(np.int16)
    with wave.open(path, "wb") as w:
        w.setnchannels(channels)
        w.setsampwidth(2)
        w.setframerate(sr)
        w.writeframes(int_samples.tobytes())


class TestRmsNormalize:
    def test_quiet_audio_boosted(self):
        rng = np.random.default_rng(42)
        data = rng.normal(0, 0.003, 16000).astype(np.float32)
        original_rms = _rms_dbfs(data)
        assert original_rms < -45

        result = _rms_normalize(data)
        result_rms = _rms_dbfs(result)
        assert result_rms > original_rms + 20

    def test_moderate_quiet_audio_hits_target(self):
        """Audio at -30 dBFS (typical scrib recording) should reach -20 dBFS."""
        rng = np.random.default_rng(42)
        target_rms = 10.0 ** (-30.0 / 20.0)
        data = rng.normal(0, 1.0, 16000).astype(np.float32)
        data = data * (target_rms / float(np.sqrt(np.mean(data**2))))

        result = _rms_normalize(data)
        result_rms = _rms_dbfs(result)
        assert -22.0 < result_rms < -18.0

    def test_loud_audio_attenuated(self):
        rng = np.random.default_rng(42)
        data = rng.normal(0, 0.5, 16000).astype(np.float32)
        original_rms = _rms_dbfs(data)
        assert original_rms > -10

        result = _rms_normalize(data)
        result_rms = _rms_dbfs(result)
        assert -22.0 < result_rms < -18.0

    def test_already_normalized_unchanged(self):
        target_rms = 10.0 ** (-20.0 / 20.0)
        rng = np.random.default_rng(42)
        data = rng.normal(0, 1.0, 16000).astype(np.float32)
        current_rms = float(np.sqrt(np.mean(data**2)))
        data = data * (target_rms / current_rms)

        result = _rms_normalize(data)
        result_rms = _rms_dbfs(result)
        assert -21.0 < result_rms < -19.0

    def test_silence_not_amplified(self):
        data = np.zeros(16000, dtype=np.float32)
        result = _rms_normalize(data)
        assert np.all(result == 0)

    def test_near_silence_gain_capped(self):
        data = np.full(16000, 1e-5, dtype=np.float32)
        result = _rms_normalize(data)
        gain = float(np.max(np.abs(result))) / float(np.max(np.abs(data)))
        assert gain <= 20.0

    def test_peak_never_exceeds_one(self):
        rng = np.random.default_rng(42)
        data = rng.normal(0, 0.001, 16000).astype(np.float32)
        data[100] = 0.8
        result = _rms_normalize(data)
        assert float(np.max(np.abs(result))) <= 1.0

    def test_does_not_invert_phase(self):
        rng = np.random.default_rng(42)
        data = rng.normal(0, 0.01, 16000).astype(np.float32)
        result = _rms_normalize(data)
        correlation = float(np.corrcoef(data, result)[0, 1])
        assert correlation > 0.99


class TestLoadAndNormalise:
    def test_mono_16k_wav(self):
        """A -30 dBFS recording should be normalized to ~-20 dBFS."""
        rng = np.random.default_rng(42)
        target_rms = 10.0 ** (-30.0 / 20.0)
        data = rng.normal(0, 1.0, 16000).astype(np.float32)
        data = data * (target_rms / float(np.sqrt(np.mean(data**2))))
        with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as f:
            _write_wav(f.name, data)
            result, sr = _load_and_normalise(f.name)
        assert sr == 16000
        assert result.ndim == 1
        result_rms = _rms_dbfs(result)
        assert -22.0 < result_rms < -18.0

    def test_stereo_downmixed_and_normalized(self):
        rng = np.random.default_rng(42)
        target_rms = 10.0 ** (-30.0 / 20.0)
        left = rng.normal(0, 1.0, 16000).astype(np.float32)
        left = left * (target_rms / float(np.sqrt(np.mean(left**2))))
        right = rng.normal(0, 1.0, 16000).astype(np.float32)
        right = right * (target_rms / float(np.sqrt(np.mean(right**2))))
        stereo = np.column_stack([left, right]).flatten()
        with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as f:
            _write_wav(f.name, stereo, channels=2)
            result, sr = _load_and_normalise(f.name)
        assert sr == 16000
        assert result.ndim == 1
        assert len(result) == 16000
        result_rms = _rms_dbfs(result)
        assert -22.0 < result_rms < -18.0

    def test_48k_resampled(self):
        rng = np.random.default_rng(42)
        target_rms = 10.0 ** (-30.0 / 20.0)
        data = rng.normal(0, 1.0, 48000).astype(np.float32)
        data = data * (target_rms / float(np.sqrt(np.mean(data**2))))
        with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as f:
            _write_wav(f.name, data, sr=48000)
            result, sr = _load_and_normalise(f.name)
        assert sr == 16000
        assert 15900 < len(result) < 16100
        result_rms = _rms_dbfs(result)
        assert -22.0 < result_rms < -18.0

    def test_real_world_level_minus_30(self):
        """Simulates a typical scrib recording at -30 dBFS RMS."""
        rng = np.random.default_rng(42)
        target_rms = 10.0 ** (-30.0 / 20.0)
        data = rng.normal(0, 1.0, 16000 * 10).astype(np.float32)
        current_rms = float(np.sqrt(np.mean(data**2)))
        data = data * (target_rms / current_rms)

        with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as f:
            _write_wav(f.name, data)
            result, sr = _load_and_normalise(f.name)
        result_rms = _rms_dbfs(result)
        assert -22.0 < result_rms < -18.0


class TestReduceNoise:
    def test_reduces_stationary_noise(self):
        """Stationary hum (like HVAC) should be attenuated."""
        rng = np.random.default_rng(42)
        sr = 16000
        t = np.linspace(0, 2, sr * 2, endpoint=False)
        hum = 0.02 * np.sin(2 * np.pi * 120 * t).astype(np.float32)
        noise = 0.01 * rng.normal(0, 1, sr * 2).astype(np.float32)
        signal = hum + noise

        result = _reduce_noise(signal, sr)
        original_rms = float(np.sqrt(np.mean(signal**2)))
        result_rms = float(np.sqrt(np.mean(result**2)))
        assert result_rms < original_rms * 0.8

    def test_preserves_speech_frequencies(self):
        """Broadband non-stationary signal (speech-like) should be mostly preserved."""
        rng = np.random.default_rng(42)
        sr = 16000
        t = np.linspace(0, 2, sr * 2, endpoint=False)
        envelope = (0.5 + 0.5 * np.sin(2 * np.pi * 3 * t)).astype(np.float32)
        f0 = 200 + 600 * t / 2
        speech = 0.1 * envelope * np.sin(2 * np.pi * f0 * t).astype(np.float32)
        harmonics = 0.05 * envelope * np.sin(2 * np.pi * f0 * 2 * t).astype(np.float32)
        speech = speech + harmonics
        noise = 0.005 * rng.normal(0, 1, sr * 2).astype(np.float32)
        signal = speech + noise

        result = _reduce_noise(signal, sr)
        speech_rms = float(np.sqrt(np.mean(speech**2)))
        result_rms = float(np.sqrt(np.mean(result**2)))
        assert result_rms > speech_rms * 0.3

    def test_silence_stays_silent(self):
        """Near-silence input should remain near-silent, not amplify artifacts."""
        data = np.zeros(16000, dtype=np.float32)
        data[100] = 0.001
        result = _reduce_noise(data, 16000)
        assert float(np.max(np.abs(result))) < 0.01

    def test_output_shape_unchanged(self):
        rng = np.random.default_rng(42)
        data = rng.normal(0, 0.01, 32000).astype(np.float32)
        result = _reduce_noise(data, 16000)
        assert result.shape == data.shape

    def test_does_not_clip(self):
        rng = np.random.default_rng(42)
        data = rng.normal(0, 0.1, 16000).astype(np.float32)
        result = _reduce_noise(data, 16000)
        assert float(np.max(np.abs(result))) <= 1.0


class TestTrimSilence:
    def test_trims_leading_silence(self):
        sr = 16000
        silence = np.zeros(sr * 3, dtype=np.float32)
        speech = 0.1 * np.ones(sr * 2, dtype=np.float32)
        data = np.concatenate([silence, speech])

        result, offset = _trim_silence(data, sr)
        assert offset > 1.0
        assert len(result) < len(data)
        assert len(result) >= sr * 2

    def test_trims_trailing_silence(self):
        sr = 16000
        speech = 0.1 * np.ones(sr * 2, dtype=np.float32)
        silence = np.zeros(sr * 3, dtype=np.float32)
        data = np.concatenate([speech, silence])

        result, offset = _trim_silence(data, sr)
        assert offset == 0.0
        assert len(result) < len(data)
        assert len(result) >= sr * 2

    def test_trims_both_ends(self):
        sr = 16000
        silence = np.zeros(sr * 2, dtype=np.float32)
        speech = 0.1 * np.ones(sr * 3, dtype=np.float32)
        data = np.concatenate([silence, speech, silence])

        result, offset = _trim_silence(data, sr)
        assert offset > 0.5
        assert len(result) < len(data)

    def test_preserves_internal_silence(self):
        sr = 16000
        speech1 = 0.1 * np.ones(sr, dtype=np.float32)
        gap = np.zeros(sr * 2, dtype=np.float32)
        speech2 = 0.1 * np.ones(sr, dtype=np.float32)
        data = np.concatenate([speech1, gap, speech2])

        result, offset = _trim_silence(data, sr)
        assert offset == 0.0
        assert len(result) == len(data)

    def test_short_silence_not_trimmed(self):
        """Silence under 1s at edges should not be trimmed."""
        sr = 16000
        short_silence = np.zeros(int(sr * 0.5), dtype=np.float32)
        speech = 0.1 * np.ones(sr * 2, dtype=np.float32)
        data = np.concatenate([short_silence, speech])

        result, offset = _trim_silence(data, sr)
        assert offset == 0.0
        assert len(result) == len(data)

    def test_all_silence_returns_unchanged(self):
        sr = 16000
        data = np.zeros(sr * 5, dtype=np.float32)

        result, offset = _trim_silence(data, sr)
        assert offset == 0.0
        assert len(result) == len(data)

    def test_no_silence_unchanged(self):
        sr = 16000
        rng = np.random.default_rng(42)
        data = 0.1 * rng.normal(0, 1, sr * 3).astype(np.float32)

        result, offset = _trim_silence(data, sr)
        assert offset == 0.0
        assert len(result) == len(data)
