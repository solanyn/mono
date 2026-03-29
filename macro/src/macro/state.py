import json
import os
import uuid
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone

DEFAULT_STATE_PATH = os.environ.get("MACRO_STATE_PATH", "macro-state.json")


@dataclass
class CyclePosition:
    phase: str = "unknown"
    confidence: float = 0.0
    rationale: str = ""
    last_reassessed: str = ""


@dataclass
class TrackedNarrative:
    id: str = ""
    claim: str = ""
    source: str = ""
    source_date: str = ""
    data_check: str = ""
    next_data_release: str | None = None
    status: str = "pending_verification"
    agent_prior: str = ""


@dataclass
class AgentState:
    last_updated: str = ""
    cycle_position: CyclePosition = field(default_factory=CyclePosition)
    tracked_narratives: list[TrackedNarrative] = field(default_factory=list)
    key_levels: dict[str, float] = field(default_factory=dict)
    last_digest: str | None = None
    divergences_active: list[dict] = field(default_factory=list)
    lessons: list[str] = field(default_factory=list)


def _default_state() -> AgentState:
    return AgentState(
        last_updated=datetime.now(timezone.utc).isoformat(),
        cycle_position=CyclePosition(
            phase="unknown",
            confidence=0.0,
            rationale="No assessment yet — awaiting first data review.",
            last_reassessed="",
        ),
        key_levels={
            "cash_rate_target": 0.0,
            "cpi_annual": 0.0,
        },
    )


def load_state(path: str = DEFAULT_STATE_PATH) -> AgentState:
    if not os.path.exists(path):
        state = _default_state()
        save_state(state, path)
        return state
    with open(path) as f:
        data = json.load(f)
    cp = data.get("cycle_position", {})
    narratives = [TrackedNarrative(**n) for n in data.get("tracked_narratives", [])]
    return AgentState(
        last_updated=data.get("last_updated", ""),
        cycle_position=CyclePosition(**cp) if cp else CyclePosition(),
        tracked_narratives=narratives,
        key_levels=data.get("key_levels", {}),
        last_digest=data.get("last_digest"),
        divergences_active=data.get("divergences_active", []),
        lessons=data.get("lessons", []),
    )


def save_state(state: AgentState, path: str = DEFAULT_STATE_PATH) -> None:
    state.last_updated = datetime.now(timezone.utc).isoformat()
    with open(path, "w") as f:
        json.dump(asdict(state), f, indent=2)


def update_cycle_position(
    phase: str,
    confidence: float,
    rationale: str,
    path: str = DEFAULT_STATE_PATH,
) -> AgentState:
    state = load_state(path)
    state.cycle_position = CyclePosition(
        phase=phase,
        confidence=confidence,
        rationale=rationale,
        last_reassessed=datetime.now(timezone.utc).isoformat(),
    )
    save_state(state, path)
    return state


def track_narrative(
    claim: str,
    source: str,
    data_check: str,
    source_date: str = "",
    next_data_release: str | None = None,
    agent_prior: str = "",
    path: str = DEFAULT_STATE_PATH,
) -> TrackedNarrative:
    state = load_state(path)
    narrative = TrackedNarrative(
        id=str(uuid.uuid4())[:8],
        claim=claim,
        source=source,
        source_date=source_date or datetime.now(timezone.utc).strftime("%Y-%m-%d"),
        data_check=data_check,
        next_data_release=next_data_release,
        status="pending_verification",
        agent_prior=agent_prior,
    )
    state.tracked_narratives.append(narrative)
    save_state(state, path)
    return narrative


def update_narrative(
    narrative_id: str,
    status: str,
    evidence: str = "",
    path: str = DEFAULT_STATE_PATH,
) -> TrackedNarrative | None:
    state = load_state(path)
    for n in state.tracked_narratives:
        if n.id == narrative_id:
            n.status = status
            if evidence:
                n.agent_prior = evidence
            save_state(state, path)
            return n
    return None
