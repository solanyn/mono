package store

import (
	"context"
	"time"
)

type Sale struct {
	ID             int64      `json:"id"`
	District       string     `json:"district"`
	PropertyID     *string    `json:"property_id"`
	UnitNumber     *string    `json:"unit_number"`
	HouseNumber    *string    `json:"house_number"`
	Street         *string    `json:"street"`
	Suburb         string     `json:"suburb"`
	Postcode       *string    `json:"postcode"`
	Area           *float64   `json:"area"`
	AreaType       *string    `json:"area_type"`
	ContractDate   *time.Time `json:"contract_date"`
	SettlementDate *time.Time `json:"settlement_date"`
	Price          *int64     `json:"price"`
	Zone           *string    `json:"zone"`
	Nature         *string    `json:"nature"`
	Purpose        *string    `json:"purpose"`
	StrataLot      *string    `json:"strata_lot"`
	DealingNumber  *string    `json:"dealing_number"`
	Source         string     `json:"source"`
	CreatedAt      time.Time  `json:"created_at"`
}

type SuburbMedian struct {
	MedianPrice *float64 `json:"median_price"`
	SaleCount   int64    `json:"sale_count"`
	MeanPrice   *float64 `json:"mean_price"`
}

func (q *Queries) InsertSale(ctx context.Context, s Sale) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO sales (
			district, property_id, unit_number, house_number, street, suburb, postcode,
			area, area_type, contract_date, settlement_date, price, zone, nature,
			purpose, strata_lot, dealing_number, source
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		) ON CONFLICT (dealing_number, property_id) DO NOTHING`,
		s.District, s.PropertyID, s.UnitNumber, s.HouseNumber, s.Street, s.Suburb, s.Postcode,
		s.Area, s.AreaType, s.ContractDate, s.SettlementDate, s.Price, s.Zone, s.Nature,
		s.Purpose, s.StrataLot, s.DealingNumber, s.Source,
	)
	return err
}

func (q *Queries) GetSalesBySuburb(ctx context.Context, suburb string, limit int) ([]Sale, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, district, property_id, unit_number, house_number, street, suburb, postcode,
			area, area_type, contract_date, settlement_date, price, zone, nature,
			purpose, strata_lot, dealing_number, source, created_at
		FROM sales
		WHERE suburb = $1 AND nature = 'R'
		ORDER BY contract_date DESC
		LIMIT $2`,
		suburb, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSales(rows)
}

func (q *Queries) GetSalesBySuburbAndType(ctx context.Context, suburb string, strataType *string, since time.Time) ([]Sale, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, district, property_id, unit_number, house_number, street, suburb, postcode,
			area, area_type, contract_date, settlement_date, price, zone, nature,
			purpose, strata_lot, dealing_number, source, created_at
		FROM sales
		WHERE suburb = $1
		  AND nature = 'R'
		  AND ($2::text IS NULL OR strata_lot IS NOT NULL = ($2 = 'strata'))
		  AND contract_date >= $3
		ORDER BY contract_date DESC`,
		suburb, strataType, since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSales(rows)
}

func (q *Queries) GetSuburbMedianPrice(ctx context.Context, suburb string, since time.Time) (SuburbMedian, error) {
	var m SuburbMedian
	err := q.pool.QueryRow(ctx,
		`SELECT percentile_cont(0.5) WITHIN GROUP (ORDER BY price) AS median_price,
			count(*) AS sale_count,
			avg(price) AS mean_price
		FROM sales
		WHERE suburb = $1 AND nature = 'R' AND contract_date >= $2`,
		suburb, since,
	).Scan(&m.MedianPrice, &m.SaleCount, &m.MeanPrice)
	return m, err
}

func (q *Queries) GetComparableSales(ctx context.Context, suburb string, strataType *string, since time.Time, priceLow, priceHigh int64, limit int) ([]Sale, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, district, property_id, unit_number, house_number, street, suburb, postcode,
			area, area_type, contract_date, settlement_date, price, zone, nature,
			purpose, strata_lot, dealing_number, source, created_at
		FROM sales
		WHERE suburb = $1
		  AND nature = 'R'
		  AND ($2::text IS NULL OR strata_lot IS NOT NULL = ($2 = 'strata'))
		  AND contract_date >= $3
		  AND price BETWEEN $4 AND $5
		ORDER BY contract_date DESC
		LIMIT $6`,
		suburb, strataType, since, priceLow, priceHigh, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSales(rows)
}

func scanSales(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]Sale, error) {
	var sales []Sale
	for rows.Next() {
		var s Sale
		if err := rows.Scan(
			&s.ID, &s.District, &s.PropertyID, &s.UnitNumber, &s.HouseNumber, &s.Street,
			&s.Suburb, &s.Postcode, &s.Area, &s.AreaType, &s.ContractDate, &s.SettlementDate,
			&s.Price, &s.Zone, &s.Nature, &s.Purpose, &s.StrataLot, &s.DealingNumber,
			&s.Source, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		sales = append(sales, s)
	}
	return sales, rows.Err()
}
