package store

import (
	"context"
	"time"
)

type ListingSnapshot struct {
	ID           int64      `json:"id"`
	ListingID    int64      `json:"listing_id"`
	SnapshotAt   time.Time  `json:"snapshot_at"`
	BlobKey      string     `json:"blob_key"`
	ListingType  string     `json:"listing_type"`
	Status       *string    `json:"status"`
	Suburb       string     `json:"suburb"`
	Postcode     *string    `json:"postcode"`
	PriceDisplay *string    `json:"price_display"`
	PriceNumeric *float64   `json:"price_numeric"`
	Bedrooms     *int16     `json:"bedrooms"`
	Bathrooms    *int16     `json:"bathrooms"`
	Carspaces    *int16     `json:"carspaces"`
	PropertyType *string    `json:"property_type"`
	LandArea     *float64   `json:"land_area"`
	Description  *string    `json:"description"`
	Headline     *string    `json:"headline"`
	PhotosCount  *int16     `json:"photos_count"`
	AgentName    *string    `json:"agent_name"`
	AgentID      *int32     `json:"agent_id"`
	DateListed   *time.Time `json:"date_listed"`
	DaysListed   *int32     `json:"days_listed"`
	Lat          *float64   `json:"lat"`
	Lon          *float64   `json:"lon"`
}

func (q *Queries) InsertListingSnapshot(ctx context.Context, l ListingSnapshot) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO listing_snapshots (
			listing_id, snapshot_at, blob_key, listing_type, status, suburb, postcode,
			price_display, price_numeric, bedrooms, bathrooms, carspaces, property_type,
			land_area, description, headline, photos_count, agent_name, agent_id,
			date_listed, days_listed, lat, lon
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23
		) ON CONFLICT (listing_id, snapshot_at) DO NOTHING`,
		l.ListingID, l.SnapshotAt, l.BlobKey, l.ListingType, l.Status, l.Suburb, l.Postcode,
		l.PriceDisplay, l.PriceNumeric, l.Bedrooms, l.Bathrooms, l.Carspaces, l.PropertyType,
		l.LandArea, l.Description, l.Headline, l.PhotosCount, l.AgentName, l.AgentID,
		l.DateListed, l.DaysListed, l.Lat, l.Lon,
	)
	return err
}

func (q *Queries) GetRentalListingsBySuburb(ctx context.Context, suburb string, limit int) ([]ListingSnapshot, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, listing_id, snapshot_at, blob_key, listing_type, status, suburb, postcode,
			price_display, price_numeric, bedrooms, bathrooms, carspaces, property_type,
			land_area, description, headline, photos_count, agent_name, agent_id,
			date_listed, days_listed, lat, lon
		FROM listing_snapshots
		WHERE suburb = $1 AND listing_type = 'rent'
		  AND snapshot_at = (
			  SELECT MAX(snapshot_at) FROM listing_snapshots ls2
			  WHERE ls2.listing_id = listing_snapshots.listing_id
		  )
		ORDER BY snapshot_at DESC
		LIMIT $2`,
		suburb, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanListings(rows)
}

func (q *Queries) GetListingsBySuburbAndBeds(ctx context.Context, suburb string, listingType string, bedrooms *int16, limit int) ([]ListingSnapshot, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, listing_id, snapshot_at, blob_key, listing_type, status, suburb, postcode,
			price_display, price_numeric, bedrooms, bathrooms, carspaces, property_type,
			land_area, description, headline, photos_count, agent_name, agent_id,
			date_listed, days_listed, lat, lon
		FROM listing_snapshots
		WHERE suburb = $1
		  AND listing_type = $2
		  AND ($3::smallint IS NULL OR bedrooms = $3)
		  AND snapshot_at = (
			  SELECT MAX(snapshot_at) FROM listing_snapshots ls2
			  WHERE ls2.listing_id = listing_snapshots.listing_id
		  )
		ORDER BY snapshot_at DESC
		LIMIT $4`,
		suburb, listingType, bedrooms, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanListings(rows)
}

func (q *Queries) InsertPropertySnapshot(ctx context.Context, propertyID string, snapshotAt time.Time, blobKey, suburb string, saleCount *int16, lastSalePrice *float64, lastSaleDate *time.Time) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO property_snapshots (
			property_id, snapshot_at, blob_key, suburb, sale_count, last_sale_price, last_sale_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (property_id, snapshot_at) DO NOTHING`,
		propertyID, snapshotAt, blobKey, suburb, saleCount, lastSalePrice, lastSaleDate,
	)
	return err
}

func scanListings(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]ListingSnapshot, error) {
	var listings []ListingSnapshot
	for rows.Next() {
		var l ListingSnapshot
		if err := rows.Scan(
			&l.ID, &l.ListingID, &l.SnapshotAt, &l.BlobKey, &l.ListingType, &l.Status,
			&l.Suburb, &l.Postcode, &l.PriceDisplay, &l.PriceNumeric, &l.Bedrooms, &l.Bathrooms,
			&l.Carspaces, &l.PropertyType, &l.LandArea, &l.Description, &l.Headline,
			&l.PhotosCount, &l.AgentName, &l.AgentID, &l.DateListed, &l.DaysListed,
			&l.Lat, &l.Lon,
		); err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}
