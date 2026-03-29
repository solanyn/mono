import json
import os
import tempfile

from macro.state import (
    AgentState,
    CyclePosition,
    TrackedNarrative,
    load_state,
    save_state,
    update_cycle_position,
    track_narrative,
    update_narrative,
)


def test_default_state_initialization():
    with tempfile.NamedTemporaryFile(suffix=".json", delete=False) as f:
        path = f.name
    os.unlink(path)
    try:
        state = load_state(path)
        assert state.cycle_position.phase == "unknown"
        assert state.cycle_position.confidence == 0.0
        assert isinstance(state.tracked_narratives, list)
        assert len(state.tracked_narratives) == 0
        assert "cash_rate_target" in state.key_levels
        assert os.path.exists(path)
    finally:
        if os.path.exists(path):
            os.unlink(path)


def test_state_round_trip():
    with tempfile.NamedTemporaryFile(suffix=".json", delete=False) as f:
        path = f.name
    try:
        state = AgentState(
            last_updated="2026-01-01T00:00:00",
            cycle_position=CyclePosition(
                phase="early_easing",
                confidence=0.8,
                rationale="Two cuts delivered",
                last_reassessed="2026-01-01",
            ),
            key_levels={"cash_rate_target": 3.85},
        )
        save_state(state, path)
        loaded = load_state(path)
        assert loaded.cycle_position.phase == "early_easing"
        assert loaded.cycle_position.confidence == 0.8
        assert loaded.key_levels["cash_rate_target"] == 3.85
    finally:
        os.unlink(path)


def test_update_cycle_position():
    with tempfile.NamedTemporaryFile(suffix=".json", delete=False) as f:
        path = f.name
    os.unlink(path)
    try:
        load_state(path)
        state = update_cycle_position("mid_easing", 0.6, "Third cut expected", path)
        assert state.cycle_position.phase == "mid_easing"
        assert state.cycle_position.confidence == 0.6
        reloaded = load_state(path)
        assert reloaded.cycle_position.phase == "mid_easing"
    finally:
        if os.path.exists(path):
            os.unlink(path)


def test_track_and_update_narrative():
    with tempfile.NamedTemporaryFile(suffix=".json", delete=False) as f:
        path = f.name
    os.unlink(path)
    try:
        load_state(path)
        n = track_narrative(
            claim="Housing cooling rapidly",
            source="user",
            data_check="CoreLogic HVI",
            agent_prior="Skeptical",
            path=path,
        )
        assert n.status == "pending_verification"
        assert n.claim == "Housing cooling rapidly"
        assert len(n.id) > 0

        updated = update_narrative(n.id, "refuted", "Prices still rising", path)
        assert updated is not None
        assert updated.status == "refuted"

        state = load_state(path)
        assert len(state.tracked_narratives) == 1
        assert state.tracked_narratives[0].status == "refuted"
    finally:
        if os.path.exists(path):
            os.unlink(path)


def test_update_nonexistent_narrative():
    with tempfile.NamedTemporaryFile(suffix=".json", delete=False) as f:
        path = f.name
    os.unlink(path)
    try:
        load_state(path)
        result = update_narrative("nonexistent", "confirmed", path=path)
        assert result is None
    finally:
        if os.path.exists(path):
            os.unlink(path)
