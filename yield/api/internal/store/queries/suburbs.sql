-- name: UpsertSuburbStats :exec
INSERT INTO suburb_stats (suburb, state, median_price, mean_price, sale_count, median_yield_pct, auction_clearance, days_on_market, school_score)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (suburb, state) DO UPDATE SET
    median_price = EXCLUDED.median_price,
    mean_price = EXCLUDED.mean_price,
    sale_count = EXCLUDED.sale_count,
    median_yield_pct = EXCLUDED.median_yield_pct,
    auction_clearance = EXCLUDED.auction_clearance,
    days_on_market = EXCLUDED.days_on_market,
    school_score = EXCLUDED.school_score,
    updated_at = NOW();

-- name: GetSuburbStats :one
SELECT * FROM suburb_stats WHERE suburb = $1 AND state = $2;

-- name: UpsertDomainCache :exec
INSERT INTO domain_api_cache (endpoint, params_hash, response)
VALUES ($1, $2, $3)
ON CONFLICT (endpoint, params_hash) DO UPDATE SET
    response = EXCLUDED.response,
    fetched_at = NOW();

-- name: GetDomainCache :one
SELECT response, fetched_at FROM domain_api_cache
WHERE endpoint = $1 AND params_hash = $2;
