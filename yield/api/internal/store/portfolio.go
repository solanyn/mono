package store

import (
	"context"
	"time"
)

type PortfolioProperty struct {
	ID            int64      `json:"id"`
	Address       string     `json:"address"`
	Suburb        string     `json:"suburb"`
	Postcode      *string    `json:"postcode"`
	PropertyType  *string    `json:"property_type"`
	Bedrooms      *int16     `json:"bedrooms"`
	Bathrooms     *int16     `json:"bathrooms"`
	PurchasePrice *int64     `json:"purchase_price"`
	PurchaseDate  *time.Time `json:"purchase_date"`
	CurrentRentPW *int32     `json:"current_rent_pw"`
	Notes         *string    `json:"notes"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (q *Queries) InsertPortfolioProperty(ctx context.Context, p PortfolioProperty) (PortfolioProperty, error) {
	var result PortfolioProperty
	err := q.pool.QueryRow(ctx,
		`INSERT INTO portfolio (address, suburb, postcode, property_type, bedrooms, bathrooms, purchase_price, purchase_date, current_rent_pw, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, address, suburb, postcode, property_type, bedrooms, bathrooms, purchase_price, purchase_date, current_rent_pw, notes, created_at, updated_at`,
		p.Address, p.Suburb, p.Postcode, p.PropertyType, p.Bedrooms, p.Bathrooms,
		p.PurchasePrice, p.PurchaseDate, p.CurrentRentPW, p.Notes,
	).Scan(
		&result.ID, &result.Address, &result.Suburb, &result.Postcode, &result.PropertyType,
		&result.Bedrooms, &result.Bathrooms, &result.PurchasePrice, &result.PurchaseDate,
		&result.CurrentRentPW, &result.Notes, &result.CreatedAt, &result.UpdatedAt,
	)
	return result, err
}

func (q *Queries) GetPortfolio(ctx context.Context) ([]PortfolioProperty, error) {
	rows, err := q.pool.Query(ctx, `SELECT id, address, suburb, postcode, property_type, bedrooms, bathrooms, purchase_price, purchase_date, current_rent_pw, notes, created_at, updated_at FROM portfolio ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var props []PortfolioProperty
	for rows.Next() {
		var p PortfolioProperty
		if err := rows.Scan(
			&p.ID, &p.Address, &p.Suburb, &p.Postcode, &p.PropertyType,
			&p.Bedrooms, &p.Bathrooms, &p.PurchasePrice, &p.PurchaseDate,
			&p.CurrentRentPW, &p.Notes, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		props = append(props, p)
	}
	return props, rows.Err()
}

func (q *Queries) GetPortfolioProperty(ctx context.Context, id int64) (PortfolioProperty, error) {
	var p PortfolioProperty
	err := q.pool.QueryRow(ctx,
		`SELECT id, address, suburb, postcode, property_type, bedrooms, bathrooms, purchase_price, purchase_date, current_rent_pw, notes, created_at, updated_at FROM portfolio WHERE id = $1`, id,
	).Scan(
		&p.ID, &p.Address, &p.Suburb, &p.Postcode, &p.PropertyType,
		&p.Bedrooms, &p.Bathrooms, &p.PurchasePrice, &p.PurchaseDate,
		&p.CurrentRentPW, &p.Notes, &p.CreatedAt, &p.UpdatedAt,
	)
	return p, err
}

func (q *Queries) UpdatePortfolioRent(ctx context.Context, id int64, rentPW int32) error {
	_, err := q.pool.Exec(ctx, `UPDATE portfolio SET current_rent_pw = $2, updated_at = NOW() WHERE id = $1`, id, rentPW)
	return err
}

func (q *Queries) DeletePortfolioProperty(ctx context.Context, id int64) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM portfolio WHERE id = $1`, id)
	return err
}
