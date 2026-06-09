from __future__ import annotations

import httpx

CALC_URL = "http://vgc-calc.default.svc.cluster.local:8080"


def _base_url() -> str:
    import os

    return os.environ.get("VGC_CALC_URL", CALC_URL)


def run_calc(
    attacker: dict,
    defender: dict,
    move: str,
    field: dict | None = None,
) -> dict:
    payload = {"attacker": attacker, "defender": defender, "move": move, "field": field or {}}
    resp = httpx.post(f"{_base_url()}/calc", json=payload, timeout=10)
    resp.raise_for_status()
    return resp.json()


def bulk_calc(calcs: list[dict]) -> list[dict]:
    resp = httpx.post(f"{_base_url()}/calc", json=calcs, timeout=60)
    resp.raise_for_status()
    return resp.json()
