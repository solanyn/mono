import numpy as np
from dataclasses import dataclass
from scipy import stats


@dataclass
class FatigueAnalysis:
    driver_fatigue_score: float
    tyre_degradation_score: float
    lap_time_trend: float
    consistency_trend: float
    brake_point_drift_trend: float
    speed_loss_trend: float
    separation_confidence: float
    diagnosis: str


def analyze_fatigue(
    lap_times_ms: list[int],
    lap_top_speeds: list[float],
    lap_brake_counts: list[int],
    lap_avg_tire_temps: list[float],
    lap_brake_point_drifts: list[float] | None = None,
) -> FatigueAnalysis:
    n = len(lap_times_ms)
    if n < 4:
        return FatigueAnalysis(
            driver_fatigue_score=0.0, tyre_degradation_score=0.0,
            lap_time_trend=0.0, consistency_trend=0.0,
            brake_point_drift_trend=0.0, speed_loss_trend=0.0,
            separation_confidence=0.0, diagnosis="insufficient_data",
        )

    times = np.array(lap_times_ms, dtype=float)
    speeds = np.array(lap_top_speeds, dtype=float)
    temps = np.array(lap_avg_tire_temps, dtype=float)
    x_laps = np.arange(n)

    time_slope, _, _, _, _ = stats.linregress(x_laps, times)
    speed_slope, _, _, _, _ = stats.linregress(x_laps, speeds)
    temp_slope, _, _, _, _ = stats.linregress(x_laps, temps)

    half = n // 2
    first_half_cv = float(np.std(times[:half]) / np.mean(times[:half])) if np.mean(times[:half]) > 0 else 0
    second_half_cv = float(np.std(times[half:]) / np.mean(times[half:])) if np.mean(times[half:]) > 0 else 0
    consistency_trend = second_half_cv - first_half_cv

    brake_drift_trend = 0.0
    if lap_brake_point_drifts and len(lap_brake_point_drifts) >= 4:
        drifts = np.array(lap_brake_point_drifts, dtype=float)
        brake_drift_trend, _, _, _, _ = stats.linregress(x_laps[:len(drifts)], drifts)

    tyre_signal = 0.0
    if temp_slope > 0.5:
        tyre_signal += 0.3
    if speed_slope < -0.1:
        tyre_signal += 0.3
    if time_slope > 50:
        tyre_signal += 0.2

    driver_signal = 0.0
    if consistency_trend > 0.01:
        driver_signal += 0.4
    if brake_drift_trend > 0.5:
        driver_signal += 0.3
    if time_slope > 50 and temp_slope < 0.2:
        driver_signal += 0.3

    total_signal = tyre_signal + driver_signal
    if total_signal > 0:
        tyre_pct = tyre_signal / total_signal
        driver_pct = driver_signal / total_signal
    else:
        tyre_pct = 0.0
        driver_pct = 0.0

    confidence = min(1.0, total_signal)

    if total_signal < 0.2:
        diagnosis = "stable"
    elif driver_pct > 0.6:
        diagnosis = "driver_fatigue"
    elif tyre_pct > 0.6:
        diagnosis = "tyre_degradation"
    else:
        diagnosis = "mixed"

    return FatigueAnalysis(
        driver_fatigue_score=float(min(1.0, driver_signal)),
        tyre_degradation_score=float(min(1.0, tyre_signal)),
        lap_time_trend=float(time_slope),
        consistency_trend=float(consistency_trend),
        brake_point_drift_trend=float(brake_drift_trend),
        speed_loss_trend=float(speed_slope),
        separation_confidence=float(confidence),
        diagnosis=diagnosis,
    )
