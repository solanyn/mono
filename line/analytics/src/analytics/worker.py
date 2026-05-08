import io
import json
import logging
import os
import signal
import sys
import time
from dataclasses import asdict

import boto3
import pyarrow.parquet as pq
from confluent_kafka import Consumer, KafkaError

from analytics.silver import compute_lap_metrics
from analytics.gold import compute_consistency
from analytics.corners import detect_corners
from analytics.tracks import TrackDatabase
from analytics.mastery import score_corners
from analytics.strategy import compute_tyre_degradation, compute_fuel_strategy
from analytics.journal import generate_journal

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
            x = df["pos_x"].values
            z = df["pos_z"].values
            speed = df["speed"].values if "speed" in df.columns else None

            track = self.track_db.identify(x, z)
            if track:
                metrics["track_id"] = track.track_id
                metrics["track_name"] = track.name

            if speed is not None:
                corners = detect_corners(x.astype(float), z.astype(float), speed.astype(float))
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

        gold = {
            "session_id": session_id,
            "car_code": car_code,
            "track_name": track_name,
            "lap_count": lap_count,
            "consistency": consistency,
            "tyre_degradation": asdict(tyre_deg),
            "fuel_strategy": asdict(fuel_strategy),
            "journal": asdict(journal),
        }

        gold_key = f"sessions/{session_id}/summary.json"
        self.s3.put_object(
            Bucket=self.gold_bucket,
            Key=gold_key,
            Body=json.dumps(gold, default=str).encode(),
            ContentType="application/json",
        )
        logger.info("wrote gold summary", extra={"key": gold_key})

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
