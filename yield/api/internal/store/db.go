package store

import "github.com/jackc/pgx/v5/pgxpool"

type Queries struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Queries {
	return &Queries{pool: pool}
}

func (q *Queries) Pool() *pgxpool.Pool {
	return q.pool
}
