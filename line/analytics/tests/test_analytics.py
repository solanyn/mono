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
    speed = (80 + 40 * np.abs(np.sin(t)) + rng.normal(0, 2, num_frames)).astype(np.float32)
    throttle = np.clip(np.where(speed > 100, 200, 50) + rng.integers(-5, 5, num_frames), 0, 255).astype(np.int32)
    brake = np.clip(np.where(speed < 60, 150, 0) + rng.integers(-3, 3, num_frames), 0, 255).astype(np.int32)
    steering = np.clip((np.sin(t * 2) * 60).astype(np.int32), -128, 127)
    rpm = (speed * 50 + rng.normal(0, 100, num_frames)).astype(np.float32)
    fuel = np.linspace(100, 95, num_frames, dtype=np.float32)

    table = pa.table({
        "packet_id": pa.array(np.arange(num_frames, dtype=np.int32)),
        "pos_x": pa.array(x, type=pa.float32()),
        "pos_y": pa.array(y, type=pa.float32()),
        "pos_z": pa.array(z, type=pa.float32()),
        "vel_x": pa.array(np.gradient(x).astype(np.float32)),
        "vel_y": pa.array(np.zeros(num_frames, dtype=np.float32)),
        "vel_z": pa.array(np.gradient(z).astype(np.float32)),
        "rot_pitch": pa.array(np.zeros(num_frames, dtype=np.float32)),
        "rot_yaw": pa.array(np.arctan2(np.gradient(z), np.gradient(x)).astype(np.float32)),
        "rot_roll": pa.array(np.zeros(num_frames, dtype=np.float32)),
        "ang_vel_x": pa.array(np.zeros(num_frames, dtype=np.float32)),
        "ang_vel_y": pa.array(rng.normal(0, 0.1, num_frames).astype(np.float32)),
        "ang_vel_z": pa.array(np.zeros(num_frames, dtype=np.float32)),
        "ride_height": pa.array(np.full(num_frames, 0.12, dtype=np.float32)),
        "rpm": pa.array(rpm),
        "fuel_level": pa.array(fuel),
        "fuel_cap": pa.array(np.full(num_frames, 100.0, dtype=np.float32)),
        "speed": pa.array(speed),
        "boost": pa.array(np.zeros(num_frames, dtype=np.float32)),
        "oil_pressure": pa.array(np.full(num_frames, 4.5, dtype=np.float32)),
        "water_temp": pa.array(np.full(num_frames, 85.0, dtype=np.float32)),
        "oil_temp": pa.array(np.full(num_frames, 95.0, dtype=np.float32)),
        "tire_temp_fl": pa.array(rng.normal(80, 5, num_frames).astype(np.float32)),
        "tire_temp_fr": pa.array(rng.normal(82, 5, num_frames).astype(np.float32)),
        "tire_temp_rl": pa.array(rng.normal(78, 5, num_frames).astype(np.float32)),
        "tire_temp_rr": pa.array(rng.normal(79, 5, num_frames).astype(np.float32)),
        "susp_fl": pa.array(rng.normal(0.1, 0.01, num_frames).astype(np.float32)),
        "susp_fr": pa.array(rng.normal(0.1, 0.01, num_frames).astype(np.float32)),
        "susp_rl": pa.array(rng.normal(0.1, 0.01, num_frames).astype(np.float32)),
        "susp_rr": pa.array(rng.normal(0.1, 0.01, num_frames).astype(np.float32)),
        "wheel_fl": pa.array(rng.normal(50, 2, num_frames).astype(np.float32)),
        "wheel_fr": pa.array(rng.normal(50, 2, num_frames).astype(np.float32)),
        "wheel_rl": pa.array(rng.normal(50, 2, num_frames).astype(np.float32)),
        "wheel_rr": pa.array(rng.normal(50, 2, num_frames).astype(np.float32)),
        "tire_rad_fl": pa.array(np.full(num_frames, 0.30, dtype=np.float32)),
        "tire_rad_fr": pa.array(np.full(num_frames, 0.30, dtype=np.float32)),
        "tire_rad_rl": pa.array(np.full(num_frames, 0.31, dtype=np.float32)),
        "tire_rad_rr": pa.array(np.full(num_frames, 0.31, dtype=np.float32)),
        "current_lap": pa.array(np.ones(num_frames, dtype=np.int32)),
        "total_laps": pa.array(np.full(num_frames, 5, dtype=np.int32)),
        "best_lap_ms": pa.array(np.full(num_frames, 90000, dtype=np.int32)),
        "last_lap_ms": pa.array(np.full(num_frames, 91000, dtype=np.int32)),
        "current_time_ms": pa.array(np.linspace(0, 60000, num_frames, dtype=np.int32)),
        "throttle": pa.array(throttle),
        "brake": pa.array(brake),
        "steering": pa.array(steering),
        "gear": pa.array(np.clip((speed / 40).astype(np.int32), 1, 6)),
        "car_id": pa.array(np.full(num_frames, 3447, dtype=np.int32)),
        "flags": pa.array(np.ones(num_frames, dtype=np.int32)),
        "time_of_day": pa.array(np.linspace(43200000, 43260000, num_frames, dtype=np.int32)),
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


def test_align_lap():
    from analytics.alignment import align_lap, compute_distance

    t = np.linspace(0, 2 * np.pi, 600)
    x = np.cos(t) * 100
    z = np.sin(t) * 100
    speed = 80 + 40 * np.abs(np.cos(t))
    throttle = np.where(speed > 100, 200.0, 50.0)
    brake = np.where(speed < 70, 150.0, 0.0)
    time_s = np.linspace(0, 60, 600)

    aligned = align_lap(x, z, speed, throttle, brake, time_s, num_points=500)
    assert len(aligned.distance) == 500
    assert len(aligned.speed) == 500
    assert aligned.distance[0] == 0
    assert aligned.distance[-1] > 0


def test_compute_time_delta():
    from analytics.alignment import align_lap, compute_time_delta

    t = np.linspace(0, 2 * np.pi, 600)
    x = np.cos(t) * 100
    z = np.sin(t) * 100
    speed1 = 80 + 40 * np.abs(np.cos(t))
    speed2 = 82 + 40 * np.abs(np.cos(t))
    throttle = np.where(speed1 > 100, 200.0, 50.0)
    brake = np.where(speed1 < 70, 150.0, 0.0)
    time1 = np.linspace(0, 60, 600)
    time2 = np.linspace(0, 58, 600)

    ref = align_lap(x, z, speed1, throttle, brake, time1, num_points=500)
    cmp = align_lap(x, z, speed2, throttle, brake, time2, num_points=500)

    delta = compute_time_delta(ref, cmp)
    assert delta.total_delta_s < 0
    assert len(delta.distance) == 500
    assert len(delta.delta_s) == 500


def test_analyze_braking():
    from analytics.braking import analyze_braking

    n = 1200
    t = np.linspace(0, 4 * np.pi, n)
    x = np.cos(t) * 200
    z = np.sin(t) * 200
    speed = 120 + 60 * np.cos(t)
    brake = np.zeros(n)
    brake[100:150] = 180
    brake[400:460] = 200
    brake[700:770] = 160

    result = analyze_braking(speed, brake, x, z)
    assert len(result.events) == 3
    assert result.avg_deceleration_g > 0
    assert result.total_brake_distance_m > 0
    assert 0 <= result.consistency_score <= 1


def test_racing_line_deviation():
    from analytics.racing_line import compute_racing_line_deviation

    rng = np.random.default_rng(42)
    t = np.linspace(0, 2 * np.pi, 600)
    laps_x = [np.cos(t) * 100 + rng.normal(0, 0.5, 600) for _ in range(5)]
    laps_z = [np.sin(t) * 100 + rng.normal(0, 0.5, 600) for _ in range(5)]

    result = compute_racing_line_deviation(laps_x, laps_z)
    assert result.consistency > 0.5
    assert result.deviation_avg_m > 0
    assert result.deviation_max_m >= result.deviation_avg_m


def test_analyze_stability():
    from analytics.stability import analyze_stability

    n = 1200
    t = np.linspace(0, 4 * np.pi, n)
    x = np.cos(t) * 200
    z = np.sin(t) * 200
    speed = 100 + 50 * np.cos(t)
    steering = (np.sin(t) * 80).astype(np.int32)

    result = analyze_stability(x, z, speed, steering)
    assert result.oversteer_count >= 0
    assert result.understeer_count >= 0
    assert 0 <= result.stability_score <= 1


def test_classify_corners():
    from analytics.corners import Corner
    from analytics.classification import classify_corners

    t = np.linspace(0, 2 * np.pi, 3600)
    x = np.cos(t) * 200
    z = np.sin(t) * 100

    corners = [
        Corner(entry_idx=100, apex_idx=200, exit_idx=300, entry_speed=150, apex_speed=60, exit_speed=130, direction="left", curvature=0.02, length_m=80),
        Corner(entry_idx=900, apex_idx=950, exit_idx=1000, entry_speed=200, apex_speed=180, exit_speed=195, direction="right", curvature=0.003, length_m=40),
    ]

    classified = classify_corners(corners, x, z)
    assert len(classified) == 2
    assert classified[0].classification in ("hairpin", "tight", "medium", "sweeper", "kink")
    assert classified[0].radius_m > 0
    assert classified[0].angle_deg > 0


def test_analyze_fatigue():
    from analytics.fatigue import analyze_fatigue

    lap_times = [90000, 90200, 90500, 91000, 91500, 92000, 92800, 93500]
    top_speeds = [250, 249, 248, 247, 246, 245, 244, 243]
    brake_counts = [12, 12, 13, 13, 14, 14, 15, 15]
    tire_temps = [75, 77, 79, 82, 85, 88, 91, 94]

    result = analyze_fatigue(lap_times, top_speeds, brake_counts, tire_temps)
    assert result.diagnosis in ("stable", "driver_fatigue", "tyre_degradation", "mixed")
    assert result.lap_time_trend > 0
    assert result.tyre_degradation_score > 0


def test_fatigue_insufficient_data():
    from analytics.fatigue import analyze_fatigue

    result = analyze_fatigue([90000, 91000], [250, 248], [12, 13], [75, 77])
    assert result.diagnosis == "insufficient_data"


class FakeS3:
    def __init__(self):
        self.objects = {}

    def get_object(self, Bucket, Key):
        data = self.objects.get(f"{Bucket}/{Key}")
        if data is None:
            raise Exception(f"NoSuchKey: {Bucket}/{Key}")
        return {"Body": io.BytesIO(data)}

    def put_object(self, Bucket, Key, Body, ContentType=None):
        self.objects[f"{Bucket}/{Key}"] = Body if isinstance(Body, bytes) else Body.encode()


def test_worker_process_lap_e2e():
    from analytics.worker import AnalyticsWorker

    parquet_data = _make_test_parquet(1800)

    worker = AnalyticsWorker.__new__(AnalyticsWorker)
    worker.s3 = FakeS3()
    worker.bronze_bucket = "line-bronze"
    worker.silver_bucket = "line-silver"
    worker.gold_bucket = "line-gold"
    worker.track_db = None

    from analytics.tracks import TrackDatabase
    worker.track_db = TrackDatabase(None)

    worker.s3.objects["line-bronze/bronze/telemetry/sess-e2e/2026/05/12/lap-001-abc12345.parquet"] = parquet_data

    event = {
        "session_id": "sess-e2e",
        "lap_number": 1,
        "s3_key": "bronze/telemetry/sess-e2e/2026/05/12/lap-001-abc12345.parquet",
    }
    worker.process_lap(event)

    silver_key = "line-silver/laps/sess-e2e/001/metrics.json"
    assert silver_key in worker.s3.objects

    metrics = json.loads(worker.s3.objects[silver_key])
    assert metrics["session_id"] == "sess-e2e"
    assert metrics["lap_number"] == 1
    assert metrics["frame_count"] == 1800
    assert metrics["total_distance_m"] > 0
    assert metrics["top_speed"] > 0
    assert metrics["brake_count"] >= 0
    assert 0 <= metrics["throttle_pct"] <= 100
    assert 0 <= metrics["brake_pct"] <= 100
    assert len(metrics["avg_tire_temps"]) == 4
    assert metrics["fuel_used"] > 0

    assert "corners" in metrics
    assert "classified_corners" in metrics
    assert "braking" in metrics
    assert metrics["braking"]["avg_deceleration_g"] >= 0
    assert metrics["braking"]["total_brake_distance_m"] >= 0
    assert "stability" in metrics
    assert metrics["stability"]["stability_score"] >= 0
    assert "aligned" in metrics
    assert len(metrics["aligned"]["distance"]) == 1000
    assert len(metrics["aligned"]["speed"]) == 1000


def test_worker_process_session_e2e():
    from analytics.worker import AnalyticsWorker
    from analytics.tracks import TrackDatabase

    worker = AnalyticsWorker.__new__(AnalyticsWorker)
    worker.s3 = FakeS3()
    worker.bronze_bucket = "line-bronze"
    worker.silver_bucket = "line-silver"
    worker.gold_bucket = "line-gold"
    worker.track_db = TrackDatabase(None)

    for lap_num in range(1, 5):
        parquet_data = _make_test_parquet(1800)
        bronze_key = f"bronze/telemetry/sess-e2e2/2026/05/12/lap-{lap_num:03d}-abc.parquet"
        worker.s3.objects[f"line-bronze/{bronze_key}"] = parquet_data

        event = {"session_id": "sess-e2e2", "lap_number": lap_num, "s3_key": bronze_key}
        worker.process_lap(event)

    for lap_num in range(1, 5):
        silver_key = f"line-silver/laps/sess-e2e2/{lap_num:03d}/metrics.json"
        assert silver_key in worker.s3.objects

    session_event = {
        "session_id": "sess-e2e2",
        "car_code": 3447,
        "track_name": "Test Circuit",
        "lap_count": 4,
    }
    worker.process_session_complete(session_event)

    gold_key = "line-gold/sessions/sess-e2e2/summary.json"
    assert gold_key in worker.s3.objects

    summary = json.loads(worker.s3.objects[gold_key])
    assert summary["session_id"] == "sess-e2e2"
    assert summary["car_code"] == 3447
    assert summary["track_name"] == "Test Circuit"
    assert summary["lap_count"] == 4

    assert "consistency" in summary
    assert summary["consistency"]["lap_count"] >= 2

    assert "tyre_degradation" in summary
    assert len(summary["tyre_degradation"]["avg_temp_per_lap"]) == 4

    assert "fuel_strategy" in summary
    assert summary["fuel_strategy"]["consumption_per_lap"] > 0

    assert "journal" in summary
    assert summary["journal"]["total_laps"] == 4

    assert "racing_line" in summary
    assert summary["racing_line"]["consistency"] > 0
    assert summary["racing_line"]["deviation_avg_m"] > 0

    assert "fatigue" in summary
    assert summary["fatigue"]["diagnosis"] in ("stable", "driver_fatigue", "tyre_degradation", "mixed", "insufficient_data")

    assert "time_deltas" in summary


def test_worker_steering_int32_handling():
    parquet_data = _make_test_parquet(600)

    table = pq.read_table(io.BytesIO(parquet_data))
    df = table.to_pandas()

    assert df["throttle"].dtype in (np.int32, np.int64)
    assert df["brake"].dtype in (np.int32, np.int64)
    assert df["steering"].dtype in (np.int32, np.int64)

    assert df["throttle"].min() >= 0
    assert df["throttle"].max() <= 255
    assert df["brake"].min() >= 0
    assert df["brake"].max() <= 255
    assert df["steering"].min() >= -128
    assert df["steering"].max() <= 127

    from analytics.stability import analyze_stability
    x = df["pos_x"].values.astype(float)
    z = df["pos_z"].values.astype(float)
    speed = df["speed"].values.astype(float)
    steering = df["steering"].values.astype(float)

    result = analyze_stability(x, z, speed, steering)
    assert 0 <= result.stability_score <= 1
