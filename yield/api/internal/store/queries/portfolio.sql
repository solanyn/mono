-- name: InsertPortfolioProperty :one
INSERT INTO portfolio (address, suburb, postcode, property_type, bedrooms, bathrooms, purchase_price, purchase_date, current_rent_pw, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetPortfolio :many
SELECT * FROM portfolio ORDER BY created_at;

-- name: GetPortfolioProperty :one
SELECT * FROM portfolio WHERE id = $1;

-- name: UpdatePortfolioRent :exec
UPDATE portfolio SET current_rent_pw = $2, updated_at = NOW() WHERE id = $1;

-- name: DeletePortfolioProperty :exec
DELETE FROM portfolio WHERE id = $1;
