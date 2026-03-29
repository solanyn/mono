package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func (q *Queries) BulkInsertSales(ctx context.Context, sales []Sale) (int64, error) {
	if len(sales) == 0 {
		return 0, nil
	}

	const batchSize = 500
	var total int64

	for i := 0; i < len(sales); i += batchSize {
		end := i + batchSize
		if end > len(sales) {
			end = len(sales)
		}
		batch := sales[i:end]

		b := &pgx.Batch{}
		for _, s := range batch {
			b.Queue(
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
		}

		br := q.pool.SendBatch(ctx, b)
		for range batch {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return total, fmt.Errorf("batch insert at offset %d: %w", i, err)
			}
			total++
		}
		br.Close()
	}

	return total, nil
}
