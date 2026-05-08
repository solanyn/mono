from dataclasses import dataclass, asdict

from analytics.gold import compute_consistency
from analytics.corners import Corner
from analytics.mastery import score_corners


@dataclass
class SessionJournal:
    session_id: str
    track_name: str
    car_code: int
    total_laps: int
    best_lap_ms: float
    worst_lap_ms: float
    consistency_score: float
    highlights: list[str]
    areas_to_improve: list[str]
    corner_notes: list[str]
    summary: str


def generate_journal(
    session_id: str,
    track_name: str,
    car_code: int,
    lap_metrics: list[dict],
    corners_per_lap: list[list[Corner]] | None = None,
) -> SessionJournal:
    if not lap_metrics:
        return SessionJournal(
            session_id=session_id, track_name=track_name, car_code=car_code,
            total_laps=0, best_lap_ms=0, worst_lap_ms=0, consistency_score=0,
            highlights=[], areas_to_improve=[], corner_notes=[], summary="No laps recorded.",
        )

    times = [m["lap_time_ms"] for m in lap_metrics if m.get("lap_time_ms", 0) > 0]
    best = min(times) if times else 0
    worst = max(times) if times else 0

    consistency = compute_consistency(lap_metrics)
    score = consistency.get("consistency_score", 0)

    highlights = []
    areas = []
    corner_notes = []

    if len(times) >= 3:
        first_half = times[: len(times) // 2]
        second_half = times[len(times) // 2 :]
        if min(second_half) < min(first_half):
            highlights.append(f"Improved by {min(first_half) - min(second_half):.0f}ms in second half of session")
        if max(second_half) > max(first_half) * 1.02:
            areas.append("Lap times degraded in second half — possible tyre or concentration drop")

    if score > 0.95:
        highlights.append("Excellent consistency — lap times very tight")
    elif score < 0.8:
        areas.append(f"Consistency needs work (score: {score:.2f}) — focus on repeatable braking points")

    top_speeds = [m.get("top_speed", 0) for m in lap_metrics]
    if top_speeds:
        speed_range = max(top_speeds) - min(top_speeds)
        if speed_range < 5:
            highlights.append("Very consistent top speed — good throttle discipline")
        elif speed_range > 15:
            areas.append(f"Top speed varies by {speed_range:.0f} km/h — check exit speed from preceding corner")

    if corners_per_lap and len(corners_per_lap) >= 2:
        corner_scores = score_corners(lap_metrics, corners_per_lap)
        for cs in corner_scores:
            if cs.overall < 0.6:
                corner_notes.append(f"Corner {cs.corner_idx + 1}: needs work (score {cs.overall:.2f}, brake drift {cs.braking_point_drift:.1f} frames)")
            elif cs.overall > 0.9:
                highlights.append(f"Corner {cs.corner_idx + 1}: mastered (score {cs.overall:.2f})")

    parts = []
    parts.append(f"Session at {track_name}: {len(times)} laps, best {best/1000:.3f}s")
    if highlights:
        parts.append("Strengths: " + "; ".join(highlights[:3]))
    if areas:
        parts.append("Focus areas: " + "; ".join(areas[:3]))
    summary = ". ".join(parts)

    return SessionJournal(
        session_id=session_id,
        track_name=track_name,
        car_code=car_code,
        total_laps=len(times),
        best_lap_ms=best,
        worst_lap_ms=worst,
        consistency_score=score,
        highlights=highlights,
        areas_to_improve=areas,
        corner_notes=corner_notes,
        summary=summary,
    )
