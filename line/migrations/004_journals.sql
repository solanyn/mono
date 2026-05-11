-- +goose Up
CREATE TABLE journals (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(session_id)
);

CREATE INDEX idx_journals_session ON journals(session_id);

-- +goose Down
DROP TABLE IF EXISTS journals;
