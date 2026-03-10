-- name: InsertSale :exec
INSERT INTO sales (
    district, property_id, unit_number, house_number, street, suburb, postcode,
    area, area_type, contract_date, settlement_date, price, zone, nature,
    purpose, strata_lot, dealing_number, source
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
) ON CONFLICT (dealing_number, property_id) DO NOTHING;

-- name: GetSalesBySuburb :many
SELECT * FROM sales
WHERE suburb = $1 AND nature = 'R'
ORDER BY contract_date DESC
LIMIT $2;

-- name: GetSalesBySuburbAndType :many
SELECT * FROM sales
WHERE suburb = $1
  AND nature = 'R'
  AND ($2::text IS NULL OR strata_lot IS NOT NULL = ($2 = 'strata'))
  AND contract_date >= $3
ORDER BY contract_date DESC;

-- name: GetSuburbMedianPrice :one
SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY price) AS median_price,
       count(*) AS sale_count,
       avg(price) AS mean_price
FROM sales
WHERE suburb = $1 AND nature = 'R' AND contract_date >= $2;

-- name: GetComparableSales :many
SELECT * FROM sales
WHERE suburb = $1
  AND nature = 'R'
  AND ($2::text IS NULL OR strata_lot IS NOT NULL = ($2 = 'strata'))
  AND contract_date >= $3
  AND price BETWEEN $4 AND $5
ORDER BY contract_date DESC
LIMIT $6;
