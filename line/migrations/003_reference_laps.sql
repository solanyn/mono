-- +goose Up
CREATE TABLE reference_laps (
    id          BIGSERIAL PRIMARY KEY,
    track_id    TEXT NOT NULL,
    car_code    INTEGER NOT NULL,
    session_id  TEXT NOT NULL REFERENCES sessions(id),
    lap_number  INTEGER NOT NULL,
    time_ms     INTEGER NOT NULL,
    s3_key      TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT 'best',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(track_id, car_code, label)
);

CREATE INDEX idx_reference_laps_track_car ON reference_laps(track_id, car_code);

-- +goose Down
DROP TABLE IF EXISTS reference_laps;
