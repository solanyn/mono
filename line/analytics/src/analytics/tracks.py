import json
import hashlib
from dataclasses import dataclass, field, asdict
from pathlib import Path

import numpy as np


@dataclass
class TrackCornerInfo:
    number: int
    name: str
    direction: str
    reference_apex_x: float
    reference_apex_z: float
    notes: str = ""


@dataclass
class TrackInfo:
    track_id: str
    name: str
    country: str
    length_m: float
    corners: list[TrackCornerInfo] = field(default_factory=list)
    reference_line_x: list[float] = field(default_factory=list)
    reference_line_z: list[float] = field(default_factory=list)
    source: str = "community"


def fingerprint_track(x: np.ndarray, z: np.ndarray, num_points: int = 64) -> str:
    indices = np.linspace(0, len(x) - 1, num_points, dtype=int)
    sampled_x = x[indices]
    sampled_z = z[indices]

    cx = np.mean(sampled_x)
    cz = np.mean(sampled_z)
    sampled_x = sampled_x - cx
    sampled_z = sampled_z - cz

    max_dist = np.max(np.sqrt(sampled_x**2 + sampled_z**2))
    if max_dist > 0:
        sampled_x = sampled_x / max_dist
        sampled_z = sampled_z / max_dist

    data = np.column_stack([sampled_x, sampled_z])
    quantized = np.round(data, decimals=2)
    raw = quantized.tobytes()
    return hashlib.sha256(raw).hexdigest()[:16]


def match_track(
    x: np.ndarray,
    z: np.ndarray,
    track_db: dict[str, TrackInfo],
    threshold: float = 0.15,
) -> TrackInfo | None:
    if not track_db:
        return None

    query_fp = fingerprint_track(x, z)

    for track_id, info in track_db.items():
        if not info.reference_line_x:
            continue
        ref_x = np.array(info.reference_line_x)
        ref_z = np.array(info.reference_line_z)
        ref_fp = fingerprint_track(ref_x, ref_z)
        if ref_fp == query_fp:
            return info

    best_match = None
    best_score = float("inf")

    query_norm = _normalize_track(x, z)

    for track_id, info in track_db.items():
        if not info.reference_line_x:
            continue
        ref_x = np.array(info.reference_line_x)
        ref_z = np.array(info.reference_line_z)
        ref_norm = _normalize_track(ref_x, ref_z)

        score = _hausdorff_approx(query_norm, ref_norm)
        if score < best_score:
            best_score = score
            best_match = info

    if best_score < threshold:
        return best_match
    return None


def _normalize_track(x: np.ndarray, z: np.ndarray, num_points: int = 128) -> np.ndarray:
    indices = np.linspace(0, len(x) - 1, num_points, dtype=int)
    pts = np.column_stack([x[indices], z[indices]])
    center = pts.mean(axis=0)
    pts = pts - center
    scale = np.max(np.linalg.norm(pts, axis=1))
    if scale > 0:
        pts = pts / scale
    return pts


def _hausdorff_approx(a: np.ndarray, b: np.ndarray) -> float:
    from scipy.spatial.distance import cdist
    d = cdist(a, b)
    forward = np.mean(np.min(d, axis=1))
    backward = np.mean(np.min(d, axis=0))
    return float(max(forward, backward))


class TrackDatabase:
    def __init__(self, data_dir: str | Path | None = None):
        self._tracks: dict[str, TrackInfo] = {}
        if data_dir:
            self._data_dir = Path(data_dir)
            self._load()
        else:
            self._data_dir = None

    def _load(self):
        if not self._data_dir or not self._data_dir.exists():
            return
        for f in self._data_dir.glob("*.json"):
            with open(f) as fp:
                data = json.load(fp)
            corners = [TrackCornerInfo(**c) for c in data.pop("corners", [])]
            info = TrackInfo(**data, corners=corners)
            self._tracks[info.track_id] = info

    def save(self, track: TrackInfo):
        self._tracks[track.track_id] = track
        if self._data_dir:
            self._data_dir.mkdir(parents=True, exist_ok=True)
            path = self._data_dir / f"{track.track_id}.json"
            data = asdict(track)
            with open(path, "w") as fp:
                json.dump(data, fp, indent=2)

    def get(self, track_id: str) -> TrackInfo | None:
        return self._tracks.get(track_id)

    def list_tracks(self) -> list[TrackInfo]:
        return list(self._tracks.values())

    def identify(self, x: np.ndarray, z: np.ndarray) -> TrackInfo | None:
        return match_track(x, z, self._tracks)

    def learn_track(
        self,
        name: str,
        x: np.ndarray,
        z: np.ndarray,
        corners: list[TrackCornerInfo] | None = None,
        country: str = "",
    ) -> TrackInfo:
        dx = np.diff(x)
        dz = np.diff(z)
        length = float(np.sum(np.sqrt(dx**2 + dz**2)))
        track_id = fingerprint_track(x, z)

        info = TrackInfo(
            track_id=track_id,
            name=name,
            country=country,
            length_m=length,
            corners=corners or [],
            reference_line_x=x.tolist(),
            reference_line_z=z.tolist(),
            source="learned",
        )
        self.save(info)
        return info
