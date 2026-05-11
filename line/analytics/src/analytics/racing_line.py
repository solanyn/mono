import numpy as np
from dataclasses import dataclass

from analytics.alignment import compute_distance


@dataclass
class RacingLineScore:
    consistency: float
    smoothness: float
    deviation_avg_m: float
    deviation_max_m: float
    worst_sections: list[dict]


def compute_racing_line_deviation(
    laps_x: list[np.ndarray],
    laps_z: list[np.ndarray],
    num_points: int = 1000,
) -> RacingLineScore:
    if len(laps_x) < 2:
        return RacingLineScore(1.0, 1.0, 0.0, 0.0, [])

    resampled = []
    for x, z in zip(laps_x, laps_z):
        dist = compute_distance(x, z)
        uniform = np.linspace(0, dist[-1], num_points)
        rx = np.interp(uniform, dist, x)
        rz = np.interp(uniform, dist, z)
        resampled.append(np.column_stack([rx, rz]))

    positions = np.array(resampled)
    mean_line = positions.mean(axis=0)

    deviations = np.linalg.norm(positions - mean_line[np.newaxis, :, :], axis=2)
    avg_deviation = float(np.mean(deviations))
    max_deviation = float(np.max(deviations))

    per_point_std = np.std(deviations, axis=0)
    max_std = np.max(per_point_std) if len(per_point_std) > 0 else 1.0
    consistency = float(max(0.0, 1.0 - np.mean(per_point_std) / 5.0))

    curvature_changes = []
    for lap_pts in resampled:
        dx = np.gradient(lap_pts[:, 0])
        dz = np.gradient(lap_pts[:, 1])
        ddx = np.gradient(dx)
        ddz = np.gradient(dz)
        jerk = np.sqrt(ddx**2 + ddz**2)
        curvature_changes.append(jerk)

    avg_jerk = np.mean([np.mean(j) for j in curvature_changes])
    smoothness = float(max(0.0, 1.0 - avg_jerk * 100))

    worst_sections = []
    window = num_points // 20
    for i in range(0, num_points - window, window):
        section_std = float(np.mean(per_point_std[i:i + window]))
        if section_std > avg_deviation:
            worst_sections.append({
                "start_pct": i / num_points * 100,
                "end_pct": (i + window) / num_points * 100,
                "deviation_m": section_std,
            })
    worst_sections.sort(key=lambda s: s["deviation_m"], reverse=True)

    return RacingLineScore(
        consistency=consistency,
        smoothness=smoothness,
        deviation_avg_m=avg_deviation,
        deviation_max_m=max_deviation,
        worst_sections=worst_sections[:5],
    )


def compute_optimal_line(
    laps_x: list[np.ndarray],
    laps_z: list[np.ndarray],
    laps_speed: list[np.ndarray],
    num_points: int = 1000,
) -> dict:
    if not laps_x:
        return {"x": [], "z": [], "distance": []}

    resampled_x = []
    resampled_z = []
    resampled_speed = []

    for x, z, speed in zip(laps_x, laps_z, laps_speed):
        dist = compute_distance(x, z)
        uniform = np.linspace(0, dist[-1], num_points)
        resampled_x.append(np.interp(uniform, dist, x))
        resampled_z.append(np.interp(uniform, dist, z))
        resampled_speed.append(np.interp(uniform, dist, speed))

    speeds = np.array(resampled_speed)
    weights = speeds / speeds.sum(axis=0, keepdims=True)

    opt_x = np.sum(np.array(resampled_x) * weights, axis=0)
    opt_z = np.sum(np.array(resampled_z) * weights, axis=0)
    opt_dist = compute_distance(opt_x, opt_z)

    return {
        "x": opt_x.tolist(),
        "z": opt_z.tolist(),
        "distance": opt_dist.tolist(),
    }
