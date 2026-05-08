-- +goose Up
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    car_code    INTEGER NOT NULL,
    track_id    TEXT,
    track_name  TEXT,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at    TIMESTAMPTZ,
    lap_count   INTEGER NOT NULL DEFAULT 0,
    best_lap_ms INTEGER,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE laps (
    id          BIGSERIAL PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES sessions(id),
    lap_number  INTEGER NOT NULL,
    time_ms     INTEGER,
    frames      INTEGER NOT NULL DEFAULT 0,
    top_speed   REAL,
    s3_key      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(session_id, lap_number)
);

CREATE INDEX idx_laps_session ON laps(session_id);

CREATE TABLE tracks (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    fingerprint TEXT,
    country     TEXT,
    length_m    REAL,
    corners     INTEGER,
    source      TEXT NOT NULL DEFAULT 'community',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS laps;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS tracks;
