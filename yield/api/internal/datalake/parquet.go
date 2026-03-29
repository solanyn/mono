package datalake

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
)

func readParquetListings(r io.ReaderAt) ([]DomainListing, error) {
	sr, ok := r.(io.ReadSeeker)
	if !ok {
		return nil, fmt.Errorf("reader must implement io.ReadSeeker")
	}

	sr.Seek(0, io.SeekStart)
	magic := make([]byte, 4)
	if _, err := io.ReadFull(sr, magic); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if string(magic) != "PAR1" {
		return nil, fmt.Errorf("not a parquet file")
	}

	sr.Seek(0, io.SeekStart)
	all, err := io.ReadAll(sr)
	if err != nil {
		return nil, err
	}

	var listings []DomainListing
	rows, err := decodeParquetJSON(all)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		l := DomainListing{
			ListingID:    getString(row, "listing_id"),
			Suburb:       getString(row, "suburb"),
			State:        getString(row, "state"),
			Postcode:     getString(row, "postcode"),
			PropertyType: getString(row, "property_type"),
			PriceGuide:   getString(row, "price_guide"),
			AuctionDate:  getString(row, "auction_date"),
		}
		if v, ok := getInt32(row, "bedrooms"); ok {
			l.Bedrooms = &v
		}
		if v, ok := getInt32(row, "bathrooms"); ok {
			l.Bathrooms = &v
		}
		if v, ok := getFloat64(row, "sold_price"); ok {
			l.SoldPrice = &v
		}
		if v, ok := getInt32(row, "days_on_market"); ok {
			l.DaysOnMarket = &v
		}
		if v, ok := getFloat64(row, "latitude"); ok {
			l.Latitude = &v
		}
		if v, ok := getFloat64(row, "longitude"); ok {
			l.Longitude = &v
		}
		listings = append(listings, l)
	}

	return listings, nil
}

func decodeParquetJSON(data []byte) ([]map[string]interface{}, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("file too small")
	}

	footerSize := binary.LittleEndian.Uint32(data[len(data)-8 : len(data)-4])
	if int(footerSize) > len(data)-8 {
		return nil, fmt.Errorf("invalid footer size")
	}

	_ = footerSize

	var rows []map[string]interface{}
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, fmt.Errorf("parquet decode not supported natively, use parquet-go library: %w", err)
	}
	return rows, nil
}

func getString(row map[string]interface{}, key string) string {
	if v, ok := row[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func getInt32(row map[string]interface{}, key string) (int32, bool) {
	v, ok := row[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int32(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int32(i), true
	}
	return 0, false
}

func getFloat64(row map[string]interface{}, key string) (float64, bool) {
	v, ok := row[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) {
			return 0, false
		}
		return n, true
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// ensure binary import is used
var _ = binary.LittleEndian
