-- +goose Up
CREATE TABLE suburb_stats (
    id                  BIGSERIAL PRIMARY KEY,
    suburb              TEXT NOT NULL,
    state               TEXT NOT NULL,
    median_price        BIGINT,
    mean_price          BIGINT,
    sale_count          INT,
    median_yield_pct    NUMERIC(5,2),
    auction_clearance   NUMERIC(5,2),
    days_on_market      INT,
    school_score        NUMERIC(5,2),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(suburb, state)
);

CREATE TABLE domain_api_cache (
    id          BIGSERIAL PRIMARY KEY,
    endpoint    TEXT NOT NULL,
    params_hash TEXT NOT NULL,
    response    JSONB NOT NULL,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(endpoint, params_hash)
);

-- +goose Down
DROP TABLE IF EXISTS domain_api_cache;
DROP TABLE IF EXISTS suburb_stats;
