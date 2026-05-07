import numpy as np
from scipy import stats


def compute_consistency(lap_metrics: list[dict]) -> dict:
    if len(lap_metrics) < 2:
        return {"consistency_score": 1.0, "lap_count": len(lap_metrics)}

    times = np.array([m.get("lap_time_ms", 0) for m in lap_metrics], dtype=float)
    times = times[times > 0]
    if len(times) < 2:
        return {"consistency_score": 1.0, "lap_count": len(lap_metrics)}

    cv = float(stats.variation(times))
    consistency_score = max(0.0, 1.0 - cv * 10)

    speeds = np.array([m["top_speed"] for m in lap_metrics])
    speed_cv = float(stats.variation(speeds))

    brake_counts = np.array([m["brake_count"] for m in lap_metrics], dtype=float)
    brake_cv = float(stats.variation(brake_counts)) if np.mean(brake_counts) > 0 else 0.0

    best_idx = int(np.argmin(times))
    worst_idx = int(np.argmax(times))
    delta_ms = float(times[worst_idx] - times[best_idx])

    return {
        "consistency_score": consistency_score,
        "lap_time_cv": cv,
        "speed_cv": speed_cv,
        "brake_count_cv": brake_cv,
        "best_lap_idx": best_idx,
        "worst_lap_idx": worst_idx,
        "best_worst_delta_ms": delta_ms,
        "lap_count": len(times),
    }
