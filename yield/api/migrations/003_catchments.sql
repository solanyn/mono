-- +goose Up
CREATE TABLE school_catchments (
    id          BIGSERIAL PRIMARY KEY,
    use_id      TEXT NOT NULL,
    catch_type  TEXT NOT NULL,
    school_name TEXT NOT NULL,
    priority    INT,
    geom        GEOMETRY(MultiPolygon, 4326),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(use_id, catch_type)
);

CREATE INDEX idx_catchments_geom ON school_catchments USING GIST(geom);

-- +goose Down
DROP TABLE IF EXISTS school_catchments;
