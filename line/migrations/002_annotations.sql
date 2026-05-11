-- +goose Up
CREATE TABLE annotations (
    id          BIGSERIAL PRIMARY KEY,
    session_id  TEXT NOT NULL REFERENCES sessions(id),
    lap_number  INTEGER NOT NULL,
    frame_idx   INTEGER NOT NULL,
    text        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_annotations_session_lap ON annotations(session_id, lap_number);

-- +goose Down
DROP TABLE IF EXISTS annotations;
