import numpy as np
from dataclasses import dataclass


@dataclass
class AlignedLap:
    distance: np.ndarray
    speed: np.ndarray
    throttle: np.ndarray
    brake: np.ndarray
    x: np.ndarray
    z: np.ndarray
    time_s: np.ndarray


@dataclass
class TimeDelta:
    distance: np.ndarray
    delta_s: np.ndarray
    total_delta_s: float
    ahead_pct: float
    max_gain_m: float
    max_loss_m: float


def compute_distance(x: np.ndarray, z: np.ndarray) -> np.ndarray:
    dx = np.diff(x, prepend=x[0])
    dz = np.diff(z, prepend=z[0])
    segment_lengths = np.sqrt(dx**2 + dz**2)
    return np.cumsum(segment_lengths)


def resample_by_distance(
    distance: np.ndarray,
    values: dict[str, np.ndarray],
    num_points: int = 1000,
) -> tuple[np.ndarray, dict[str, np.ndarray]]:
    uniform_dist = np.linspace(0, distance[-1], num_points)
    resampled = {}
    for key, arr in values.items():
        resampled[key] = np.interp(uniform_dist, distance, arr)
    return uniform_dist, resampled


def align_lap(
    x: np.ndarray,
    z: np.ndarray,
    speed: np.ndarray,
    throttle: np.ndarray,
    brake: np.ndarray,
    time_s: np.ndarray,
    num_points: int = 1000,
) -> AlignedLap:
    distance = compute_distance(x, z)
    uniform_dist, resampled = resample_by_distance(
        distance,
        {"speed": speed, "throttle": throttle, "brake": brake, "x": x, "z": z, "time_s": time_s},
        num_points,
    )
    return AlignedLap(
        distance=uniform_dist,
        speed=resampled["speed"],
        throttle=resampled["throttle"],
        brake=resampled["brake"],
        x=resampled["x"],
        z=resampled["z"],
        time_s=resampled["time_s"],
    )


def compute_time_delta(reference: AlignedLap, comparison: AlignedLap) -> TimeDelta:
    ref_time = reference.time_s
    cmp_time = comparison.time_s

    delta = cmp_time - ref_time

    total_delta = float(delta[-1])
    ahead = np.sum(delta < 0) / len(delta) * 100.0

    gains = np.diff(delta)
    gain_distances = np.where(gains < 0)[0]
    loss_distances = np.where(gains > 0)[0]

    max_gain = float(reference.distance[gain_distances[-1]]) if len(gain_distances) > 0 else 0.0
    max_loss = float(reference.distance[loss_distances[-1]]) if len(loss_distances) > 0 else 0.0

    return TimeDelta(
        distance=reference.distance,
        delta_s=delta,
        total_delta_s=total_delta,
        ahead_pct=float(ahead),
        max_gain_m=max_gain,
        max_loss_m=max_loss,
    )


def find_biggest_gains(reference: AlignedLap, comparison: AlignedLap, window: int = 50) -> list[dict]:
    delta = comparison.time_s - reference.time_s
    gains = []
    for i in range(0, len(delta) - window, window // 2):
        segment_delta = delta[i + window] - delta[i]
        gains.append({
            "start_m": float(reference.distance[i]),
            "end_m": float(reference.distance[min(i + window, len(reference.distance) - 1)]),
            "delta_s": float(segment_delta),
            "avg_speed_ref": float(np.mean(reference.speed[i:i + window])),
            "avg_speed_cmp": float(np.mean(comparison.speed[i:i + window])),
        })
    gains.sort(key=lambda g: g["delta_s"])
    return gains[:5]
