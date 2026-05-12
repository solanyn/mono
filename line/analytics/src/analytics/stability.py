import numpy as np
from dataclasses import dataclass
from scipy.signal import savgol_filter


@dataclass
class StabilityEvent:
    start_idx: int
    end_idx: int
    event_type: str
    severity: float
    yaw_rate: float
    steering_angle: float
    speed: float


@dataclass
class StabilityAnalysis:
    events: list[StabilityEvent]
    oversteer_count: int
    understeer_count: int
    avg_yaw_deviation: float
    stability_score: float
    worst_corner_idx: int


def compute_yaw_rate(x: np.ndarray, z: np.ndarray, fps: float = 60.0) -> np.ndarray:
    heading = np.arctan2(np.gradient(z), np.gradient(x))
    heading_unwrapped = np.unwrap(heading)
    yaw_rate = np.gradient(heading_unwrapped) * fps
    return yaw_rate


def compute_expected_yaw(
    speed: np.ndarray,
    steering: np.ndarray,
    wheelbase: float = 2.5,
) -> np.ndarray:
    speed_ms = speed / 3.6
    steering_rad = steering * np.pi / 180.0
    speed_ms = np.where(speed_ms < 1.0, 1.0, speed_ms)
    expected = speed_ms * np.tan(steering_rad) / wheelbase
    return expected


def detect_stability_events(
    x: np.ndarray,
    z: np.ndarray,
    speed: np.ndarray,
    steering: np.ndarray,
    fps: float = 60.0,
    oversteer_threshold: float = 0.3,
    understeer_threshold: float = 0.3,
    min_duration: int = 5,
) -> list[StabilityEvent]:
    actual_yaw = compute_yaw_rate(x, z, fps)
    expected_yaw = compute_expected_yaw(speed, steering)

    if len(actual_yaw) > 15:
        actual_yaw = savgol_filter(actual_yaw, 15, 3)

    deviation = actual_yaw - expected_yaw

    events = []
    i = 0
    n = len(deviation)

    while i < n:
        if abs(deviation[i]) > oversteer_threshold:
            start = i
            event_type = "oversteer" if abs(actual_yaw[i]) > abs(expected_yaw[i]) else "understeer"
            while i < n and abs(deviation[i]) > oversteer_threshold * 0.5:
                i += 1
            end = i

            if (end - start) >= min_duration:
                severity = float(np.max(np.abs(deviation[start:end])))
                events.append(StabilityEvent(
                    start_idx=start,
                    end_idx=end,
                    event_type=event_type,
                    severity=severity,
                    yaw_rate=float(np.mean(actual_yaw[start:end])),
                    steering_angle=float(np.mean(steering[start:end])),
                    speed=float(np.mean(speed[start:end])),
                ))
        else:
            i += 1

    return events


def analyze_stability(
    x: np.ndarray,
    z: np.ndarray,
    speed: np.ndarray,
    steering: np.ndarray,
    fps: float = 60.0,
) -> StabilityAnalysis:
    events = detect_stability_events(x, z, speed, steering, fps)

    oversteer = [e for e in events if e.event_type == "oversteer"]
    understeer = [e for e in events if e.event_type == "understeer"]

    actual_yaw = compute_yaw_rate(x, z, fps)
    expected_yaw = compute_expected_yaw(speed, steering)
    avg_deviation = float(np.mean(np.abs(actual_yaw - expected_yaw)))

    stability_score = float(max(0.0, 1.0 - len(events) * 0.05 - avg_deviation))

    worst_idx = -1
    if events:
        worst = max(events, key=lambda e: e.severity)
        worst_idx = worst.start_idx

    return StabilityAnalysis(
        events=events,
        oversteer_count=len(oversteer),
        understeer_count=len(understeer),
        avg_yaw_deviation=avg_deviation,
        stability_score=stability_score,
        worst_corner_idx=worst_idx,
    )
