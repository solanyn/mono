-- name: InsertListingSnapshot :exec
INSERT INTO listing_snapshots (
    listing_id, snapshot_at, blob_key, listing_type, status, suburb, postcode,
    price_display, price_numeric, bedrooms, bathrooms, carspaces, property_type,
    land_area, description, headline, photos_count, agent_name, agent_id,
    date_listed, days_listed, lat, lon
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
    $17, $18, $19, $20, $21, $22, $23
) ON CONFLICT (listing_id, snapshot_at) DO NOTHING;

-- name: GetRentalListingsBySuburb :many
SELECT * FROM listing_snapshots
WHERE suburb = $1 AND listing_type = 'rent'
  AND snapshot_at = (
      SELECT MAX(snapshot_at) FROM listing_snapshots ls2
      WHERE ls2.listing_id = listing_snapshots.listing_id
  )
ORDER BY snapshot_at DESC
LIMIT $2;

-- name: GetListingsBySuburbAndBeds :many
SELECT * FROM listing_snapshots
WHERE suburb = $1
  AND listing_type = $2
  AND ($3::smallint IS NULL OR bedrooms = $3)
  AND snapshot_at = (
      SELECT MAX(snapshot_at) FROM listing_snapshots ls2
      WHERE ls2.listing_id = listing_snapshots.listing_id
  )
ORDER BY snapshot_at DESC
LIMIT $4;

-- name: InsertPropertySnapshot :exec
INSERT INTO property_snapshots (
    property_id, snapshot_at, blob_key, suburb, sale_count, last_sale_price, last_sale_date
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (property_id, snapshot_at) DO NOTHING;
