package domain

import "fmt"

type AnalyseRequest struct {
	Address      string `json:"address"`
	Price        int64  `json:"price"`
	RentPerWeek  int    `json:"rent_per_week"`
	PropertyType string `json:"property_type,omitempty"`
	Bedrooms     int    `json:"bedrooms,omitempty"`
}

type AnalyseResult struct {
	GrossYield    float64      `json:"gross_yield_pct"`
	RentFairness  string       `json:"rent_fairness"`
	SuburbMedian  int64        `json:"suburb_median_price"`
	PriceVsMedian float64      `json:"price_vs_median_pct"`
	Stats         *SuburbStats `json:"suburb_stats,omitempty"`
}

type RentCheckRequest struct {
	Address      string `json:"address"`
	Bedrooms     int    `json:"bedrooms,omitempty"`
	PropertyType string `json:"property_type,omitempty"`
}

type RentCheckResult struct {
	CurrentRent     int     `json:"current_rent_pw"`
	MedianRent      int     `json:"median_rent_pw"`
	Percentile      float64 `json:"percentile"`
	Fairness        string  `json:"fairness"`
	ComparableCount int     `json:"comparable_count"`
}

func GrossYield(annualRent int, purchasePrice int64) float64 {
	if purchasePrice == 0 {
		return 0
	}
	return float64(annualRent) / float64(purchasePrice) * 100
}

func RentFairness(grossYield float64) string {
	switch {
	case grossYield >= 6:
		return "excellent"
	case grossYield >= 4.5:
		return "good"
	case grossYield >= 3:
		return "fair"
	default:
		return "poor"
	}
}

func Analyse(req AnalyseRequest, stats *SuburbStats) AnalyseResult {
	annualRent := req.RentPerWeek * 52
	yield := GrossYield(annualRent, req.Price)

	var priceVsMedian float64
	if stats != nil && stats.MedianPrice > 0 {
		priceVsMedian = (float64(req.Price) - float64(stats.MedianPrice)) / float64(stats.MedianPrice) * 100
	}

	result := AnalyseResult{
		GrossYield:    yield,
		RentFairness:  RentFairness(yield),
		PriceVsMedian: priceVsMedian,
		Stats:         stats,
	}
	if stats != nil {
		result.SuburbMedian = stats.MedianPrice
	}
	return result
}

func FormatYield(yield float64) string {
	return fmt.Sprintf("%.2f%%", yield)
}
