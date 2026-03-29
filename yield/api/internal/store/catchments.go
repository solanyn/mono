package store

import "context"

func (q *Queries) UpsertCatchment(ctx context.Context, useID, catchType, schoolName string, priority int, geomWKB []byte) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO school_catchments (use_id, catch_type, school_name, priority, geom)
		VALUES ($1, $2, $3, $4, ST_GeomFromWKB($5, 4326))
		ON CONFLICT (use_id, catch_type) DO UPDATE SET
			school_name = EXCLUDED.school_name,
			priority = EXCLUDED.priority,
			geom = EXCLUDED.geom`,
		useID, catchType, schoolName, priority, geomWKB,
	)
	return err
}
