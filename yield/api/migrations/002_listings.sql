-- +goose Up
CREATE TABLE listing_snapshots (
    id              BIGSERIAL PRIMARY KEY,
    listing_id      BIGINT NOT NULL,
    snapshot_at     TIMESTAMPTZ NOT NULL,
    blob_key        TEXT NOT NULL,
    listing_type    TEXT NOT NULL,
    status          TEXT,
    suburb          TEXT NOT NULL,
    postcode        TEXT,
    price_display   TEXT,
    price_numeric   NUMERIC,
    bedrooms        SMALLINT,
    bathrooms       SMALLINT,
    carspaces       SMALLINT,
    property_type   TEXT,
    land_area       NUMERIC,
    description     TEXT,
    headline        TEXT,
    photos_count    SMALLINT,
    agent_name      TEXT,
    agent_id        INT,
    date_listed     DATE,
    days_listed     INT,
    lat             DOUBLE PRECISION,
    lon             DOUBLE PRECISION,
    UNIQUE(listing_id, snapshot_at)
);

CREATE INDEX idx_listing_snap_suburb ON listing_snapshots(suburb, listing_type);
CREATE INDEX idx_listing_snap_listing ON listing_snapshots(listing_id);
CREATE INDEX idx_listing_snap_desc ON listing_snapshots USING gin(to_tsvector('english', description));

CREATE TABLE property_snapshots (
    id              BIGSERIAL PRIMARY KEY,
    property_id     TEXT NOT NULL,
    snapshot_at     TIMESTAMPTZ NOT NULL,
    blob_key        TEXT NOT NULL,
    suburb          TEXT NOT NULL,
    sale_count      SMALLINT,
    last_sale_price NUMERIC,
    last_sale_date  DATE,
    UNIQUE(property_id, snapshot_at)
);

-- +goose Down
DROP TABLE IF EXISTS property_snapshots;
DROP TABLE IF EXISTS listing_snapshots;
