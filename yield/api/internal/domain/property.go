package domain

import "time"

type Property struct {
	ID           string    `json:"id"`
	Address      string    `json:"address"`
	Suburb       string    `json:"suburb"`
	Postcode     string    `json:"postcode"`
	PropertyType string    `json:"property_type"`
	Bedrooms     int       `json:"bedrooms"`
	Bathrooms    int       `json:"bathrooms"`
	LandArea     float64   `json:"land_area_sqm"`
	SalePrice    int64     `json:"sale_price"`
	SaleDate     time.Time `json:"sale_date"`
	StrataLot    string    `json:"strata_lot,omitempty"`
	Source       string    `json:"source"`
	CreatedAt    time.Time `json:"created_at"`
}

type Listing struct {
	ID           int64     `json:"id"`
	ListingType  string    `json:"listing_type"`
	Status       string    `json:"status"`
	Suburb       string    `json:"suburb"`
	Postcode     string    `json:"postcode"`
	PriceDisplay string    `json:"price_display"`
	PriceNumeric *int64    `json:"price_numeric,omitempty"`
	Bedrooms     int       `json:"bedrooms"`
	Bathrooms    int       `json:"bathrooms"`
	Carspaces    int       `json:"carspaces"`
	PropertyType string    `json:"property_type"`
	LandArea     float64   `json:"land_area"`
	Headline     string    `json:"headline"`
	Description  string    `json:"description"`
	AgentName    string    `json:"agent_name"`
	DateListed   time.Time `json:"date_listed"`
	DaysListed   int       `json:"days_listed"`
	Lat          float64   `json:"lat"`
	Lon          float64   `json:"lon"`
	BlobKey      string    `json:"blob_key"`
	SnapshotAt   time.Time `json:"snapshot_at"`
}

type SuburbStats struct {
	Suburb           string  `json:"suburb"`
	State            string  `json:"state"`
	MedianPrice      int64   `json:"median_price"`
	MeanPrice        int64   `json:"mean_price"`
	SaleCount        int     `json:"sale_count"`
	MedianYield      float64 `json:"median_yield_pct"`
	AuctionClearance float64 `json:"auction_clearance_pct"`
	DaysOnMarket     int     `json:"days_on_market"`
	SchoolScore      float64 `json:"school_score"`
}

type SchoolCatchment struct {
	UseID     string `json:"use_id"`
	CatchType string `json:"catch_type"`
	School    string `json:"school"`
	Priority  int    `json:"priority"`
}
