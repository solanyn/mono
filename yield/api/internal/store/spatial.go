package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/solanyn/mono/yield/api/internal/domain"
)

type SpatialStore struct {
	pool *pgxpool.Pool
}

func NewSpatialStore(pool *pgxpool.Pool) *SpatialStore {
	return &SpatialStore{pool: pool}
}

func (s *SpatialStore) CatchmentsForPoint(ctx context.Context, lon, lat float64) ([]domain.SchoolCatchment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT use_id, catch_type, school_name, priority
		 FROM school_catchments
		 WHERE ST_Contains(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326))`,
		lon, lat,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var catchments []domain.SchoolCatchment
	for rows.Next() {
		var c domain.SchoolCatchment
		if err := rows.Scan(&c.UseID, &c.CatchType, &c.School, &c.Priority); err != nil {
			return nil, err
		}
		catchments = append(catchments, c)
	}
	return catchments, rows.Err()
}

func (s *SpatialStore) NearbyAddresses(ctx context.Context, lon, lat float64, radiusMeters int) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT gnaf_pid FROM gnaf_addresses
		 WHERE ST_DWithin(
		     ST_SetSRID(ST_MakePoint(lon, lat), 4326)::geography,
		     ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
		     $3
		 )
		 LIMIT 100`,
		lon, lat, radiusMeters,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pids []string
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err != nil {
			return nil, err
		}
		pids = append(pids, pid)
	}
	return pids, rows.Err()
}
