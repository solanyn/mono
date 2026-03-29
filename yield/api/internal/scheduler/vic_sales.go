package scheduler

import (
	"context"
	"encoding/csv"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/solanyn/mono/yield/api/internal/store"
)

const melbDataURL = "https://data.melbourne.vic.gov.au/api/v2/catalog/datasets/house-prices-by-small-area-sale-year/exports/csv"

func (s *Scheduler) syncVICSales() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Println("vic_sales: fetching Melbourne City Council data")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, melbDataURL, nil)
	if err != nil {
		log.Printf("vic_sales: request: %v", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("vic_sales: download: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("vic_sales: download status %d", resp.StatusCode)
		return
	}

	reader := csv.NewReader(resp.Body)
	reader.Comma = ';'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		log.Printf("vic_sales: read header: %v", err)
		return
	}

	areaIdx := colIdx(header, "Small Area")
	yearIdx := colIdx(header, "Sale Year")
	medianIdx := colIdx(header, "Median Price")
	countIdx := colIdx(header, "Number of Sales")
	typeIdx := colIdx(header, "Property Type")

	if areaIdx < 0 || yearIdx < 0 {
		log.Printf("vic_sales: missing columns in header: %v", header)
		return
	}

	var total int64
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("vic_sales: read row: %v", err)
			continue
		}

		suburb := strings.ToUpper(strings.TrimSpace(getCol(row, areaIdx)))
		if suburb == "" {
			continue
		}

		yearStr := strings.TrimSpace(getCol(row, yearIdx))
		year, _ := strconv.Atoi(yearStr)
		if year == 0 {
			continue
		}

		var medianPrice *int64
		if v := strings.TrimSpace(getCol(row, medianIdx)); v != "" {
			v = strings.ReplaceAll(v, "$", "")
			v = strings.ReplaceAll(v, ",", "")
			if p, err := strconv.ParseInt(v, 10, 64); err == nil {
				medianPrice = &p
			}
		}

		var saleCount *int32
		if v := strings.TrimSpace(getCol(row, countIdx)); v != "" {
			if c, err := strconv.ParseInt(v, 10, 32); err == nil {
				c32 := int32(c)
				saleCount = &c32
			}
		}

		propType := strings.TrimSpace(getCol(row, typeIdx))
		_ = propType

		stats := store.SuburbStatsRow{
			Suburb:      suburb,
			State:       "VIC",
			MedianPrice: medianPrice,
			SaleCount:   saleCount,
		}

		if err := s.queries.UpsertSuburbStats(ctx, stats); err != nil {
			log.Printf("vic_sales: upsert %s %d: %v", suburb, year, err)
			continue
		}
		total++
	}

	log.Printf("vic_sales: synced %d suburb-year records", total)
}

func colIdx(header []string, name string) int {
	for i, h := range header {
		if strings.TrimSpace(h) == name {
			return i
		}
	}
	return -1
}

func getCol(row []string, idx int) string {
	if idx >= 0 && idx < len(row) {
		return row[idx]
	}
	return ""
}

func (s *Scheduler) SyncVICSales() {
	s.syncVICSales()
}
