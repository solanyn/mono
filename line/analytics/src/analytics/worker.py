import io
import json
import logging
import os
import signal
import sys
import time
from dataclasses import asdict

import boto3
import numpy as np
import pyarrow.parquet as pq
from confluent_kafka import Consumer, KafkaError

from analytics.silver import compute_lap_metrics
from analytics.gold import compute_consistency
from analytics.corners import detect_corners
from analytics.tracks import TrackDatabase
from analytics.mastery import score_corners
from analytics.strategy import compute_tyre_degradation, compute_fuel_strategy
from analytics.journal import generate_journal
from analytics.alignment import align_lap, compute_time_delta
from analytics.braking import analyze_braking
from analytics.racing_line import compute_racing_line_deviation, compute_optimal_line
from analytics.stability import analyze_stability
from analytics.classification import classify_corners
from analytics.fatigue import analyze_fatigue

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)


class AnalyticsWorker:
    def __init__(self):
        self.s3 = boto3.client(
            "s3",
            endpoint_url=os.environ.get("S3_ENDPOINT", "http://localhost:3900"),
            aws_access_key_id=os.environ.get("S3_ACCESS_KEY", ""),
            aws_secret_access_key=os.environ.get("S3_SECRET_KEY", ""),
            region_name=os.environ.get("S3_REGION", "us-east-1"),
        )
        self.bronze_bucket = os.environ.get("S3_BRONZE_BUCKET", "line-bronze")
        self.silver_bucket = os.environ.get("S3_SILVER_BUCKET", "line-silver")
        self.gold_bucket = os.environ.get("S3_GOLD_BUCKET", "line-gold")
        self.track_db = TrackDatabase(os.environ.get("TRACK_DATA_DIR", None))
        self.running = True

    def process_lap(self, event: dict):
        session_id = event.get("session_id", "")
        lap_num = event.get("lap_number", 0)
        s3_key = event.get("s3_key", "")

        if not s3_key:
            logger.warning("no s3_key in event")
            return

        logger.info("processing lap", extra={"session_id": session_id, "lap": lap_num, "key": s3_key})

        resp = self.s3.get_object(Bucket=self.bronze_bucket, Key=s3_key)
        parquet_bytes = resp["Body"].read()

        metrics = compute_lap_metrics(parquet_bytes)
        metrics["session_id"] = session_id
        metrics["lap_number"] = lap_num
        metrics["s3_key"] = s3_key

        table = pq.read_table(io.BytesIO(parquet_bytes))
        df = table.to_pandas()

        if "pos_x" in df.columns and "pos_z" in df.columns:
            x = df["pos_x"].values.astype(float)
            z = df["pos_z"].values.astype(float)
            speed = df["speed"].values.astype(float) if "speed" in df.columns else None

            track = self.track_db.identify(x, z)
            if track:
                metrics["track_id"] = track.track_id
                metrics["track_name"] = track.name

            if speed is not None:
                corners = detect_corners(x, z, speed)
                metrics["corner_count"] = len(corners)
                metrics["corners"] = [
                    {
                        "entry_idx": c.entry_idx,
                        "apex_idx": c.apex_idx,
                        "exit_idx": c.exit_idx,
                        "entry_speed": c.entry_speed,
                        "apex_speed": c.apex_speed,
                        "exit_speed": c.exit_speed,
                        "direction": c.direction,
                    }
                    for c in corners
                ]

                classified = classify_corners(corners, x, z)
                metrics["classified_corners"] = [
                    {
                        "corner_idx": i,
                        "classification": cc.classification,
                        "radius_m": cc.radius_m,
                        "angle_deg": cc.angle_deg,
                        "is_complex": cc.is_complex,
                        "direction": cc.corner.direction,
                        "entry_speed": cc.corner.entry_speed,
                        "apex_speed": cc.corner.apex_speed,
                        "exit_speed": cc.corner.exit_speed,
                    }
                    for i, cc in enumerate(classified)
                ]

                brake = df["brake"].values.astype(float) if "brake" in df.columns else None
                if brake is not None:
                    braking = analyze_braking(speed, brake, x, z)
                    metrics["braking"] = {
                        "avg_deceleration_g": braking.avg_deceleration_g,
                        "avg_trail_brake_pct": braking.avg_trail_brake_pct,
                        "avg_release_smoothness": braking.avg_release_smoothness,
                        "avg_efficiency": braking.avg_efficiency,
                        "consistency_score": braking.consistency_score,
                        "total_brake_distance_m": braking.total_brake_distance_m,
                        "events": [
                            {
                                "start_idx": e.start_idx,
                                "end_idx": e.end_idx,
                                "start_speed": e.start_speed,
                                "end_speed": e.end_speed,
                                "deceleration_g": e.deceleration_g,
                                "duration_s": e.duration_s,
                                "distance_m": e.distance_m,
                                "trail_brake_pct": e.trail_brake_pct,
                                "release_smoothness": e.release_smoothness,
                                "efficiency": e.efficiency,
                            }
                            for e in braking.events
                        ],
                    }

                steering = df["steering"].values.astype(float) if "steering" in df.columns else None
                if steering is not None:
                    stability = analyze_stability(x, z, speed, steering)
                    metrics["stability"] = {
                        "oversteer_count": stability.oversteer_count,
                        "understeer_count": stability.understeer_count,
                        "avg_yaw_deviation": stability.avg_yaw_deviation,
                        "stability_score": stability.stability_score,
                        "worst_corner_idx": stability.worst_corner_idx,
                        "events": [
                            {
                                "start_idx": e.start_idx,
                                "end_idx": e.end_idx,
                                "event_type": e.event_type,
                                "severity": e.severity,
                                "yaw_rate": e.yaw_rate,
                                "steering_angle": e.steering_angle,
                                "speed": e.speed,
                            }
                            for e in stability.events
                        ],
                    }

                throttle = df["throttle"].values.astype(float) if "throttle" in df.columns else None
                if throttle is not None and brake is not None:
                    time_s = np.arange(len(speed)) / 60.0
                    aligned = align_lap(x, z, speed, throttle, brake, time_s)
                    metrics["aligned"] = {
                        "distance": aligned.distance.tolist(),
                        "speed": aligned.speed.tolist(),
                        "throttle": aligned.throttle.tolist(),
                        "brake": aligned.brake.tolist(),
                        "x": aligned.x.tolist(),
                        "z": aligned.z.tolist(),
                        "time_s": aligned.time_s.tolist(),
                    }

        silver_key = f"laps/{session_id}/{lap_num:03d}/metrics.json"
        self.s3.put_object(
            Bucket=self.silver_bucket,
            Key=silver_key,
            Body=json.dumps(metrics, default=str).encode(),
            ContentType="application/json",
        )
        logger.info("wrote silver metrics", extra={"key": silver_key})

    def process_session_complete(self, event: dict):
        session_id = event.get("session_id", "")
        car_code = event.get("car_code", 0)
        track_name = event.get("track_name", "Unknown")
        lap_count = event.get("lap_count", 0)

        lap_metrics = []
        for lap_num in range(1, lap_count + 1):
            key = f"laps/{session_id}/{lap_num:03d}/metrics.json"
            try:
                resp = self.s3.get_object(Bucket=self.silver_bucket, Key=key)
                m = json.loads(resp["Body"].read())
                lap_metrics.append(m)
            except Exception:
                continue

        if not lap_metrics:
            return

        consistency = compute_consistency(lap_metrics)

        tire_temps = [
            {
                "tire_fl_temp": m.get("avg_tire_temps", {}).get("tire_fl_temp", 0),
                "tire_fr_temp": m.get("avg_tire_temps", {}).get("tire_fr_temp", 0),
                "tire_rl_temp": m.get("avg_tire_temps", {}).get("tire_rl_temp", 0),
                "tire_rr_temp": m.get("avg_tire_temps", {}).get("tire_rr_temp", 0),
            }
            for m in lap_metrics
        ]
        tyre_deg = compute_tyre_degradation(tire_temps)

        fuel_per_lap = [m.get("fuel_used", 0) for m in lap_metrics]
        last_fuel = lap_metrics[-1].get("fuel_level", 0) if lap_metrics else 0
        fuel_strategy = compute_fuel_strategy(fuel_per_lap, last_fuel, 100.0)

        journal = generate_journal(session_id, track_name, car_code, lap_metrics)

        racing_line = self._compute_racing_line(lap_metrics)
        fatigue = self._compute_fatigue(lap_metrics)
        time_deltas = self._compute_time_deltas(lap_metrics)

        gold = {
            "session_id": session_id,
            "car_code": car_code,
            "track_name": track_name,
            "lap_count": lap_count,
            "consistency": consistency,
            "tyre_degradation": asdict(tyre_deg),
            "fuel_strategy": asdict(fuel_strategy),
            "journal": asdict(journal),
            "racing_line": racing_line,
            "fatigue": fatigue,
            "time_deltas": time_deltas,
        }

        gold_key = f"sessions/{session_id}/summary.json"
        self.s3.put_object(
            Bucket=self.gold_bucket,
            Key=gold_key,
            Body=json.dumps(gold, default=str).encode(),
            ContentType="application/json",
        )
        logger.info("wrote gold summary", extra={"key": gold_key})

    def _compute_racing_line(self, lap_metrics: list[dict]) -> dict:
        laps_with_aligned = [m for m in lap_metrics if "aligned" in m]
        if len(laps_with_aligned) < 2:
            return {}

        laps_x = [np.array(m["aligned"]["x"]) for m in laps_with_aligned]
        laps_z = [np.array(m["aligned"]["z"]) for m in laps_with_aligned]
        laps_speed = [np.array(m["aligned"]["speed"]) for m in laps_with_aligned]

        deviation = compute_racing_line_deviation(laps_x, laps_z)
        optimal = compute_optimal_line(laps_x, laps_z, laps_speed)

        return {
            "consistency": deviation.consistency,
            "smoothness": deviation.smoothness,
            "deviation_avg_m": deviation.deviation_avg_m,
            "deviation_max_m": deviation.deviation_max_m,
            "worst_sections": deviation.worst_sections,
            "optimal_line": optimal,
        }

    def _compute_fatigue(self, lap_metrics: list[dict]) -> dict:
        times = [m.get("lap_time_ms", 0) for m in lap_metrics]
        speeds = [m.get("top_speed", 0) for m in lap_metrics]
        brakes = [m.get("brake_count", 0) for m in lap_metrics]
        temps = [
            np.mean(list(m.get("avg_tire_temps", {}).values())) if m.get("avg_tire_temps") else 0
            for m in lap_metrics
        ]

        brake_drifts = []
        for m in lap_metrics:
            braking_data = m.get("braking", {})
            events = braking_data.get("events", [])
            if events:
                points = [e["start_idx"] for e in events]
                brake_drifts.append(float(np.std(points)) if len(points) > 1 else 0.0)

        result = analyze_fatigue(times, speeds, brakes, temps, brake_drifts or None)
        return asdict(result)

    def _compute_time_deltas(self, lap_metrics: list[dict]) -> list[dict]:
        laps_with_aligned = [m for m in lap_metrics if "aligned" in m]
        if len(laps_with_aligned) < 2:
            return []

        times = [m.get("lap_time_ms", 0) for m in laps_with_aligned]
        best_idx = int(np.argmin([t for t in times if t > 0])) if any(t > 0 for t in times) else 0

        best = laps_with_aligned[best_idx]
        from analytics.alignment import AlignedLap
        ref = AlignedLap(
            distance=np.array(best["aligned"]["distance"]),
            speed=np.array(best["aligned"]["speed"]),
            throttle=np.array(best["aligned"]["throttle"]),
            brake=np.array(best["aligned"]["brake"]),
            x=np.array(best["aligned"]["x"]),
            z=np.array(best["aligned"]["z"]),
            time_s=np.array(best["aligned"]["time_s"]),
        )

        deltas = []
        for i, m in enumerate(laps_with_aligned):
            if i == best_idx:
                continue
            cmp = AlignedLap(
                distance=np.array(m["aligned"]["distance"]),
                speed=np.array(m["aligned"]["speed"]),
                throttle=np.array(m["aligned"]["throttle"]),
                brake=np.array(m["aligned"]["brake"]),
                x=np.array(m["aligned"]["x"]),
                z=np.array(m["aligned"]["z"]),
                time_s=np.array(m["aligned"]["time_s"]),
            )
            td = compute_time_delta(ref, cmp)
            deltas.append({
                "lap_number": m.get("lap_number", i),
                "total_delta_s": td.total_delta_s,
                "ahead_pct": td.ahead_pct,
                "max_gain_m": td.max_gain_m,
                "max_loss_m": td.max_loss_m,
                "delta_curve": td.delta_s[::10].tolist(),
                "distance_curve": td.distance[::10].tolist(),
            })

        return deltas

    def run(self):
        brokers = os.environ.get("KAFKA_BROKERS", "localhost:9092")
        group = os.environ.get("KAFKA_GROUP", "line.analytics")
        topics = os.environ.get("KAFKA_TOPICS", "line.lap.written,line.session.complete").split(",")

        conf = {
            "bootstrap.servers": brokers,
            "group.id": group,
            "auto.offset.reset": "earliest",
            "enable.auto.commit": False,
        }
        consumer = Consumer(conf)
        consumer.subscribe(topics)

        def shutdown(sig, frame):
            self.running = False

        signal.signal(signal.SIGINT, shutdown)
        signal.signal(signal.SIGTERM, shutdown)

        logger.info("analytics worker started", extra={"topics": topics, "group": group})

        while self.running:
            msg = consumer.poll(1.0)
            if msg is None:
                continue
            if msg.error():
                if msg.error().code() == KafkaError._PARTITION_EOF:
                    continue
                logger.error("kafka error: %s", msg.error())
                continue

            try:
                event = json.loads(msg.value().decode())
                topic = msg.topic()

                if topic == "line.lap.written":
                    self.process_lap(event)
                elif topic == "line.session.complete":
                    self.process_session_complete(event)

                consumer.commit(msg)
            except Exception as e:
                logger.error("processing error: %s", e, exc_info=True)

        consumer.close()
        logger.info("analytics worker stopped")


if __name__ == "__main__":
    worker = AnalyticsWorker()
    worker.run()
