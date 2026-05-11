import numpy as np
from dataclasses import dataclass
from scipy.signal import find_peaks


@dataclass
class BrakingEvent:
    start_idx: int
    end_idx: int
    start_speed: float
    end_speed: float
    deceleration_g: float
    duration_s: float
    distance_m: float
    trail_brake_pct: float
    release_smoothness: float
    efficiency: float


@dataclass
class BrakingAnalysis:
    events: list[BrakingEvent]
    avg_deceleration_g: float
    avg_trail_brake_pct: float
    avg_release_smoothness: float
    avg_efficiency: float
    consistency_score: float
    total_brake_distance_m: float


def detect_braking_events(
    speed: np.ndarray,
    brake: np.ndarray,
    x: np.ndarray,
    z: np.ndarray,
    fps: float = 60.0,
    brake_threshold: float = 10.0,
    min_duration_frames: int = 10,
) -> list[BrakingEvent]:
    in_brake = brake > brake_threshold
    events = []
    i = 0
    n = len(in_brake)

    while i < n:
        if in_brake[i]:
            start = i
            while i < n and in_brake[i]:
                i += 1
            end = i

            if (end - start) < min_duration_frames:
                continue

            dx = np.diff(x[start:end + 1])
            dz = np.diff(z[start:end + 1])
            distance = float(np.sum(np.sqrt(dx**2 + dz**2)))

            duration = (end - start) / fps
            start_speed = float(speed[start])
            end_speed = float(speed[end - 1])
            delta_v = (start_speed - end_speed) / 3.6
            decel_g = delta_v / (duration * 9.81) if duration > 0 else 0.0

            brake_segment = brake[start:end]
            peak_idx = np.argmax(brake_segment)
            trail_portion = brake_segment[peak_idx:]
            if len(trail_portion) > 1:
                trail_brake_pct = float(np.sum(trail_portion < brake_segment[peak_idx] * 0.9) / len(trail_portion) * 100)
            else:
                trail_brake_pct = 0.0

            release_segment = brake_segment[peak_idx:]
            if len(release_segment) > 2:
                diffs = np.abs(np.diff(release_segment))
                max_diff = np.max(diffs) if len(diffs) > 0 else 1.0
                release_smoothness = float(max(0.0, 1.0 - np.std(diffs) / (max_diff + 1e-6)))
            else:
                release_smoothness = 1.0

            ideal_distance = (start_speed / 3.6)**2 / (2 * 9.81 * 1.2) if start_speed > 0 else 1.0
            efficiency = float(min(1.0, ideal_distance / (distance + 1e-6)))

            events.append(BrakingEvent(
                start_idx=start,
                end_idx=end,
                start_speed=start_speed,
                end_speed=end_speed,
                deceleration_g=decel_g,
                duration_s=duration,
                distance_m=distance,
                trail_brake_pct=trail_brake_pct,
                release_smoothness=release_smoothness,
                efficiency=efficiency,
            ))
        else:
            i += 1

    return events


def analyze_braking(
    speed: np.ndarray,
    brake: np.ndarray,
    x: np.ndarray,
    z: np.ndarray,
    fps: float = 60.0,
) -> BrakingAnalysis:
    events = detect_braking_events(speed, brake, x, z, fps)

    if not events:
        return BrakingAnalysis(
            events=[], avg_deceleration_g=0, avg_trail_brake_pct=0,
            avg_release_smoothness=0, avg_efficiency=0, consistency_score=1.0,
            total_brake_distance_m=0,
        )

    decels = [e.deceleration_g for e in events]
    trails = [e.trail_brake_pct for e in events]
    smooths = [e.release_smoothness for e in events]
    effs = [e.efficiency for e in events]
    dists = [e.distance_m for e in events]

    decel_cv = np.std(decels) / (np.mean(decels) + 1e-6)
    consistency = float(max(0.0, 1.0 - decel_cv))

    return BrakingAnalysis(
        events=events,
        avg_deceleration_g=float(np.mean(decels)),
        avg_trail_brake_pct=float(np.mean(trails)),
        avg_release_smoothness=float(np.mean(smooths)),
        avg_efficiency=float(np.mean(effs)),
        consistency_score=consistency,
        total_brake_distance_m=float(np.sum(dists)),
    )


def compare_braking_points(
    laps_brake_events: list[list[BrakingEvent]],
    tolerance_m: float = 20.0,
) -> list[dict]:
    if len(laps_brake_events) < 2:
        return []

    ref_events = laps_brake_events[0]
    results = []

    for i, ref_event in enumerate(ref_events):
        points = [ref_event.start_idx]
        for lap_events in laps_brake_events[1:]:
            closest = min(lap_events, key=lambda e: abs(e.start_idx - ref_event.start_idx), default=None)
            if closest:
                points.append(closest.start_idx)

        drift = float(np.std(points))
        results.append({
            "corner_idx": i,
            "ref_start_idx": ref_event.start_idx,
            "brake_point_drift_frames": drift,
            "consistent": drift < tolerance_m,
        })

    return results
