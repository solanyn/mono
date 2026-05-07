import numpy as np
from scipy.signal import find_peaks, savgol_filter
from scipy.ndimage import uniform_filter1d
from dataclasses import dataclass


@dataclass
class Corner:
    entry_idx: int
    apex_idx: int
    exit_idx: int
    entry_speed: float
    apex_speed: float
    exit_speed: float
    direction: str
    curvature: float
    length_m: float


@dataclass
class Sector:
    start_idx: int
    end_idx: int
    corners: list[Corner]
    length_m: float
    sector_type: str


def compute_curvature(x: np.ndarray, z: np.ndarray, window: int = 15) -> np.ndarray:
    if len(x) < window:
        return np.zeros(len(x))
    dx = savgol_filter(x, window, 3, deriv=1)
    dz = savgol_filter(z, window, 3, deriv=1)
    ddx = savgol_filter(x, window, 3, deriv=2)
    ddz = savgol_filter(z, window, 3, deriv=2)
    num = np.abs(dx * ddz - dz * ddx)
    denom = (dx**2 + dz**2) ** 1.5
    denom = np.where(denom < 1e-10, 1e-10, denom)
    return num / denom


def compute_heading(x: np.ndarray, z: np.ndarray) -> np.ndarray:
    dx = np.gradient(x)
    dz = np.gradient(z)
    return np.arctan2(dz, dx)


def detect_corners(
    x: np.ndarray,
    z: np.ndarray,
    speed: np.ndarray,
    curvature_threshold: float = 0.005,
    min_corner_frames: int = 30,
    merge_gap: int = 20,
) -> list[Corner]:
    curvature = compute_curvature(x, z)
    curvature_smooth = uniform_filter1d(curvature, size=10)
    heading = compute_heading(x, z)

    in_corner = curvature_smooth > curvature_threshold
    corners = []
    i = 0
    n = len(in_corner)

    while i < n:
        if in_corner[i]:
            start = i
            while i < n and in_corner[i]:
                i += 1
            end = i

            while i < n and (i - end) < merge_gap:
                if in_corner[i]:
                    while i < n and in_corner[i]:
                        i += 1
                    end = i
                else:
                    i += 1
            if not in_corner[min(i - 1, n - 1)]:
                end_search = i
                for j in range(end, end_search):
                    if in_corner[j]:
                        while j < n and in_corner[j]:
                            j += 1
                        end = j
                        i = j
                        break

            if (end - start) >= min_corner_frames:
                apex_idx = start + int(np.argmin(speed[start:end]))
                heading_delta = heading[end - 1] - heading[start]
                heading_delta = (heading_delta + np.pi) % (2 * np.pi) - np.pi
                direction = "right" if heading_delta < 0 else "left"

                dx = np.diff(x[start:end])
                dz = np.diff(z[start:end])
                length = float(np.sum(np.sqrt(dx**2 + dz**2)))

                corners.append(Corner(
                    entry_idx=start,
                    apex_idx=apex_idx,
                    exit_idx=end - 1,
                    entry_speed=float(speed[start]),
                    apex_speed=float(speed[apex_idx]),
                    exit_speed=float(speed[end - 1]),
                    direction=direction,
                    curvature=float(np.mean(curvature_smooth[start:end])),
                    length_m=length,
                ))
        else:
            i += 1

    return corners


def detect_sectors(
    x: np.ndarray,
    z: np.ndarray,
    speed: np.ndarray,
    corners: list[Corner],
    min_straight_length: float = 50.0,
) -> list[Sector]:
    if not corners:
        dx = np.diff(x)
        dz = np.diff(z)
        total = float(np.sum(np.sqrt(dx**2 + dz**2)))
        return [Sector(start_idx=0, end_idx=len(x) - 1, corners=[], length_m=total, sector_type="straight")]

    sectors = []

    if corners[0].entry_idx > 0:
        start = 0
        end = corners[0].entry_idx
        dx = np.diff(x[start:end + 1])
        dz = np.diff(z[start:end + 1])
        length = float(np.sum(np.sqrt(dx**2 + dz**2)))
        if length >= min_straight_length:
            sectors.append(Sector(start_idx=start, end_idx=end, corners=[], length_m=length, sector_type="straight"))

    group_start = 0
    for i in range(1, len(corners)):
        gap_start = corners[i - 1].exit_idx
        gap_end = corners[i].entry_idx
        dx = np.diff(x[gap_start:gap_end + 1])
        dz = np.diff(z[gap_start:gap_end + 1])
        gap_length = float(np.sum(np.sqrt(dx**2 + dz**2)))

        if gap_length >= min_straight_length:
            group = corners[group_start:i]
            s_start = group[0].entry_idx
            s_end = group[-1].exit_idx
            dx2 = np.diff(x[s_start:s_end + 1])
            dz2 = np.diff(z[s_start:s_end + 1])
            s_length = float(np.sum(np.sqrt(dx2**2 + dz2**2)))
            sectors.append(Sector(start_idx=s_start, end_idx=s_end, corners=group, length_m=s_length, sector_type="corner_complex"))

            sectors.append(Sector(start_idx=gap_start, end_idx=gap_end, corners=[], length_m=gap_length, sector_type="straight"))
            group_start = i

    group = corners[group_start:]
    s_start = group[0].entry_idx
    s_end = group[-1].exit_idx
    dx2 = np.diff(x[s_start:s_end + 1])
    dz2 = np.diff(z[s_start:s_end + 1])
    s_length = float(np.sum(np.sqrt(dx2**2 + dz2**2)))
    sectors.append(Sector(start_idx=s_start, end_idx=s_end, corners=group, length_m=s_length, sector_type="corner_complex"))

    if corners[-1].exit_idx < len(x) - 1:
        start = corners[-1].exit_idx
        end = len(x) - 1
        dx = np.diff(x[start:end + 1])
        dz = np.diff(z[start:end + 1])
        length = float(np.sum(np.sqrt(dx**2 + dz**2)))
        if length >= min_straight_length:
            sectors.append(Sector(start_idx=start, end_idx=end, corners=[], length_m=length, sector_type="straight"))

    return sectors
