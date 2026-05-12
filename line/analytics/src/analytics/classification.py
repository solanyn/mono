import numpy as np
from dataclasses import dataclass

from analytics.corners import Corner, compute_curvature


@dataclass
class ClassifiedCorner:
    corner: Corner
    classification: str
    radius_m: float
    angle_deg: float
    is_complex: bool


CORNER_TYPES = {
    "hairpin": {"min_angle": 120, "max_radius": 30},
    "tight": {"min_angle": 70, "max_radius": 60},
    "medium": {"min_angle": 40, "max_radius": 120},
    "sweeper": {"min_angle": 20, "max_radius": 300},
    "kink": {"min_angle": 0, "max_radius": 1000},
}


def compute_corner_angle(x: np.ndarray, z: np.ndarray, entry_idx: int, exit_idx: int) -> float:
    entry_heading = np.arctan2(
        z[min(entry_idx + 5, len(z) - 1)] - z[entry_idx],
        x[min(entry_idx + 5, len(x) - 1)] - x[entry_idx],
    )
    exit_heading = np.arctan2(
        z[exit_idx] - z[max(exit_idx - 5, 0)],
        x[exit_idx] - x[max(exit_idx - 5, 0)],
    )
    angle = abs(exit_heading - entry_heading)
    angle = (angle + np.pi) % (2 * np.pi) - np.pi
    return float(np.degrees(abs(angle)))


def compute_corner_radius(x: np.ndarray, z: np.ndarray, entry_idx: int, exit_idx: int) -> float:
    segment_x = x[entry_idx:exit_idx + 1]
    segment_z = z[entry_idx:exit_idx + 1]
    if len(segment_x) < 3:
        return 1000.0
    curvature = compute_curvature(segment_x, segment_z, min(15, len(segment_x) - 1 if len(segment_x) % 2 == 0 else len(segment_x)))
    mean_curv = np.mean(curvature)
    if mean_curv < 1e-6:
        return 1000.0
    return float(1.0 / mean_curv)


def classify_corner(
    corner: Corner,
    x: np.ndarray,
    z: np.ndarray,
) -> ClassifiedCorner:
    angle = compute_corner_angle(x, z, corner.entry_idx, corner.exit_idx)
    radius = compute_corner_radius(x, z, corner.entry_idx, corner.exit_idx)

    classification = "kink"
    for ctype, params in CORNER_TYPES.items():
        if angle >= params["min_angle"] and radius <= params["max_radius"]:
            classification = ctype
            break

    is_complex = corner.length_m > 100 and angle > 90

    return ClassifiedCorner(
        corner=corner,
        classification=classification,
        radius_m=radius,
        angle_deg=angle,
        is_complex=is_complex,
    )


def classify_corners(
    corners: list[Corner],
    x: np.ndarray,
    z: np.ndarray,
) -> list[ClassifiedCorner]:
    return [classify_corner(c, x, z) for c in corners]


def detect_chicane(classified: list[ClassifiedCorner], max_gap_frames: int = 60) -> list[tuple[int, int]]:
    chicanes = []
    i = 0
    while i < len(classified) - 1:
        c1 = classified[i]
        c2 = classified[i + 1]
        gap = c2.corner.entry_idx - c1.corner.exit_idx
        if gap < max_gap_frames and c1.corner.direction != c2.corner.direction:
            chicanes.append((i, i + 1))
            i += 2
        else:
            i += 1
    return chicanes
