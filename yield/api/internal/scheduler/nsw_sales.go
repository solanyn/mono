package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/solanyn/mono/yield/api/internal/client"
	"github.com/solanyn/mono/yield/api/internal/metrics"
	"github.com/solanyn/mono/yield/api/internal/store"
)

func (s *Scheduler) ingestNSWVGWeekly() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	now := time.Now()
	monday := now.AddDate(0, 0, -int(now.Weekday()-time.Monday))
	if now.Weekday() < time.Monday {
		monday = monday.AddDate(0, 0, -7)
	}

	for attempt := 0; attempt < 3; attempt++ {
		date := monday.AddDate(0, 0, -7*attempt)
		log.Printf("nsw_sales: trying weekly zip for %s", date.Format("2006-01-02"))

		data, err := client.DownloadNSWVGWeekly(ctx, date)
		if err != nil {
			log.Printf("nsw_sales: download %s: %v", date.Format("20060102"), err)
			continue
		}

		records, err := client.ParseNSWVGZip(data)
		if err != nil {
			log.Printf("nsw_sales: parse %s: %v", date.Format("20060102"), err)
			continue
		}

		if len(records) == 0 {
			log.Printf("nsw_sales: no records in %s, trying earlier week", date.Format("20060102"))
			continue
		}

		sales := recordsToSales(records)
		inserted, err := s.queries.BulkInsertSales(ctx, sales)
		if err != nil {
			log.Printf("nsw_sales: bulk insert: %v", err)
			return
		}

		metrics.Global.SalesIngested.Add(inserted)
		log.Printf("nsw_sales: ingested %d/%d sales from week of %s", inserted, len(records), date.Format("2006-01-02"))
		return
	}

	log.Println("nsw_sales: no weekly zip found in last 3 weeks")
}

func (s *Scheduler) IngestNSWVGBulk(startYear, endYear int) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	for year := startYear; year <= endYear; year++ {
		log.Printf("nsw_sales_bulk: downloading year %d", year)

		data, err := client.DownloadNSWVGYearly(ctx, year)
		if err != nil {
			log.Printf("nsw_sales_bulk: download %d: %v", year, err)
			continue
		}

		records, err := client.ParseNSWVGZip(data)
		if err != nil {
			log.Printf("nsw_sales_bulk: parse %d: %v", year, err)
			continue
		}

		if len(records) == 0 {
			log.Printf("nsw_sales_bulk: no records for %d", year)
			continue
		}

		sales := recordsToSales(records)
		inserted, err := s.queries.BulkInsertSales(ctx, sales)
		if err != nil {
			log.Printf("nsw_sales_bulk: bulk insert %d: %v", year, err)
			continue
		}

		metrics.Global.SalesIngested.Add(inserted)
		log.Printf("nsw_sales_bulk: year %d — %d/%d sales ingested", year, inserted, len(records))
	}

	log.Println("nsw_sales_bulk: bulk import complete")
}

func recordsToSales(records []client.SaleRecord) []store.Sale {
	sales := make([]store.Sale, 0, len(records))
	for _, r := range records {
		s := store.Sale{
			District: r.District,
			Suburb:   r.Suburb,
			Source:   "nsw_vg",
		}
		if r.PropertyID != "" {
			s.PropertyID = &r.PropertyID
		}
		if r.UnitNumber != "" {
			s.UnitNumber = &r.UnitNumber
		}
		if r.HouseNumber != "" {
			s.HouseNumber = &r.HouseNumber
		}
		if r.Street != "" {
			s.Street = &r.Street
		}
		if r.Postcode != "" {
			s.Postcode = &r.Postcode
		}
		if r.Area > 0 {
			s.Area = &r.Area
		}
		if r.AreaType != "" {
			s.AreaType = &r.AreaType
		}
		if !r.ContractDate.IsZero() {
			s.ContractDate = &r.ContractDate
		}
		if !r.SettlementDate.IsZero() {
			s.SettlementDate = &r.SettlementDate
		}
		if r.Price > 0 {
			s.Price = &r.Price
		}
		if r.Zone != "" {
			s.Zone = &r.Zone
		}
		if r.Nature != "" {
			s.Nature = &r.Nature
		}
		if r.Purpose != "" {
			s.Purpose = &r.Purpose
		}
		if r.StrataLot != "" {
			s.StrataLot = &r.StrataLot
		}
		if r.DealingNumber != "" {
			s.DealingNumber = &r.DealingNumber
		}
		sales = append(sales, s)
	}
	return sales
}
