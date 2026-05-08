import numpy as np
from dataclasses import dataclass

from analytics.corners import Corner


@dataclass
class CornerScore:
    corner_idx: int
    consistency: float
    braking_stability: float
    apex_speed_stability: float
    exit_speed_stability: float
    braking_point_drift: float
    overall: float


def score_corners(
    laps_data: list[dict],
    corners_per_lap: list[list[Corner]],
) -> list[CornerScore]:
    if not corners_per_lap or len(corners_per_lap) < 2:
        return []

    ref_corners = corners_per_lap[0]
    num_corners = len(ref_corners)
    scores = []

    for c_idx in range(num_corners):
        entry_speeds = []
        apex_speeds = []
        exit_speeds = []
        brake_points = []

        for lap_corners in corners_per_lap:
            if c_idx >= len(lap_corners):
                continue
            c = lap_corners[c_idx]
            entry_speeds.append(c.entry_speed)
            apex_speeds.append(c.apex_speed)
            exit_speeds.append(c.exit_speed)
            brake_points.append(c.entry_idx)

        if len(entry_speeds) < 2:
            continue

        entry_arr = np.array(entry_speeds)
        apex_arr = np.array(apex_speeds)
        exit_arr = np.array(exit_speeds)
        brake_arr = np.array(brake_points, dtype=float)

        braking_stability = _cv_to_score(entry_arr)
        apex_stability = _cv_to_score(apex_arr)
        exit_stability = _cv_to_score(exit_arr)

        brake_drift = float(np.std(brake_arr))
        brake_drift_score = max(0.0, 1.0 - brake_drift / 30.0)

        consistency = (braking_stability + apex_stability + exit_stability) / 3.0
        overall = consistency * 0.6 + brake_drift_score * 0.4

        scores.append(CornerScore(
            corner_idx=c_idx,
            consistency=consistency,
            braking_stability=braking_stability,
            apex_speed_stability=apex_stability,
            exit_speed_stability=exit_stability,
            braking_point_drift=brake_drift,
            overall=overall,
        ))

    return scores


def _cv_to_score(arr: np.ndarray) -> float:
    mean = np.mean(arr)
    if mean == 0:
        return 1.0
    cv = float(np.std(arr) / mean)
    return max(0.0, 1.0 - cv * 10)


@dataclass
class ProgressionPoint:
    session_idx: int
    lap_time_ms: float
    consistency_score: float
    top_speed: float
    corner_scores_avg: float


def compute_progression(
    sessions: list[dict],
) -> list[ProgressionPoint]:
    points = []
    for i, session in enumerate(sessions):
        laps = session.get("laps", [])
        if not laps:
            continue

        times = [l["lap_time_ms"] for l in laps if l.get("lap_time_ms", 0) > 0]
        if not times:
            continue

        best_time = min(times)
        cv = float(np.std(times) / np.mean(times)) if len(times) > 1 else 0.0
        consistency = max(0.0, 1.0 - cv * 10)
        top_speed = max(l.get("top_speed", 0) for l in laps)
        corner_avg = session.get("corner_scores_avg", 0.0)

        points.append(ProgressionPoint(
            session_idx=i,
            lap_time_ms=best_time,
            consistency_score=consistency,
            top_speed=top_speed,
            corner_scores_avg=corner_avg,
        ))

    return points
