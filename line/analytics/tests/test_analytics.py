import io
import json
import tempfile
from pathlib import Path

import numpy as np
import pyarrow as pa
import pyarrow.parquet as pq

from analytics.silver import compute_lap_metrics
from analytics.gold import compute_consistency
from analytics.corners import detect_corners, detect_sectors, compute_curvature
from analytics.tracks import TrackDatabase, TrackInfo, TrackCornerInfo, fingerprint_track
from analytics.mastery import score_corners, compute_progression
from analytics.strategy import compute_tyre_degradation, compute_fuel_strategy
from analytics.journal import generate_journal


def _make_oval_track(num_frames: int = 3600):
    t = np.linspace(0, 2 * np.pi, num_frames, endpoint=False)
    x = np.cos(t) * 200
    z = np.sin(t) * 100
    speed = 80 + 40 * np.abs(np.cos(t))
    return x, z, speed


def _make_test_parquet(num_frames: int = 3600) -> bytes:
    rng = np.random.default_rng(42)
    t = np.linspace(0, 2 * np.pi * 3, num_frames)
    x = np.cos(t) * 100 + rng.normal(0, 0.1, num_frames)
    y = np.zeros(num_frames)
    z = np.sin(t) * 100 + rng.normal(0, 0.1, num_frames)
    speed = 80 + 40 * np.abs(np.sin(t)) + rng.normal(0, 2, num_frames)
    throttle = np.where(speed > 100, 200.0, 50.0) + rng.normal(0, 5, num_frames)
    brake = np.where(speed < 60, 150.0, 0.0) + rng.normal(0, 3, num_frames)
    brake = np.clip(brake, 0, 255)
    throttle = np.clip(throttle, 0, 255)
    rpm = speed * 50 + rng.normal(0, 100, num_frames)
    fuel = np.linspace(100, 95, num_frames)

    table = pa.table({
        "pos_x": pa.array(x, type=pa.float32()),
        "pos_y": pa.array(y, type=pa.float32()),
        "pos_z": pa.array(z, type=pa.float32()),
        "speed": pa.array(speed, type=pa.float32()),
        "throttle": pa.array(throttle, type=pa.float32()),
        "brake": pa.array(brake, type=pa.float32()),
        "rpm": pa.array(rpm, type=pa.float32()),
        "fuel_level": pa.array(fuel, type=pa.float32()),
        "tire_fl_temp": pa.array(rng.normal(80, 5, num_frames), type=pa.float32()),
        "tire_fr_temp": pa.array(rng.normal(82, 5, num_frames), type=pa.float32()),
        "tire_rl_temp": pa.array(rng.normal(78, 5, num_frames), type=pa.float32()),
        "tire_rr_temp": pa.array(rng.normal(79, 5, num_frames), type=pa.float32()),
    })

    buf = io.BytesIO()
    pq.write_table(table, buf, compression="snappy")
    return buf.getvalue()


def test_compute_lap_metrics():
    data = _make_test_parquet()
    metrics = compute_lap_metrics(data)

    assert metrics["frame_count"] == 3600
    assert metrics["total_distance_m"] > 0
    assert metrics["top_speed"] > 0
    assert metrics["brake_count"] > 0
    assert 0 <= metrics["throttle_pct"] <= 100
    assert 0 <= metrics["brake_pct"] <= 100
    assert 0 <= metrics["coast_pct"] <= 100
    assert abs(metrics["throttle_pct"] + metrics["brake_pct"] + metrics["coast_pct"] - 100) < 1
    assert metrics["fuel_used"] > 0
    assert len(metrics["avg_tire_temps"]) == 4


def test_compute_consistency():
    laps = [
        {"lap_time_ms": 90000, "top_speed": 250, "brake_count": 12},
        {"lap_time_ms": 90500, "top_speed": 248, "brake_count": 13},
        {"lap_time_ms": 91000, "top_speed": 252, "brake_count": 11},
        {"lap_time_ms": 90200, "top_speed": 249, "brake_count": 12},
    ]
    result = compute_consistency(laps)

    assert result["consistency_score"] > 0.9
    assert result["lap_count"] == 4
    assert result["best_worst_delta_ms"] == 1000
    assert result["lap_time_cv"] < 0.01


def test_consistency_single_lap():
    result = compute_consistency([{"lap_time_ms": 90000, "top_speed": 250, "brake_count": 12}])
    assert result["consistency_score"] == 1.0


def test_detect_corners():
    x, z, speed = _make_oval_track(3600)
    corners = detect_corners(x.astype(np.float64), z.astype(np.float64), speed.astype(np.float64))
    assert len(corners) >= 2
    for c in corners:
        assert c.entry_idx < c.apex_idx < c.exit_idx
        assert c.apex_speed <= c.entry_speed or c.apex_speed <= c.exit_speed
        assert c.direction in ("left", "right")


def test_detect_sectors():
    x, z, speed = _make_oval_track(3600)
    corners = detect_corners(x.astype(np.float64), z.astype(np.float64), speed.astype(np.float64))
    sectors = detect_sectors(x.astype(np.float64), z.astype(np.float64), speed.astype(np.float64), corners)
    assert len(sectors) >= 1
    for s in sectors:
        assert s.sector_type in ("straight", "corner_complex")
        assert s.length_m > 0


def test_compute_curvature():
    t = np.linspace(0, 2 * np.pi, 1000)
    x = np.cos(t) * 100
    z = np.sin(t) * 100
    curv = compute_curvature(x, z)
    assert len(curv) == 1000
    assert np.mean(curv) > 0


def test_fingerprint_track():
    t = np.linspace(0, 2 * np.pi, 1000)
    x = np.cos(t) * 100
    z = np.sin(t) * 100
    fp1 = fingerprint_track(x, z)
    fp2 = fingerprint_track(x, z)
    assert fp1 == fp2
    assert len(fp1) == 16

    x2 = np.cos(t) * 200
    z2 = np.sin(t) * 200
    fp3 = fingerprint_track(x2, z2)
    assert fp3 == fp1

    x3 = np.cos(t) * 100
    z3 = np.sin(t) * 50
    fp4 = fingerprint_track(x3, z3)
    assert fp4 != fp1


def test_track_database():
    with tempfile.TemporaryDirectory() as tmpdir:
        db = TrackDatabase(tmpdir)
        t = np.linspace(0, 2 * np.pi, 1000)
        x = np.cos(t) * 100
        z = np.sin(t) * 100

        track = db.learn_track("Test Oval", x, z, country="Test")
        assert track.name == "Test Oval"
        assert track.length_m > 0
        assert track.source == "learned"

        found = db.identify(x, z)
        assert found is not None
        assert found.name == "Test Oval"

        db2 = TrackDatabase(tmpdir)
        assert len(db2.list_tracks()) == 1
        assert db2.get(track.track_id) is not None


def test_community_tracks_load():
    data_path = Path(__file__).parent.parent.parent / "data" / "tracks.json"
    if not data_path.exists():
        data_path = Path("line/data/tracks.json")
    if not data_path.exists():
        return

    with open(data_path) as f:
        data = json.load(f)

    tracks = data["tracks"]
    assert len(tracks) >= 5
    for t in tracks:
        assert "track_id" in t
        assert "name" in t
        assert "corners" in t
        assert len(t["corners"]) > 0
        for c in t["corners"]:
            assert "number" in c
            assert "name" in c
            assert "direction" in c


def test_score_corners():
    from analytics.corners import Corner

    corners_lap1 = [
        Corner(entry_idx=100, apex_idx=120, exit_idx=140, entry_speed=150, apex_speed=80, exit_speed=130, direction="left", curvature=0.01, length_m=50),
        Corner(entry_idx=300, apex_idx=320, exit_idx=340, entry_speed=180, apex_speed=100, exit_speed=160, direction="right", curvature=0.008, length_m=60),
    ]
    corners_lap2 = [
        Corner(entry_idx=102, apex_idx=122, exit_idx=142, entry_speed=148, apex_speed=79, exit_speed=128, direction="left", curvature=0.01, length_m=50),
        Corner(entry_idx=298, apex_idx=318, exit_idx=338, entry_speed=182, apex_speed=102, exit_speed=162, direction="right", curvature=0.008, length_m=60),
    ]
    corners_lap3 = [
        Corner(entry_idx=101, apex_idx=121, exit_idx=141, entry_speed=149, apex_speed=81, exit_speed=131, direction="left", curvature=0.01, length_m=50),
        Corner(entry_idx=301, apex_idx=321, exit_idx=341, entry_speed=179, apex_speed=99, exit_speed=159, direction="right", curvature=0.008, length_m=60),
    ]

    laps_data = [{"lap_time_ms": 90000}, {"lap_time_ms": 90200}, {"lap_time_ms": 90100}]
    corners_per_lap = [corners_lap1, corners_lap2, corners_lap3]

    scores = score_corners(laps_data, corners_per_lap)
    assert len(scores) == 2
    for s in scores:
        assert s.overall > 0.8
        assert s.braking_point_drift < 5


def test_compute_progression():
    sessions = [
        {"laps": [{"lap_time_ms": 95000, "top_speed": 240}, {"lap_time_ms": 94000, "top_speed": 242}]},
        {"laps": [{"lap_time_ms": 93000, "top_speed": 245}, {"lap_time_ms": 92500, "top_speed": 246}]},
        {"laps": [{"lap_time_ms": 91000, "top_speed": 248}, {"lap_time_ms": 90500, "top_speed": 250}]},
    ]
    points = compute_progression(sessions)
    assert len(points) == 3
    assert points[0].lap_time_ms > points[2].lap_time_ms
    assert points[2].top_speed > points[0].top_speed


def test_tyre_degradation():
    laps = [
        {"tire_fl_temp": 75, "tire_fr_temp": 76, "tire_rl_temp": 73, "tire_rr_temp": 74},
        {"tire_fl_temp": 78, "tire_fr_temp": 79, "tire_rl_temp": 76, "tire_rr_temp": 77},
        {"tire_fl_temp": 82, "tire_fr_temp": 83, "tire_rl_temp": 80, "tire_rr_temp": 81},
        {"tire_fl_temp": 86, "tire_fr_temp": 87, "tire_rl_temp": 84, "tire_rr_temp": 85},
    ]
    result = compute_tyre_degradation(laps, tire_radius_first=0.28)
    assert result.degradation_rate > 0
    assert result.estimated_laps_remaining > 0
    assert result.compound_guess == "soft"
    assert len(result.avg_temp_per_lap) == 4


def test_fuel_strategy():
    fuel_per_lap = [3.2, 3.1, 3.3, 3.2]
    result = compute_fuel_strategy(fuel_per_lap, current_fuel=50.0, fuel_capacity=100.0, total_race_laps=20)
    assert result.consumption_per_lap > 3.0
    assert result.laps_remaining > 10
    assert result.laps_remaining < 20


def test_generate_journal():
    lap_metrics = [
        {"lap_time_ms": 90000, "top_speed": 250, "brake_count": 12},
        {"lap_time_ms": 90500, "top_speed": 248, "brake_count": 13},
        {"lap_time_ms": 91000, "top_speed": 252, "brake_count": 11},
        {"lap_time_ms": 89500, "top_speed": 251, "brake_count": 12},
    ]
    journal = generate_journal("sess-001", "Tsukuba Circuit", 1234, lap_metrics)
    assert journal.total_laps == 4
    assert journal.best_lap_ms == 89500
    assert journal.worst_lap_ms == 91000
    assert journal.consistency_score > 0.8
    assert len(journal.summary) > 0
