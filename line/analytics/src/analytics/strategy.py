import numpy as np
from dataclasses import dataclass


@dataclass
class TyreDegradation:
    avg_temp_per_lap: list[float]
    degradation_rate: float
    estimated_laps_remaining: int
    compound_guess: str
    front_rear_balance: float


@dataclass
class FuelStrategy:
    consumption_per_lap: float
    fuel_remaining: float
    laps_remaining: int
    optimal_pit_lap: int
    total_fuel_capacity: float


def compute_tyre_degradation(
    laps_tire_temps: list[dict],
    tire_radius_first: float = 0.0,
) -> TyreDegradation:
    if not laps_tire_temps:
        return TyreDegradation([], 0.0, 0, "unknown", 0.0)

    avg_temps = []
    front_temps = []
    rear_temps = []

    for lap in laps_tire_temps:
        fl = lap.get("tire_fl_temp", 0)
        fr = lap.get("tire_fr_temp", 0)
        rl = lap.get("tire_rl_temp", 0)
        rr = lap.get("tire_rr_temp", 0)
        avg_temps.append((fl + fr + rl + rr) / 4.0)
        front_temps.append((fl + fr) / 2.0)
        rear_temps.append((rl + rr) / 2.0)

    if len(avg_temps) >= 2:
        x = np.arange(len(avg_temps))
        coeffs = np.polyfit(x, avg_temps, 1)
        deg_rate = float(coeffs[0])
    else:
        deg_rate = 0.0

    critical_temp = 110.0
    if deg_rate > 0 and avg_temps[-1] < critical_temp:
        laps_left = int((critical_temp - avg_temps[-1]) / deg_rate)
    else:
        laps_left = 99

    compound = _guess_compound(tire_radius_first)

    front_avg = np.mean(front_temps) if front_temps else 0
    rear_avg = np.mean(rear_temps) if rear_temps else 0
    balance = float(front_avg - rear_avg)

    return TyreDegradation(
        avg_temp_per_lap=avg_temps,
        degradation_rate=deg_rate,
        estimated_laps_remaining=laps_left,
        compound_guess=compound,
        front_rear_balance=balance,
    )


def _guess_compound(radius: float) -> str:
    if radius <= 0:
        return "unknown"
    if radius < 0.29:
        return "soft"
    elif radius < 0.31:
        return "medium"
    else:
        return "hard"


def compute_fuel_strategy(
    fuel_per_lap: list[float],
    current_fuel: float,
    fuel_capacity: float,
    total_race_laps: int = 0,
) -> FuelStrategy:
    if not fuel_per_lap:
        return FuelStrategy(0.0, current_fuel, 0, 0, fuel_capacity)

    consumption = float(np.mean(fuel_per_lap))
    if consumption <= 0:
        return FuelStrategy(0.0, current_fuel, 99, 0, fuel_capacity)

    laps_remaining = int(current_fuel / consumption)

    if total_race_laps > 0 and laps_remaining < total_race_laps:
        fuel_needed = total_race_laps * consumption
        if fuel_needed > fuel_capacity:
            optimal_pit = int(fuel_capacity / consumption) - 1
        else:
            optimal_pit = 0
    else:
        optimal_pit = 0

    return FuelStrategy(
        consumption_per_lap=consumption,
        fuel_remaining=current_fuel,
        laps_remaining=laps_remaining,
        optimal_pit_lap=optimal_pit,
        total_fuel_capacity=fuel_capacity,
    )
