-- name: UpsertCatchment :exec
INSERT INTO school_catchments (use_id, catch_type, school_name, priority, geom)
VALUES ($1, $2, $3, $4, ST_GeomFromWKB($5, 4326))
ON CONFLICT (use_id, catch_type) DO UPDATE SET
    school_name = EXCLUDED.school_name,
    priority = EXCLUDED.priority,
    geom = EXCLUDED.geom;

-- name: GetCatchmentsForPoint :many
SELECT use_id, catch_type, school_name, priority
FROM school_catchments
WHERE ST_Contains(geom, ST_SetSRID(ST_MakePoint($1, $2), 4326));
