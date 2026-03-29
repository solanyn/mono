package store

import "context"

type GNAFAddress struct {
	GNAFPID      string   `json:"gnaf_pid"`
	StreetNumber *string  `json:"street_number"`
	StreetName   *string  `json:"street_name"`
	StreetType   *string  `json:"street_type"`
	Suburb       *string  `json:"suburb"`
	State        *string  `json:"state"`
	Postcode     *string  `json:"postcode"`
	Lat          *float64 `json:"lat"`
	Lon          *float64 `json:"lon"`
}

func (q *Queries) UpsertGNAFAddress(ctx context.Context, a GNAFAddress) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO gnaf_addresses (gnaf_pid, street_number, street_name, street_type, suburb, state, postcode, lat, lon)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (gnaf_pid) DO UPDATE SET
			street_number = EXCLUDED.street_number,
			street_name = EXCLUDED.street_name,
			street_type = EXCLUDED.street_type,
			suburb = EXCLUDED.suburb,
			state = EXCLUDED.state,
			postcode = EXCLUDED.postcode,
			lat = EXCLUDED.lat,
			lon = EXCLUDED.lon`,
		a.GNAFPID, a.StreetNumber, a.StreetName, a.StreetType,
		a.Suburb, a.State, a.Postcode, a.Lat, a.Lon,
	)
	return err
}

func (q *Queries) LookupAddress(ctx context.Context, suburb, streetName string, streetNumber *string) ([]GNAFAddress, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT gnaf_pid, street_number, street_name, street_type, suburb, state, postcode, lat, lon
		FROM gnaf_addresses
		WHERE suburb = $1 AND street_name = $2
		  AND ($3::text IS NULL OR street_number = $3)
		LIMIT 10`,
		suburb, streetName, streetNumber,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addrs []GNAFAddress
	for rows.Next() {
		var a GNAFAddress
		if err := rows.Scan(&a.GNAFPID, &a.StreetNumber, &a.StreetName, &a.StreetType,
			&a.Suburb, &a.State, &a.Postcode, &a.Lat, &a.Lon); err != nil {
			return nil, err
		}
		addrs = append(addrs, a)
	}
	return addrs, rows.Err()
}

func (q *Queries) GetAddressByPID(ctx context.Context, pid string) (GNAFAddress, error) {
	var a GNAFAddress
	err := q.pool.QueryRow(ctx,
		`SELECT gnaf_pid, street_number, street_name, street_type, suburb, state, postcode, lat, lon
		FROM gnaf_addresses WHERE gnaf_pid = $1`, pid,
	).Scan(&a.GNAFPID, &a.StreetNumber, &a.StreetName, &a.StreetType,
		&a.Suburb, &a.State, &a.Postcode, &a.Lat, &a.Lon)
	return a, err
}
