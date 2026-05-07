import math

import numpy as np
import pyarrow.parquet as pq


def compute_lap_metrics(parquet_bytes: bytes) -> dict:
    import io

    table = pq.read_table(io.BytesIO(parquet_bytes))
    df = table.to_pandas()

    x = df["pos_x"].values
    y = df["pos_y"].values
    z = df["pos_z"].values
    dx = np.diff(x)
    dy = np.diff(y)
    dz = np.diff(z)
    distances = np.sqrt(dx**2 + dy**2 + dz**2)
    total_distance = float(np.sum(distances))

    speed = df["speed"].values
    throttle = df["throttle"].values
    brake = df["brake"].values
    rpm = df["rpm"].values

    brake_threshold = 10
    brake_on = brake > brake_threshold
    brake_starts = np.where(np.diff(brake_on.astype(int)) == 1)[0]

    throttle_pct = float(np.mean(throttle > 10)) * 100
    coast_pct = float(np.mean((throttle <= 10) & (brake <= brake_threshold))) * 100
    brake_pct = float(np.mean(brake_on)) * 100

    tire_cols = ["tire_fl_temp", "tire_fr_temp", "tire_rl_temp", "tire_rr_temp"]
    tire_temps = {col: float(df[col].mean()) for col in tire_cols if col in df.columns}

    fuel = df["fuel_level"].values
    fuel_used = float(fuel[0] - fuel[-1]) if len(fuel) > 1 else 0.0

    return {
        "total_distance_m": total_distance,
        "top_speed": float(np.max(speed)),
        "avg_speed": float(np.mean(speed)),
        "min_speed": float(np.min(speed[speed > 0])) if np.any(speed > 0) else 0.0,
        "max_rpm": float(np.max(rpm)),
        "brake_count": int(len(brake_starts)),
        "throttle_pct": throttle_pct,
        "coast_pct": coast_pct,
        "brake_pct": brake_pct,
        "avg_tire_temps": tire_temps,
        "fuel_used": fuel_used,
        "frame_count": len(df),
    }
