-- +goose Up
CREATE TABLE gnaf_addresses (
    gnaf_pid        TEXT PRIMARY KEY,
    street_number   TEXT,
    street_name     TEXT,
    street_type     TEXT,
    suburb          TEXT NOT NULL,
    state           TEXT NOT NULL,
    postcode        TEXT,
    lat             DOUBLE PRECISION,
    lon             DOUBLE PRECISION,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gnaf_suburb ON gnaf_addresses(suburb, street_name, street_number);
CREATE INDEX idx_gnaf_postcode ON gnaf_addresses(postcode);

-- +goose Down
DROP TABLE IF EXISTS gnaf_addresses;
