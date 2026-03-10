-- name: UpsertGNAFAddress :exec
INSERT INTO gnaf_addresses (gnaf_pid, street_number, street_name, street_type, suburb, state, postcode, lat, lon)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (gnaf_pid) DO UPDATE SET
    street_number = EXCLUDED.street_number,
    street_name = EXCLUDED.street_name,
    street_type = EXCLUDED.street_type,
    suburb = EXCLUDED.suburb,
    state = EXCLUDED.state,
    postcode = EXCLUDED.postcode,
    lat = EXCLUDED.lat,
    lon = EXCLUDED.lon;

-- name: LookupAddress :many
SELECT * FROM gnaf_addresses
WHERE suburb = $1 AND street_name = $2
  AND ($3::text IS NULL OR street_number = $3)
LIMIT 10;

-- name: GetAddressByPID :one
SELECT * FROM gnaf_addresses WHERE gnaf_pid = $1;
