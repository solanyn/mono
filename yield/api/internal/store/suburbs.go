package store

import (
	"context"
	"time"
)

type SuburbStatsRow struct {
	ID               int64     `json:"id"`
	Suburb           string    `json:"suburb"`
	State            string    `json:"state"`
	MedianPrice      *int64    `json:"median_price"`
	MeanPrice        *int64    `json:"mean_price"`
	SaleCount        *int32    `json:"sale_count"`
	MedianYieldPct   *float64  `json:"median_yield_pct"`
	AuctionClearance *float64  `json:"auction_clearance"`
	DaysOnMarket     *int32    `json:"days_on_market"`
	SchoolScore      *float64  `json:"school_score"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (q *Queries) UpsertSuburbStats(ctx context.Context, s SuburbStatsRow) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO suburb_stats (suburb, state, median_price, mean_price, sale_count, median_yield_pct, auction_clearance, days_on_market, school_score)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (suburb, state) DO UPDATE SET
			median_price = EXCLUDED.median_price,
			mean_price = EXCLUDED.mean_price,
			sale_count = EXCLUDED.sale_count,
			median_yield_pct = EXCLUDED.median_yield_pct,
			auction_clearance = EXCLUDED.auction_clearance,
			days_on_market = EXCLUDED.days_on_market,
			school_score = EXCLUDED.school_score,
			updated_at = NOW()`,
		s.Suburb, s.State, s.MedianPrice, s.MeanPrice, s.SaleCount,
		s.MedianYieldPct, s.AuctionClearance, s.DaysOnMarket, s.SchoolScore,
	)
	return err
}

func (q *Queries) GetSuburbStats(ctx context.Context, suburb, state string) (SuburbStatsRow, error) {
	var s SuburbStatsRow
	err := q.pool.QueryRow(ctx,
		`SELECT id, suburb, state, median_price, mean_price, sale_count, median_yield_pct, auction_clearance, days_on_market, school_score, updated_at
		FROM suburb_stats WHERE suburb = $1 AND state = $2`,
		suburb, state,
	).Scan(&s.ID, &s.Suburb, &s.State, &s.MedianPrice, &s.MeanPrice, &s.SaleCount,
		&s.MedianYieldPct, &s.AuctionClearance, &s.DaysOnMarket, &s.SchoolScore, &s.UpdatedAt)
	return s, err
}

func (q *Queries) UpsertDomainCache(ctx context.Context, endpoint, paramsHash string, response []byte) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO domain_api_cache (endpoint, params_hash, response)
		VALUES ($1, $2, $3)
		ON CONFLICT (endpoint, params_hash) DO UPDATE SET
			response = EXCLUDED.response,
			fetched_at = NOW()`,
		endpoint, paramsHash, response,
	)
	return err
}

type DomainCacheRow struct {
	Response  []byte    `json:"response"`
	FetchedAt time.Time `json:"fetched_at"`
}

func (q *Queries) GetDomainCache(ctx context.Context, endpoint, paramsHash string) (DomainCacheRow, error) {
	var r DomainCacheRow
	err := q.pool.QueryRow(ctx,
		`SELECT response, fetched_at FROM domain_api_cache WHERE endpoint = $1 AND params_hash = $2`,
		endpoint, paramsHash,
	).Scan(&r.Response, &r.FetchedAt)
	return r, err
}
