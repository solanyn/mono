-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE sales (
    id              BIGSERIAL PRIMARY KEY,
    district        TEXT NOT NULL,
    property_id     TEXT,
    unit_number     TEXT,
    house_number    TEXT,
    street          TEXT,
    suburb          TEXT NOT NULL,
    postcode        TEXT,
    area            NUMERIC,
    area_type       TEXT,
    contract_date   DATE,
    settlement_date DATE,
    price           BIGINT,
    zone            TEXT,
    nature          TEXT,
    purpose         TEXT,
    strata_lot      TEXT,
    dealing_number  TEXT,
    source          TEXT NOT NULL DEFAULT 'nsw_vg',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(dealing_number, property_id)
);

CREATE INDEX idx_sales_suburb ON sales(suburb);
CREATE INDEX idx_sales_contract_date ON sales(contract_date);
CREATE INDEX idx_sales_postcode ON sales(postcode);

-- +goose Down
DROP TABLE IF EXISTS sales;
