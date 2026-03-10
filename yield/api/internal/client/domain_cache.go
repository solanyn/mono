package client

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type CacheTTL struct {
	ListingsSearch  time.Duration
	SuburbStats     time.Duration
	PropertyProfile time.Duration
	PropertySuggest time.Duration
}

var DefaultCacheTTL = CacheTTL{
	ListingsSearch:  24 * time.Hour,
	SuburbStats:     7 * 24 * time.Hour,
	PropertyProfile: 30 * 24 * time.Hour,
	PropertySuggest: 7 * 24 * time.Hour,
}

type DomainCache struct {
	rdb  *redis.Client
	pool *pgxpool.Pool
	ttl  CacheTTL
}

func NewDomainCache(rdb *redis.Client, pool *pgxpool.Pool) *DomainCache {
	return &DomainCache{
		rdb:  rdb,
		pool: pool,
		ttl:  DefaultCacheTTL,
	}
}

func (c *DomainCache) Get(ctx context.Context, endpoint string, params map[string]string) ([]byte, bool, error) {
	key := cacheKey(endpoint, params)

	val, err := c.rdb.Get(ctx, key).Bytes()
	if err == nil {
		return val, true, nil
	}

	row := c.pool.QueryRow(ctx,
		`SELECT response FROM domain_api_cache WHERE endpoint = $1 AND params_hash = $2`,
		endpoint, paramsHash(params),
	)
	var response []byte
	if err := row.Scan(&response); err == nil {
		return response, false, nil
	}

	return nil, false, nil
}

func (c *DomainCache) Put(ctx context.Context, endpoint string, params map[string]string, data []byte) error {
	key := cacheKey(endpoint, params)
	ttl := c.ttlFor(endpoint)

	if err := c.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("dragonfly set: %w", err)
	}

	_, err := c.pool.Exec(ctx,
		`INSERT INTO domain_api_cache (endpoint, params_hash, response)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (endpoint, params_hash) DO UPDATE SET
		     response = EXCLUDED.response, fetched_at = NOW()`,
		endpoint, paramsHash(params), data,
	)
	if err != nil {
		return fmt.Errorf("postgres cache: %w", err)
	}

	return nil
}

func (c *DomainCache) ttlFor(endpoint string) time.Duration {
	switch endpoint {
	case "listings_search":
		return c.ttl.ListingsSearch
	case "suburb_stats":
		return c.ttl.SuburbStats
	case "property_profile":
		return c.ttl.PropertyProfile
	case "property_suggest":
		return c.ttl.PropertySuggest
	default:
		return 24 * time.Hour
	}
}

func cacheKey(endpoint string, params map[string]string) string {
	return fmt.Sprintf("domain:cache:%s:%s", endpoint, paramsHash(params))
}

func paramsHash(params map[string]string) string {
	b, _ := json.Marshal(params)
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h[:8])
}
