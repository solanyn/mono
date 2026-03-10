-- +goose Up
CREATE TABLE portfolio (
    id              BIGSERIAL PRIMARY KEY,
    address         TEXT NOT NULL,
    suburb          TEXT NOT NULL,
    postcode        TEXT,
    property_type   TEXT,
    bedrooms        INT,
    bathrooms       INT,
    purchase_price  BIGINT NOT NULL,
    purchase_date   DATE NOT NULL,
    current_rent_pw INT,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS portfolio;
